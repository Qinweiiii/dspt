package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/redis/go-redis/v9"
)

// ========================================
// Redis Key 常量
// ========================================
const (
	BlogLikedKey = "blog:liked:" // ZSet  → blog:liked:{blogId}   member=userId  score=时间戳
	FeedKey      = "feed:"       // ZSet  → feed:{userId}          member=blogId  score=时间戳
	MaxPageSize  = 10
)

// ========================================
// BlogService
// ========================================

type BlogService struct {
	blogRepo   *repositories.BlogRepository
	userRepo   *repositories.UserRepository
	followRepo *repositories.FollowRepository
	rdb        *redis.Client
}

func NewBlogService(
	blogRepo *repositories.BlogRepository,
	userRepo *repositories.UserRepository,
	followRepo *repositories.FollowRepository,
	rdb *redis.Client,
) *BlogService {
	return &BlogService{
		blogRepo:   blogRepo,
		userRepo:   userRepo,
		followRepo: followRepo,
		rdb:        rdb,
	}
}

// ----------------------------------------
// QueryHotBlog 热门博客分页（按 liked 降序）
// ----------------------------------------
func (s *BlogService) QueryHotBlog(ctx context.Context, current int, loginUserID int64) (models.Result, error) {
	offset := (current - 1) * MaxPageSize
	blogs, err := s.blogRepo.FindHotPage(ctx, offset, MaxPageSize)
	if err != nil {
		return models.Fail("系统异常"), err
	}

	for _, b := range blogs {
		if err := s.fillBlogUser(ctx, b); err != nil {
			return models.Fail("系统异常"), err
		}
		s.fillIsLike(ctx, b, loginUserID)
	}
	return models.OKData(blogs), nil
}

// ----------------------------------------
// QueryBlogByID 根据 id 查博客详情
// ----------------------------------------
func (s *BlogService) QueryBlogByID(ctx context.Context, id int64, loginUserID int64) (models.Result, error) {
	blog, err := s.blogRepo.FindByID(ctx, id)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	if blog == nil {
		return models.Fail("博客不存在"), nil
	}

	if err := s.fillBlogUser(ctx, blog); err != nil {
		return models.Fail("系统异常"), err
	}
	s.fillIsLike(ctx, blog, loginUserID)

	return models.OKData(blog), nil
}

// ----------------------------------------
// LikeBlog 点赞 / 取消点赞
// 用 ZSet 实现，score = 时间戳，方便后续按点赞时间排序
// ----------------------------------------
func (s *BlogService) LikeBlog(ctx context.Context, blogID int64, loginUserID int64) (models.Result, error) {
	key := BlogLikedKey + strconv.FormatInt(blogID, 10)
	userIDStr := strconv.FormatInt(loginUserID, 10)

	// 判断当前用户是否已点赞（score != nil 则已点赞）
	score, err := s.rdb.ZScore(ctx, key, userIDStr).Result()
	alreadyLiked := err == nil && score > 0

	if !alreadyLiked {
		// 未点赞 → DB liked+1，ZSet 写入（score = 当前时间戳）
		if err := s.blogRepo.IncrLiked(ctx, blogID); err != nil {
			return models.Fail("系统异常"), err
		}
		s.rdb.ZAdd(ctx, key, redis.Z{
			Score:  float64(time.Now().UnixMilli()),
			Member: userIDStr,
		})
	} else {
		// 已点赞 → DB liked-1，ZSet 移除
		if err := s.blogRepo.DecrLiked(ctx, blogID); err != nil {
			return models.Fail("系统异常"), err
		}
		s.rdb.ZRem(ctx, key, userIDStr)
	}

	return models.OK(), nil
}

// ----------------------------------------
// QueryBlogLikesByID 点赞排行榜（最早点赞的 Top5）
// ZRANGE key 0 4 → 取 score 最小（最早点赞）的 5 人
// ----------------------------------------
func (s *BlogService) QueryBlogLikesByID(ctx context.Context, blogID int64) (models.Result, error) {
	key := BlogLikedKey + strconv.FormatInt(blogID, 10)

	// ZSet score 是时间戳，ZRANGE 默认升序，取前 5
	members, err := s.rdb.ZRange(ctx, key, 0, 4).Result()
	if err != nil || len(members) == 0 {
		return models.OKData([]models.UserDTO{}), nil
	}

	// 把 string member 转为 int64 userId
	ids := make([]int64, 0, len(members))
	for _, m := range members {
		id, _ := strconv.ParseInt(m, 10, 64)
		ids = append(ids, id)
	}

	// 批量查用户，并保持顺序
	dtos := make([]models.UserDTO, 0, len(ids))
	for _, id := range ids {
		user, err := s.userRepo.FindByID(ctx, id)
		if err != nil || user == nil {
			continue
		}
		dtos = append(dtos, models.UserDTO{
			ID:       user.ID,
			NickName: user.NickName,
			Icon:     user.Icon,
		})
	}

	return models.OKData(dtos), nil
}

