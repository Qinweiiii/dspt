package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/services"
	"github.com/redis/go-redis/v9"
)

const (
	loginUserKey = "login:token:" // 与 services 包保持一致（或提取到共享常量包）
	loginUserTTL = 30 * time.Minute
	ctxUserKey   = "currentUser" // gin.Context 中存储当前用户的 key
)

// ========================================
// RefreshTokenMiddleware
// 作用：所有请求都经过，尝试从 Redis 读用户信息存入 Context，并续期 Token
// ========================================
func RefreshTokenMiddleware(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头读取 token
		token := c.GetHeader("authorization")
		if token == "" {
			c.Next() // 匿名用户，直接放行
			return
		}

		// 从 Redis 读用户信息：HGETALL login:token:{token}
		ctx := context.Background()
		userMap, err := rdb.HGetAll(ctx, loginUserKey+token).Result()
		if err != nil || len(userMap) == 0 {
			// token 无效或已过期，作为匿名用户放行（后续 LoginRequired 再拦）
			c.Next()
			return
		}

		// 将 map 转换为 UserDTO
		idStr := userMap["id"]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.Next()
			return
		}
		userDTO := models.UserDTO{
			ID:       id,
			NickName: userMap["nickName"],
			Icon:     userMap["icon"],
		}

		// 存入 Gin Context
		c.Set(ctxUserKey, userDTO)

		// Token 续期
		rdb.Expire(ctx, loginUserKey+token, loginUserTTL)

		c.Next()
		// 请求结束后 gin.Context 自动销毁，无需手动 UserHolder.removeUser()
	}
}

// ========================================
// LoginRequiredMiddleware
// 作用：检查 Context 中是否有用户，没有则返回 401
// ========================================
func LoginRequiredMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get(ctxUserKey); !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Fail("请先登录"))
			return
		}
		c.Next()
	}
}

// ========================================
// 以下是其他模块的 handler 占位符
// 随着各 Phase 推进，逐步移到对应的 handler 文件
// ========================================

// -- 文件上传 --
type UploadHandler struct{}

func NewUploadHandler() *UploadHandler { return &UploadHandler{} }
func (h *UploadHandler) RegisterRoutes(r *gin.Engine) {
	upload := r.Group("/upload")
	{
		upload.POST("", func(c *gin.Context) { c.JSON(200, gin.H{"success": true, "data": "TODO"}) })
		upload.DELETE("", func(c *gin.Context) { c.JSON(200, gin.H{"success": true}) })
	}
}

// GetCurrentUser 从 Gin Context 中安全获取当前用户（供其他 handler 文件调用）
func GetCurrentUser(c *gin.Context) (models.UserDTO, bool) {
	val, exists := c.Get(ctxUserKey)
	if !exists {
		return models.UserDTO{}, false
	}
	user, ok := val.(models.UserDTO)
	return user, ok
}

// ========================================
// 保留：统一服务依赖（供 main 注入使用）
// ========================================
var UserSvc *services.UserService
