package services

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/redis/go-redis/v9"
)

// ========================================
// Redis Key 常量
// ========================================
const (
	LoginCodeKey   = "login:code:"  // String, TTL 2min  → login:code:{phone}
	LoginUserKey   = "login:token:" // Hash,   TTL 30min → login:token:{uuid}
	LoginCodeTTL   = 2 * time.Minute
	LoginUserTTL   = 30 * time.Minute
	UserSignKey    = "sign:" // BitMap → sign:{yyyy:MM:}{userId}
	UserNickPrefix = "user_"
)

// 手机号正则
var phoneRegex = regexp.MustCompile(`^1([38][0-9]|4[579]|5[0-3,5-9]|6[6]|7[0135678]|9[89])\d{8}$`)

// ========================================
// UserService 业务层
// ========================================

// Service = 业务逻辑中心 | 它指挥数据库和缓存一起干活。
type UserService struct {
	repo *repositories.UserRepository // 查数据库
	rdb  *redis.Client                // 存缓存/验证码
}

func NewUserService(repo *repositories.UserRepository, rdb *redis.Client) *UserService {
	return &UserService{repo: repo, rdb: rdb}
}

// ----------------------------------------
// SendCode 发送手机验证码
// ----------------------------------------
// SendCode 函数 = 生成验证码 → 存在 Redis 2 分钟 → 给前端返回
func (s *UserService) SendCode(ctx context.Context, phone string) (models.Result, error) {
	// 1. 校验手机号格式
	if !phoneRegex.MatchString(phone) {
		return models.Fail("手机号格式错误"), nil
	}

	// 2. 生成6位随机验证码
	// code := fmt.Sprintf("%06d", rand.Intn(1000000))
	code := "000000"

	// 3. 存入 Redis，TTL=2min
	//    Key: login:code:{phone}
	if err := s.rdb.Set(ctx, LoginCodeKey+phone, code, LoginCodeTTL).Err(); err != nil {
		return models.Fail("系统异常"), fmt.Errorf("SendCode redis set: %w", err)
	}

	// 4. 模拟发送（Java 中是 log.debug，这里同样打印日志）
	//    真实项目接入短信SDK后替换这行
	fmt.Printf("[验证码] 手机号: %s  验证码: %s\n", phone, code)

	return models.OK(), nil
}

// ----------------------------------------
// Login 登录
// ----------------------------------------
func (s *UserService) Login(ctx context.Context, form models.LoginFormDTO) (models.Result, error) {
	// 1. 校验手机号
	if !phoneRegex.MatchString(form.Phone) {
		return models.Fail("手机号格式错误"), nil
	}

	// 2. 从 Redis 取验证码并校验
	cacheCode, err := s.rdb.Get(ctx, LoginCodeKey+form.Phone).Result()
	if err == redis.Nil {
		return models.Fail("验证码已过期"), nil
	}
	if err != nil {
		return models.Fail("系统异常"), fmt.Errorf("Login redis get code: %w", err)
	}
	if cacheCode != form.Code {
		return models.Fail("验证码错误"), nil
	}

	// 3. 根据手机号查用户
	user, err := s.repo.FindByPhone(ctx, form.Phone)
	if err != nil {
		return models.Fail("系统异常"), err
	}

	// 4. 不存在则创建新用户
	if user == nil {
		user, err = s.createUserWithPhone(ctx, form.Phone)
		if err != nil {
			return models.Fail("系统异常"), err
		}
	}

	// 5. 生成 UUID Token
	token := uuid.New().String()

	// 6. 将 UserDTO 存入 Redis Hash
	//    Key: login:token:{uuid}
	//    字段: id / nickName / icon（与 Java 中 BeanUtil.beanToMap 一致）
	userKey := LoginUserKey + token
	userFields := map[string]interface{}{
		"id":       strconv.FormatInt(user.ID, 10), // 存字符串，防 JS 大数精度丢失
		"nickName": user.NickName,
		"icon":     user.Icon,
	}
	if err := s.rdb.HSet(ctx, userKey, userFields).Err(); err != nil {
		return models.Fail("系统异常"), fmt.Errorf("Login redis hset: %w", err)
	}
	// 设置 TTL 30min
	s.rdb.Expire(ctx, userKey, LoginUserTTL)

	// 7. 返回 token（前端后续每次请求都带上它）
	return models.OKData(token), nil
}

