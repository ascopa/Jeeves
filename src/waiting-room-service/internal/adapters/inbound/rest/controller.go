package rest

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"jeeves/waiting-room-service/internal/core/domain"
	"jeeves/waiting-room-service/internal/core/service"

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
	users.POST("/queue/:eventId/join", c.Join)
	users.GET("/queue/:eventId/status", c.Status)

	router.POST("/internal/queue/:eventId/release", c.Release)
	return router
}

func (c *Controller) Join(ctx *gin.Context) {
	eventID, ok := parseEventID(ctx)
	if !ok {
		return
	}
	status, err := c.srv.Join(ctx.Request.Context(), userID(ctx), eventID)
	c.respond(ctx, eventID, status, err)
}

func (c *Controller) Status(ctx *gin.Context) {
	eventID, ok := parseEventID(ctx)
	if !ok {
		return
	}
	status, err := c.srv.Status(ctx.Request.Context(), userID(ctx), eventID)
	c.respond(ctx, eventID, status, err)
}

type releaseRequest struct {
	UserID uuid.UUID `json:"userId" binding:"required"`
}

func (c *Controller) Release(ctx *gin.Context) {
	eventID, ok := parseEventID(ctx)
	if !ok {
		return
	}
	var req releaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "userId must be a UUID"})
		return
	}
	if err := c.srv.Release(ctx.Request.Context(), req.UserID, eventID); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "releasing slot failed", "eventId", eventID, "userId", req.UserID, "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	ctx.Status(http.StatusNoContent)
}

func parseEventID(ctx *gin.Context) (uuid.UUID, bool) {
	eventID, err := uuid.Parse(ctx.Param("eventId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "eventId must be a UUID"})
		return uuid.Nil, false
	}
	return eventID, true
}

func (c *Controller) respond(ctx *gin.Context, eventID uuid.UUID, status domain.QueueStatus, err error) {
	switch {
	case err == nil:
		if status.RetryAfterSeconds > 0 {
			ctx.Header("Retry-After", strconv.FormatInt(status.RetryAfterSeconds, 10))
		}
		ctx.JSON(http.StatusOK, status)
	case errors.Is(err, domain.ErrEventNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "event_not_found", "eventId": eventID})
	case errors.Is(err, domain.ErrNotInQueue):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not_in_queue", "eventId": eventID})
	case errors.Is(err, domain.ErrQueueNotEnabled):
		ctx.JSON(http.StatusConflict, gin.H{"error": "queue_not_enabled", "eventId": eventID})
	case errors.Is(err, domain.ErrQueueClosed):
		ctx.JSON(http.StatusConflict, gin.H{"error": "queue_closed", "eventId": eventID})
	default:
		slog.ErrorContext(ctx.Request.Context(), "waiting room request failed", "eventId", eventID, "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
