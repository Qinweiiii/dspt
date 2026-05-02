package handlers

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// -- 文件上传 --
type UploadHandler struct {
	baseDir string
}

func NewUploadHandler(baseDir string) *UploadHandler {
	return &UploadHandler{baseDir: baseDir}
}

func (h *UploadHandler) RegisterRoutes(r *gin.Engine) {
	upload := r.Group("/upload")
	{
		upload.POST("/blog", h.UploadBlogImage)
		upload.GET("/blog/delete", h.DeleteBlogImage)
		upload.POST("", h.UploadBlogImage)
		upload.DELETE("", h.DeleteBlogImage)
	}
}

func (h *UploadHandler) UploadBlogImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("文件不能为空"))
		return
	}

	fileName := h.createNewFileName(file.Filename)
	savePath := filepath.Join(h.baseDir, filepath.FromSlash(strings.TrimLeft(fileName, "/")))
	if err := os.MkdirAll(filepath.Dir(savePath), 0o755); err != nil {
		c.JSON(http.StatusOK, models.Fail("创建目录失败"))
		return
	}
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusOK, models.Fail("文件上传失败"))
		return
	}

	c.JSON(http.StatusOK, models.OKData(fileName))
}

func (h *UploadHandler) DeleteBlogImage(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusOK, models.Fail("文件名不能为空"))
		return
	}
	if strings.Contains(name, "..") {
		c.JSON(http.StatusOK, models.Fail("错误的文件名称"))
		return
	}

	rel := filepath.FromSlash(strings.TrimLeft(name, "/"))
	path := filepath.Join(h.baseDir, rel)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, models.OK())
			return
		}
		c.JSON(http.StatusOK, models.Fail("删除失败"))
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusOK, models.Fail("错误的文件名称"))
		return
	}
	if err := os.Remove(path); err != nil {
		c.JSON(http.StatusOK, models.Fail("删除失败"))
		return
	}

	c.JSON(http.StatusOK, models.OK())
}

func (h *UploadHandler) createNewFileName(original string) string {
	ext := filepath.Ext(original)
	name := uuid.New().String()
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(name))
	hash := hasher.Sum32()
	d1 := hash & 0xF
	d2 := (hash >> 4) & 0xF

	if ext == "" {
		return fmt.Sprintf("/blogs/%x/%x/%s", d1, d2, name)
	}
	return fmt.Sprintf("/blogs/%x/%x/%s%s", d1, d2, name, ext)
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
