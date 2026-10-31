package rest

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"

	"jeeves/card-provider-mock/internal/core/domain"
	"jeeves/card-provider-mock/internal/core/service"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	srv *service.Service
}

func NewController(srv *service.Service) *Controller {
	return &Controller{srv: srv}
}

func NewRouter(c *Controller, apiKey string) *gin.Engine {
	router := gin.New()
	router.GET("/health", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	router.Use(gin.Logger(), gin.Recovery())
	router.POST("/tokens", c.Tokenize)

	merchant := router.Group("/", requireAPIKey(apiKey))
	merchant.POST("/charges", c.Charge)
	merchant.POST("/refunds", c.Refund)
	return router
}

func requireAPIKey(apiKey string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if subtle.ConstantTimeCompare([]byte(ctx.GetHeader("X-Api-Key")), []byte(apiKey)) != 1 {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		ctx.Next()
	}
}

type tokenizeRequest struct {
	PAN      string `json:"pan"`
	ExpMonth int    `json:"expMonth"`
	ExpYear  int    `json:"expYear"`
	CVV      string `json:"cvv"`
}

func (c *Controller) Tokenize(ctx *gin.Context) {
	var req tokenizeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	card, err := c.srv.Tokenize(ctx.Request.Context(), service.CardInput{
		PAN: req.PAN, ExpMonth: req.ExpMonth, ExpYear: req.ExpYear, CVV: req.CVV,
	})
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"token": card.Token, "last4": card.Last4, "brand": card.Brand})
}

type chargeRequest struct {
	Token          string `json:"token"`
	AmountCents    int64  `json:"amountCents"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (c *Controller) Charge(ctx *gin.Context) {
	var req chargeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	charge, err := c.srv.Charge(ctx.Request.Context(), req.Token, req.AmountCents, req.IdempotencyKey)
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, chargeResponse(charge))
}

type refundRequest struct {
	ChargeID string `json:"chargeId"`
}

func (c *Controller) Refund(ctx *gin.Context) {
	var req refundRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	charge, err := c.srv.Refund(ctx.Request.Context(), req.ChargeID)
	if err != nil {
		writeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, chargeResponse(charge))
}

func chargeResponse(charge domain.Charge) gin.H {
	return gin.H{"id": charge.ID, "status": charge.Status, "amountCents": charge.AmountCents}
}

func writeError(ctx *gin.Context, err error) {
	var invalid domain.InvalidRequestError
	switch {
	case errors.As(err, &invalid):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": invalid.Reason})
	case errors.Is(err, domain.ErrUnknownToken):
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_token"})
	case errors.Is(err, domain.ErrChargeNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "charge_not_found"})
	case errors.Is(err, domain.ErrChargeNotRefundable):
		ctx.JSON(http.StatusConflict, gin.H{"error": "charge_not_refundable"})
	default:
		slog.ErrorContext(ctx.Request.Context(), "card provider request failed", "path", ctx.FullPath(), "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
