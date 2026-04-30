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
		public.GET("/of/type", h.QueryShopByType) // 按类型查店铺
		public.GET("/of/name", h.QueryShopByName) // 按名称查店铺

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

// QueryShopByType 按类型查询，支持 GEO 附近排序
// GET /shop/of/type?typeId=1&current=1&x=120.149&y=30.164
func (h *ShopHandler) QueryShopByType(c *gin.Context) {
	typeID, err := strconv.Atoi(c.Query("typeId"))
	if err != nil || typeID <= 0 {
		c.JSON(http.StatusOK, models.Fail("typeId格式错误"))
		return
	}
	current := parseIntParam(c.Query("current"), 1)

	// x/y 是可选参数，用指针表达"有值 vs 无值"
	// 对应 Java: @RequestParam(required = false) Double x
	var x, y *float64
	if xStr := c.Query("x"); xStr != "" {
		v, err := strconv.ParseFloat(xStr, 64)
		if err != nil {
			c.JSON(http.StatusOK, models.Fail("x坐标格式错误"))
			return
		}
		x = &v
	}
	if yStr := c.Query("y"); yStr != "" {
		v, err := strconv.ParseFloat(yStr, 64)
		if err != nil {
			c.JSON(http.StatusOK, models.Fail("y坐标格式错误"))
			return
		}
		y = &v
	}

	result, err := h.svc.QueryShopByType(c.Request.Context(), typeID, current, x, y)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// QueryShopByName 按名称关键字模糊查询
// GET /shop/of/name?name=咖啡&current=1
func (h *ShopHandler) QueryShopByName(c *gin.Context) {
	name := c.Query("name")
	current := parseIntParam(c.Query("current"), 1)

	result, err := h.svc.QueryShopByName(c.Request.Context(), name, current)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}
