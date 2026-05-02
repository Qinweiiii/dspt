package services

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/redis/go-redis/v9"
)

// ========================================
// Redis Key 常量
// ========================================
const (
	// FollowKey ZSet → follow:{userId}   member=followUserId
	// 用于共同关注的 SINTER 交集查询
	FollowKey = "follow:"
)

// ========================================
// FollowService
// 对应 Java IFollowService / FollowServiceImpl
// ========================================

type FollowService struct {
	followRepo *repositories.FollowRepository
	userRepo   *repositories.UserRepository
	rdb        *redis.Client
}

func NewFollowService(
	followRepo *repositories.FollowRepository,
	userRepo *repositories.UserRepository,
	rdb *redis.Client,
) *FollowService {
	return &FollowService{followRepo: followRepo, userRepo: userRepo, rdb: rdb}
}

// ----------------------------------------
// Follow 关注 / 取关
// 对应 Java: IFollowService.follow()
// ----------------------------------------
func (s *FollowService) Follow(ctx context.Context, loginUserID int64, followUserID int64, isFollow bool) (models.Result, error) {
	log.Printf("[Follow] user=%d target=%d isFollow=%t", loginUserID, followUserID, isFollow)
	if isFollow {
		// 关注：DB 插入 + Redis Set 写入
		if err := s.followRepo.Insert(ctx, loginUserID, followUserID); err != nil {
			return models.Fail("系统异常"), err
		}
		// SADD follow:{loginUserId} {followUserId}
		key := FollowKey + strconv.FormatInt(loginUserID, 10)
		s.rdb.SAdd(ctx, key, strconv.FormatInt(followUserID, 10))
		if err := s.userRepo.UpdateFollowee(ctx, loginUserID, +1); err != nil {
			return models.Fail("系统异常"), err
		}
		if err := s.userRepo.UpdateFans(ctx, followUserID, +1); err != nil {
			return models.Fail("系统异常"), err
		}
	} else {
		// 取关：DB 删除 + Redis Set 移除
		if err := s.followRepo.Delete(ctx, loginUserID, followUserID); err != nil {
			return models.Fail("系统异常"), err
		}
		key := FollowKey + strconv.FormatInt(loginUserID, 10)
		s.rdb.SRem(ctx, key, strconv.FormatInt(followUserID, 10))
		if err := s.userRepo.UpdateFollowee(ctx, loginUserID, -1); err != nil {
			return models.Fail("系统异常"), err
		}
		if err := s.userRepo.UpdateFans(ctx, followUserID, -1); err != nil {
			return models.Fail("系统异常"), err
		}
	}
	return models.OK(), nil
}

// ----------------------------------------
// IsFollow 判断当前用户是否已关注目标用户
// 对应 Java: IFollowService.isFollow()
// ----------------------------------------
func (s *FollowService) IsFollow(ctx context.Context, loginUserID int64, followUserID int64) (models.Result, error) {
	exists, err := s.followRepo.Exists(ctx, loginUserID, followUserID)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	return models.OKData(exists), nil
}

// ----------------------------------------
// FollowCommons 共同关注
// 对应 Java: IFollowService.followCommons()
// 用 Redis SINTER 取两个用户关注列表的交集，再查用户详情
// ----------------------------------------
func (s *FollowService) FollowCommons(ctx context.Context, loginUserID int64, targetUserID int64) (models.Result, error) {
	key1 := FollowKey + strconv.FormatInt(loginUserID, 10)
	key2 := FollowKey + strconv.FormatInt(targetUserID, 10)

	// SINTER follow:{loginUserId} follow:{targetUserId}
	commonMembers, err := s.rdb.SInter(ctx, key1, key2).Result()
	if err != nil || len(commonMembers) == 0 {
		// Redis 中没有缓存时降级走 DB
		if err != nil {
			fmt.Printf("[FollowCommons] Redis SINTER 失败，降级走DB: %v\n", err)
		}
		return s.followCommonsFromDB(ctx, loginUserID, targetUserID)
	}

	// 把 string member 转为 int64 userId
	ids := make([]int64, 0, len(commonMembers))
	for _, m := range commonMembers {
		id, _ := strconv.ParseInt(m, 10, 64)
		ids = append(ids, id)
	}

	return s.buildUserDTOs(ctx, ids)
}

// ----------------------------------------
// 内部工具
// ----------------------------------------

// followCommonsFromDB DB 降级：直接用 INNER JOIN 查共同关注
func (s *FollowService) followCommonsFromDB(ctx context.Context, userID1, userID2 int64) (models.Result, error) {
	ids, err := s.followRepo.FindCommonFollowUserIDs(ctx, userID1, userID2)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	return s.buildUserDTOs(ctx, ids)
}

// buildUserDTOs 根据 userId 列表批量查用户并转 DTO
func (s *FollowService) buildUserDTOs(ctx context.Context, ids []int64) (models.Result, error) {
	if len(ids) == 0 {
		return models.OKData([]models.UserDTO{}), nil
	}
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
