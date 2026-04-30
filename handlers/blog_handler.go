package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/services"
)

type BlogHandler struct {
	svc *services.BlogService
}

func NewBlogHandler(svc *services.BlogService) *BlogHandler {
	return &BlogHandler{svc: svc}
}

func (h *BlogHandler) RegisterRoutes(r *gin.Engine, auth gin.HandlerFunc) {
	public := r.Group("/blog")
	{
		public.GET("/hot", h.QueryHotBlog)
	}

	protected := r.Group("/blog")
	protected.Use(auth)
	{
		protected.GET("/:id", h.QueryBlogByID)
		protected.POST("", h.SaveBlog)
		protected.PUT("/like/:id", h.LikeBlog)
		protected.GET("/likes/:id", h.QueryBlogLikesByID)
		protected.GET("/of/me", h.QueryMyBlog)
		protected.GET("/of/user", h.QueryBlogByUserID)
		protected.GET("/of/follow", h.QueryBlogOfFollow)
	}
}

// ----------------------------------------
// GET /blog/hot?current=1
// ----------------------------------------
func (h *BlogHandler) QueryHotBlog(c *gin.Context) {
	current := parseIntParam(c.Query("current"), 1)

	// 未登录用户 loginUserID = 0，fillIsLike 会跳过
	loginUserID := getLoginUserID(c)

	result, err := h.svc.QueryHotBlog(c.Request.Context(), current, loginUserID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GET /blog/:id
// ----------------------------------------
func (h *BlogHandler) QueryBlogByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("博客ID格式错误"))
		return
	}

	loginUserID := getLoginUserID(c)
	result, err := h.svc.QueryBlogByID(c.Request.Context(), id, loginUserID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// POST /blog
// ----------------------------------------
func (h *BlogHandler) SaveBlog(c *gin.Context) {
	var blog models.Blog
	if err := c.ShouldBindJSON(&blog); err != nil {
		c.JSON(http.StatusOK, models.Fail("参数格式错误"))
		return
	}

	loginUserID := getLoginUserID(c)
	result, err := h.svc.SaveBlog(c.Request.Context(), &blog, loginUserID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// PUT /blog/like/:id
// ----------------------------------------
func (h *BlogHandler) LikeBlog(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("博客ID格式错误"))
		return
	}

	loginUserID := getLoginUserID(c)
	result, err := h.svc.LikeBlog(c.Request.Context(), id, loginUserID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GET /blog/likes/:id
// ----------------------------------------
func (h *BlogHandler) QueryBlogLikesByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("博客ID格式错误"))
		return
	}

	result, err := h.svc.QueryBlogLikesByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GET /blog/of/me?current=1
// ----------------------------------------
func (h *BlogHandler) QueryMyBlog(c *gin.Context) {
	current := parseIntParam(c.Query("current"), 1)
	loginUserID := getLoginUserID(c)

	result, err := h.svc.QueryMyBlog(c.Request.Context(), loginUserID, current)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GET /blog/of/user?id=xxx&current=1
// ----------------------------------------
func (h *BlogHandler) QueryBlogByUserID(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("用户ID格式错误"))
		return
	}
	current := parseIntParam(c.Query("current"), 1)

	result, err := h.svc.QueryBlogByUserID(c.Request.Context(), userID, current)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GET /blog/of/follow?lastId=xxx&offset=0
// ----------------------------------------
func (h *BlogHandler) QueryBlogOfFollow(c *gin.Context) {
	max, err := strconv.ParseInt(c.Query("lastId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("lastId格式错误"))
		return
	}
	offset := parseIntParam(c.Query("offset"), 0)
	loginUserID := getLoginUserID(c)

	result, err := h.svc.QueryBlogOfFollow(c.Request.Context(), loginUserID, max, offset)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ========================================
// 工具函数（handler 包内共用）
// ========================================

// getLoginUserID 从 gin.Context 取当前登录用户 ID，未登录返回 0
func getLoginUserID(c *gin.Context) int64 {
	user, ok := GetCurrentUser(c)
	if !ok {
		return 0
	}
	return user.ID
}

// parseIntParam 解析字符串为 int，解析失败返回 defaultVal
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