// ----------------------------------------
// Logout 登出
// ----------------------------------------
func (s *UserService) Logout(ctx context.Context, token string) models.Result {
	if token != "" {
		// 直接删除 Redis 中的 token，使其失效
		s.rdb.Del(ctx, LoginUserKey+token)
	}
	return models.OK()
}

// ----------------------------------------
// GetUserByID 根据 id 查用户（返回 UserDTO，脱敏）
// ----------------------------------------
func (s *UserService) GetUserByID(ctx context.Context, userID int64) (models.Result, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	if user == nil {
		return models.OK(), nil // Java 中返回 Result.ok()（空data）
	}

	dto := models.UserDTO{
		ID:       user.ID,
		NickName: user.NickName,
		Icon:     user.Icon,
	}
	return models.OKData(dto), nil
}

// ----------------------------------------
// GetUserInfo 查用户详情
// ----------------------------------------
func (s *UserService) GetUserInfo(ctx context.Context, userID int64) (models.Result, error) {
	info, err := s.repo.FindInfoByUserID(ctx, userID)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	if info == nil {
		return models.OK(), nil // 第一次查看，无详情
	}
	// CreateTime / UpdateTime 已通过 json:"-" 标签屏蔽，无需手动置空
	return models.OKData(info), nil
}

// ----------------------------------------
// Sign 用户签到（Redis BitMap）
// ----------------------------------------
func (s *UserService) Sign(ctx context.Context, userID int64) (models.Result, error) {
	now := time.Now()
	// Key: sign:{yyyy:MM:}{userId}
	key := fmt.Sprintf("%s%s:%d", UserSignKey, now.Format("2006:01:"), userID)
	// 今天是本月第几天（1-indexed），bit offset = dayOfMonth-1
	dayOfMonth := now.Day()

	// SETBIT key (dayOfMonth-1) 1
	if err := s.rdb.SetBit(ctx, key, int64(dayOfMonth-1), 1).Err(); err != nil {
		return models.Fail("系统异常"), fmt.Errorf("Sign setbit: %w", err)
	}
	return models.OK(), nil
}

// ----------------------------------------
// SignCount 查询连续签到天数（Redis BitMap + BITFIELD）
// ----------------------------------------
func (s *UserService) SignCount(ctx context.Context, userID int64) (models.Result, error) {
	now := time.Now()
	key := fmt.Sprintf("%s%s:%d", UserSignKey, now.Format("2006:01:"), userID)
	dayOfMonth := now.Day()

	// BITFIELD key GET u{dayOfMonth} 0
	// 获取从第1天到今天的所有签到位（无符号整数）
	results, err := s.rdb.BitField(ctx, key,
		"GET", fmt.Sprintf("u%d", dayOfMonth), "0",
	).Result()
	if err != nil {
		return models.Fail("系统异常"), fmt.Errorf("SignCount bitfield: %w", err)
	}
	if len(results) == 0 || results[0] == 0 {
		return models.OKData(0), nil
	}

	num := results[0]
	// 从最低位（今天）往前数，连续 1 的个数就是连续签到天数
	count := 0
	for num != 0 {
		if num&1 == 1 {
			count++
		} else {
			break // 遇到0，停止
		}
		num >>= 1
	}
	return models.OKData(count), nil
}

// ----------------------------------------
// 内部方法：创建新用户
// ----------------------------------------
func (s *UserService) createUserWithPhone(ctx context.Context, phone string) (*models.User, error) {
	// 生成随机昵称
	nickName := UserNickPrefix + randomString(10)
	user := &models.User{
		Phone:    phone,
		NickName: nickName,
		Icon:     "",
	}
	id, err := s.repo.Create(ctx, user)
	if err != nil {
		return nil, err
	}
	user.ID = id
	return user, nil
}

// randomString 生成指定长度的随机小写字母+数字字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