// ----------------------------------------
// SaveBlog 发布博客，并推送到粉丝的 Feed ZSet
// ----------------------------------------
func (s *BlogService) SaveBlog(ctx context.Context, blog *models.Blog, loginUserID int64) (models.Result, error) {
	blog.UserID = loginUserID

	id, err := s.blogRepo.Save(ctx, blog)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	blog.ID = id

	// 查询所有粉丝（关注了当前用户的人）
	fans, err := s.followRepo.FindFanUserIDs(ctx, loginUserID)
	if err != nil {
		// 推 Feed 失败不影响发布成功，记录错误即可
		fmt.Printf("[SaveBlog] 推送 Feed 失败: %v\n", err)
		return models.OKData(id), nil
	}

	// 推送博客 id 到每个粉丝的 Feed ZSet（score = 当前时间戳）
	score := float64(time.Now().UnixMilli())
	blogIDStr := strconv.FormatInt(id, 10)
	for _, fanID := range fans {
		key := FeedKey + strconv.FormatInt(fanID, 10)
		s.rdb.ZAdd(ctx, key, redis.Z{Score: score, Member: blogIDStr})
		// fmt.Printf("[Feed] ZAdd key=%s, blogId=%s, res=%d, err=%v\n", key, blogIDStr, res, err)
	}

	return models.OKData(id), nil
}

// ----------------------------------------
// QueryBlogOfFollow Feed 流滚动分页
// ----------------------------------------
func (s *BlogService) QueryBlogOfFollow(ctx context.Context, loginUserID int64, max int64, offset int) (models.Result, error) {
	key := FeedKey + strconv.FormatInt(loginUserID, 10)
	fmt.Printf("[Feed] key=%s, max=%d, offset=%d\n", key, max, offset)

	// ZREVRANGEBYSCORE key max 0 WITHSCORES LIMIT 0 size
	// 按 score（时间戳）降序，从 max 往前取 MaxPageSize 条
	results, err := s.rdb.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min:    "0",
		Max:    strconv.FormatInt(max, 10),
		Offset: int64(offset),
		Count:  int64(MaxPageSize),
	}).Result()
	fmt.Printf("[Feed] ZRevRange results=%v, err=%v\n", results, err)
	if err != nil || len(results) == 0 {
		return models.OKData(models.ScrollResult{}), nil
	}

	// 解析 blogId 列表，同时记录最小 score 和 offset
	ids := make([]int64, 0, len(results))
	minTime := int64(0)
	cnt := 0 // 与 minTime 相同 score 的元素个数

	for _, z := range results {
		id, _ := strconv.ParseInt(fmt.Sprintf("%v", z.Member), 10, 64)
		ids = append(ids, id)

		t := int64(z.Score)
		if t == minTime {
			cnt++
		} else {
			minTime = t
			cnt = 1
		}
	}
	fmt.Printf("[Feed] ids=%v, minTime=%d, cnt=%d\n", ids, minTime, cnt)

	// 查博客详情（保持顺序）
	blogs, err := s.blogRepo.FindByIDsOrdered(ctx, ids)
	fmt.Printf("[Feed] FindByIDsOrdered blogs=%v, err=%v\n", blogs, err)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	for _, b := range blogs {
		s.fillBlogUser(ctx, b)
		s.fillIsLike(ctx, b, loginUserID)
	}

	scroll := models.ScrollResult{
		List:    blogs,
		MinTime: minTime,
		Offset:  cnt,
	}
	return models.OKData(scroll), nil
}

// ----------------------------------------
// QueryMyBlog 查询当前用户的博客（分页）
// ----------------------------------------
func (s *BlogService) QueryMyBlog(ctx context.Context, userID int64, current int) (models.Result, error) {
	offset := (current - 1) * MaxPageSize
	blogs, err := s.blogRepo.FindByUserID(ctx, userID, offset, MaxPageSize)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	return models.OKData(blogs), nil
}

// ----------------------------------------
// QueryBlogByUserID 查询指定用户的博客（分页）
// ----------------------------------------
func (s *BlogService) QueryBlogByUserID(ctx context.Context, userID int64, current int) (models.Result, error) {
	offset := (current - 1) * MaxPageSize
	blogs, err := s.blogRepo.FindByUserID(ctx, userID, offset, MaxPageSize)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	return models.OKData(blogs), nil
}

// ========================================
// 内部工具方法
// ========================================

// fillBlogUser 用博客的 userId 查用户，填充 icon / name 字段
func (s *BlogService) fillBlogUser(ctx context.Context, blog *models.Blog) error {
	user, err := s.userRepo.FindByID(ctx, blog.UserID)
	if err != nil {
		return err
	}
	if user != nil {
		blog.Icon = user.Icon
		blog.Name = user.NickName
	}
	return nil
}

// fillIsLike 查询当前登录用户是否对该博客点过赞
func (s *BlogService) fillIsLike(ctx context.Context, blog *models.Blog, loginUserID int64) {
	if loginUserID == 0 {
		return
	}
	key := BlogLikedKey + strconv.FormatInt(blog.ID, 10)
	userIDStr := strconv.FormatInt(loginUserID, 10)
	score, err := s.rdb.ZScore(ctx, key, userIDStr).Result()
	blog.IsLike = err == nil && score > 0
}
