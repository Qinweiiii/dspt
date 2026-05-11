package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/services"
)

// ========================================
// VoucherHandler 优惠券管理（增加/查询）
// ========================================

type VoucherHandler struct {
	svc *services.VoucherService
}

func NewVoucherHandler(svc *services.VoucherService) *VoucherHandler {
	return &VoucherHandler{svc: svc}
}

func (h *VoucherHandler) RegisterRoutes(r *gin.Engine, auth gin.HandlerFunc) {
	r.GET("/voucher/list/:shopId", h.QueryVoucherOfShop)

	g := r.Group("/voucher")
	g.Use(auth)
	g.Use(SeckillRateLimitMiddleware())
	g.POST("", h.AddVoucher)
	g.POST("/seckill", h.AddSeckill)
}

func (h *VoucherHandler) QueryVoucherOfShop(c *gin.Context) {
	shopID, err := strconv.ParseInt(c.Param("shopId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("shopId格式错误"))
		return
	}
	result, err := h.svc.QueryVoucherOfShop(c.Request.Context(), shopID)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *VoucherHandler) AddVoucher(c *gin.Context) {
	var v models.Voucher
	if err := c.ShouldBindJSON(&v); err != nil {
		c.JSON(http.StatusOK, models.Fail("参数格式错误"))
		return
	}
	result, err := h.svc.AddVoucher(c.Request.Context(), &v)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *VoucherHandler) AddSeckill(c *gin.Context) {
	var v models.Voucher
	if err := c.ShouldBindJSON(&v); err != nil {
		c.JSON(http.StatusOK, models.Fail("参数格式错误"))
		return
	}
	result, err := h.svc.AddSeckillVoucher(c.Request.Context(), &v)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}

// ========================================
// VoucherOrderHandler 秒杀下单
// ========================================

type VoucherOrderHandler struct {
	svc *services.VoucherService
}

func NewVoucherOrderHandler(svc *services.VoucherService) *VoucherOrderHandler {
	return &VoucherOrderHandler{svc: svc}
}

func (h *VoucherOrderHandler) RegisterRoutes(r *gin.Engine, auth gin.HandlerFunc) {
	g := r.Group("/voucher-order")
	g.Use(auth)
	g.POST("/seckill/:voucherId", h.SeckillVoucher)
}

func (h *VoucherOrderHandler) SeckillVoucher(c *gin.Context) {
	voucherID, err := strconv.ParseInt(c.Param("voucherId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, models.Fail("voucherId格式错误"))
		return
	}

	loginUserID := getLoginUserID(c)

	result, err := h.svc.SeckillVoucher(c.Request.Context(), voucherID, loginUserID)
	if err != nil {
		// 用 errors.Is 区分已知业务错误和系统错误
		// 这里 SeckillVoucher 内部已经处理了业务错误，走到这里都是系统级 err
		_ = errors.Is(err, services.ErrStockInsufficient) // 仅示意，实际不会走到这里
		c.JSON(http.StatusOK, models.Fail("系统异常"))
		return
	}
	c.JSON(http.StatusOK, result)
}
