package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/services"
)

type FollowHandler struct {
	svc *services.FollowService
}

func NewFollowHandler(svc *services.FollowService) *FollowHandler {
	return &FollowHandler{svc: svc}
}

func (h *FollowHandler) RegisterRoutes(r *gin.Engine, auth gin.HandlerFunc) {
	protected := r.Group("/follow")
	protected.Use(auth)
	{
		protected.PUT("/:id/:isFollow", h.Follow)     // PUT /follow/{id}/{isFollow}
		protected.GET("/or/not/:id", h.IsFollow)      // GET /follow/or/not/{id}
		protected.GET("/common/:id", h.FollowCommons) // GET /follow/common/{id}
	}
}

// ----------------------------------------
// PUT /follow/:id/:isFollow
// ----------------------------------------
func (h *FollowHandler) Follow(c *gin.Context) {
	followUserID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("用户ID格式错误"))
		return
	}

	isFollowStr := c.Param("isFollow")
	isFollow, err := strconv.ParseBool(isFollowStr)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("isFollow参数格式错误"))
		return
	}

	loginUserID := getLoginUserID(c)
	result, err := h.svc.Follow(c.Request.Context(), loginUserID, followUserID, isFollow)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GET /follow/or/not/:id
// ----------------------------------------
func (h *FollowHandler) IsFollow(c *gin.Context) {
	followUserID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("用户ID格式错误"))
		return
	}

	loginUserID := getLoginUserID(c)
	result, err := h.svc.IsFollow(c.Request.Context(), loginUserID, followUserID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------
// GET /follow/common/:id
// ----------------------------------------
func (h *FollowHandler) FollowCommons(c *gin.Context) {
	targetUserID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("用户ID格式错误"))
		return
	}

	loginUserID := getLoginUserID(c)
	result, err := h.svc.FollowCommons(c.Request.Context(), loginUserID, targetUserID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}
