package rest

import (
	"errors"
	"log/slog"
	"net/http"

	"jeeves/checkout-service/internal/core/domain"
	"jeeves/checkout-service/internal/core/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxIdempotencyKeyLength = 255

type Controller struct {
	srv *service.Service
}

func NewController(srv *service.Service) *Controller {
	return &Controller{srv: srv}
}

func NewRouter(c *Controller) *gin.Engine {
	router := gin.New()
	router.GET("/health", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	router.Use(gin.Logger(), gin.Recovery())
	users := router.Group("/", RequireUser())
	users.POST("/checkout", c.Checkout)
	return router
}

type checkoutRequest struct {
	TicketID     string `json:"ticketId"`
	PaymentToken string `json:"paymentToken"`
}

func (c *Controller) Checkout(ctx *gin.Context) {
	idempotencyKey := ctx.GetHeader("Idempotency-Key")
	if idempotencyKey == "" || len(idempotencyKey) > maxIdempotencyKeyLength {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key header is required (max 255 chars)"})
		return
	}

	var req checkoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	ticketID, err := uuid.Parse(req.TicketID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ticketId must be a UUID"})
		return
	}
	if req.PaymentToken == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "paymentToken is required"})
		return
	}

	purchase, err := c.srv.Checkout(ctx.Request.Context(), service.CheckoutInput{
		UserID:         userID(ctx),
		TicketID:       ticketID,
		PaymentToken:   req.PaymentToken,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		c.writeError(ctx, ticketID, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"ticketId":    purchase.TicketID,
		"status":      purchase.Status,
		"paymentId":   purchase.PaymentID,
		"amountCents": purchase.AmountCents,
	})
}

func (c *Controller) writeError(ctx *gin.Context, ticketID uuid.UUID, err error) {
	var conflict domain.PurchaseConflictError
	switch {
	case errors.As(err, &conflict):
		ctx.JSON(http.StatusConflict, gin.H{"error": conflict.Reason, "ticketId": ticketID})
	case errors.Is(err, domain.ErrTicketNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "ticket_not_found", "ticketId": ticketID})
	case errors.Is(err, domain.ErrUnknownUser):
		ctx.JSON(http.StatusForbidden, gin.H{"error": "unknown_user"})
	case errors.Is(err, domain.ErrPaymentDeclined):
		ctx.JSON(http.StatusPaymentRequired, gin.H{"error": "payment_declined", "ticketId": ticketID, "status": domain.StatusBooked})
	case errors.Is(err, domain.ErrInvalidPaymentToken):
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_payment_token"})
	case errors.Is(err, domain.ErrIdempotencyKeyReused):
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "idempotency_key_reused"})
	case errors.Is(err, domain.ErrPaymentInProgress):
		ctx.JSON(http.StatusConflict, gin.H{"error": "payment_in_progress", "ticketId": ticketID})
	case errors.Is(err, domain.ErrPaymentPending):
		ctx.JSON(http.StatusBadGateway, gin.H{
			"error":   "payment_pending",
			"message": "the payment outcome is unknown and needs review; don't retry with a new Idempotency-Key",
		})
	case errors.Is(err, domain.ErrPaymentFailed):
		ctx.JSON(http.StatusPaymentRequired, gin.H{
			"error":   "payment_failed",
			"message": "the card was not charged; retry with a new Idempotency-Key",
		})
	default:
		slog.ErrorContext(ctx.Request.Context(), "checkout failed", "ticketId", ticketID, "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
