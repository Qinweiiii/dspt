package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/services"
)

// UserHandler 负责用户模块的所有 HTTP 请求处理
type UserHandler struct {
	svc *services.UserService
}

// NewUserHandler 构造函数
func NewUserHandler(svc *services.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// ----------------------------------------
// RegisterRoutes 注册用户模块路由
// ----------------------------------------
func (h *UserHandler) RegisterRoutes(r *gin.Engine, authRequired gin.HandlerFunc) {
	// 白名单（不需要登录）
	public := r.Group("/user")
	{
		public.POST("/code", h.SendCode) // POST /user/code
		public.POST("/login", h.Login)   // POST /user/login
		public.POST("/logout", h.Logout) // POST /user/logout
	}

	// 需要登录
	auth := r.Group("/user")
	auth.Use(authRequired)
	{
		auth.GET("/me", h.Me)                // GET /user/me
		auth.GET("/info/:id", h.GetUserInfo) // GET /user/info/:id
		auth.GET("/:id", h.GetUserByID)      // GET /user/:id
		auth.POST("/sign", h.Sign)           // POST /user/sign
		auth.GET("/sign/count", h.SignCount) // GET /user/sign/count
	}
}

// ----------------------------------------
// SendCode POST /user/code?phone=xxx
// ----------------------------------------
func (h *UserHandler) SendCode(c *gin.Context) {
	// @RequestParam("phone") → c.Query("phone")
	// phone := c.Query("phone")
	// if phone == "" {
	// 	c.JSON(http.StatusOK, models.Fail("手机号不能为空"))
	// 	return
	// }

	type SendCodeForm struct {
		Phone string `json:"phone" binding:"required"`
	}
	var form SendCodeForm

	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusOK, models.Fail("手机号不能为空"))
		return
	}

	result, err := h.svc.SendCode(c.Request.Context(), form.Phone)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// Login POST /user/login
// ----------------------------------------
func (h *UserHandler) Login(c *gin.Context) {
	var form models.LoginFormDTO
	// @RequestBody → c.ShouldBindJSON（binding:"required" 会自动校验必填字段）
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusOK, models.Fail("参数格式错误："+err.Error()))
		return
	}

	result, err := h.svc.Login(c.Request.Context(), form)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// Logout POST /user/logout
// ----------------------------------------
func (h *UserHandler) Logout(c *gin.Context) {
	token := c.GetHeader("authorization")
	result := h.svc.Logout(c.Request.Context(), token)
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// Me GET /user/me
// ----------------------------------------
func (h *UserHandler) Me(c *gin.Context) {
	// currentUser 由 refreshTokenMiddleware 存入
	userDTO := mustGetCurrentUser(c)
	c.JSON(http.StatusOK, models.OKData(userDTO))
}

// ----------------------------------------
// GetUserByID GET /user/:id
// ----------------------------------------
func (h *UserHandler) GetUserByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("用户id格式错误"))
		return
	}

	result, err := h.svc.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GetUserInfo GET /user/info/:id
// ----------------------------------------
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("用户id格式错误"))
		return
	}

	result, err := h.svc.GetUserInfo(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// Sign POST /user/sign
// ----------------------------------------
func (h *UserHandler) Sign(c *gin.Context) {
	currentUser := mustGetCurrentUser(c)

	result, err := h.svc.Sign(c.Request.Context(), currentUser.ID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// SignCount GET /user/sign/count
// ----------------------------------------
func (h *UserHandler) SignCount(c *gin.Context) {
	currentUser := mustGetCurrentUser(c)

	result, err := h.svc.SignCount(c.Request.Context(), currentUser.ID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// 工具函数：从 Gin Context 中取当前登录用户
// 由 refreshTokenMiddleware 写入，这里读取
// ----------------------------------------
func mustGetCurrentUser(c *gin.Context) models.UserDTO {
	val, _ := c.Get("currentUser")
	return val.(models.UserDTO)
}
