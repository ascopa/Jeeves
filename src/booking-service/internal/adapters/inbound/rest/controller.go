package rest

import (
	"errors"
	"log/slog"
	"net/http"

	"jeeves/booking-service/internal/core/domain"
	"jeeves/booking-service/internal/core/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
	users.GET("/events/:eventId/seats", c.SeatMap)
	users.POST("/book", c.Book)
	return router
}

func (c *Controller) SeatMap(ctx *gin.Context) {
	eventID, err := uuid.Parse(ctx.Param("eventId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "eventId must be a UUID"})
		return
	}

	seatMap, err := c.srv.SeatMap(ctx.Request.Context(), service.SeatMapInput{
		UserID:         userID(ctx),
		EventID:        eventID,
		AdmissionToken: ctx.GetHeader("Admission-Token"),
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEventNotFound):
			ctx.JSON(http.StatusNotFound, gin.H{"error": "event_not_found", "eventId": eventID})
		case errors.Is(err, domain.ErrAdmissionRequired):
			ctx.JSON(http.StatusForbidden, gin.H{"error": "admission_required", "eventId": eventID})
		default:
			slog.ErrorContext(ctx.Request.Context(), "loading seat map failed", "eventId", eventID, "error", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	ctx.JSON(http.StatusOK, seatMap)
}

type bookRequest struct {
	TicketID string `json:"ticketId"`
}

func (c *Controller) Book(ctx *gin.Context) {
	var req bookRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	ticketID, err := uuid.Parse(req.TicketID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ticketId must be a UUID"})
		return
	}

	booking, err := c.srv.Book(ctx.Request.Context(), service.BookInput{
		UserID:         userID(ctx),
		TicketID:       ticketID,
		AdmissionToken: ctx.GetHeader("Admission-Token"),
	})
	if err != nil {
		c.writeError(ctx, ticketID, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"ticketId":      booking.TicketID,
		"status":        booking.Status,
		"seatNumber":    booking.SeatNumber,
		"priceCents":    booking.PriceCents,
		"holdExpiresAt": booking.HoldExpiresAt,
		"next": gin.H{
			"method": http.MethodPost,
			"href":   "/checkout",
			"body":   gin.H{"ticketId": booking.TicketID, "paymentToken": "<token from the card provider>"},
		},
	})
}

func (c *Controller) writeError(ctx *gin.Context, ticketID uuid.UUID, err error) {
	var unavailable domain.TicketUnavailableError
	switch {
	case errors.As(err, &unavailable):
		ctx.JSON(http.StatusConflict, gin.H{"error": "ticket_unavailable", "ticketId": ticketID, "status": unavailable.Status})
	case errors.Is(err, domain.ErrTicketBusy):
		ctx.Header("Retry-After", "1")
		ctx.JSON(http.StatusConflict, gin.H{"error": "ticket_busy", "ticketId": ticketID})
	case errors.Is(err, domain.ErrHoldLimitReached):
		ctx.JSON(http.StatusConflict, gin.H{"error": "hold_limit_reached", "ticketId": ticketID})
	case errors.Is(err, domain.ErrTicketNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "ticket_not_found", "ticketId": ticketID})
	case errors.Is(err, domain.ErrAdmissionRequired):
		ctx.JSON(http.StatusForbidden, gin.H{"error": "admission_required", "ticketId": ticketID})
	case errors.Is(err, domain.ErrUnknownUser):
		ctx.JSON(http.StatusForbidden, gin.H{"error": "unknown_user"})
	default:
		slog.ErrorContext(ctx.Request.Context(), "booking ticket failed", "ticketId", ticketID, "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
