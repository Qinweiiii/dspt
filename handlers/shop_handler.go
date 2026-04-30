package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/services"
)

type ShopHandler struct {
	svc *services.ShopService
}

func NewShopHandler(svc *services.ShopService) *ShopHandler {
	return &ShopHandler{svc: svc}
}

func (h *ShopHandler) RegisterRoutes(r *gin.Engine, auth gin.HandlerFunc) {
	public := r.Group("/shop")
	{
		public.GET("/:id", h.QueryByID)
	}

	protected := r.Group("/shop")
	protected.Use(auth)
	{
		protected.PUT("", h.UpdateShop)
	}

	r.GET("/shop-type/list", h.GetTypeList)
}

func (h *ShopHandler) QueryByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("商铺ID格式错误"))
		return
	}

	result, err := h.svc.QueryByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ShopHandler) UpdateShop(c *gin.Context) {
	var shop models.Shop
	if err := c.ShouldBindJSON(&shop); err != nil {
		c.JSON(http.StatusOK, models.Fail("参数格式错误"))
		return
	}

	result, err := h.svc.Update(c.Request.Context(), &shop)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ShopHandler) GetTypeList(c *gin.Context) {
	result, err := h.svc.GetTypeList(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}
