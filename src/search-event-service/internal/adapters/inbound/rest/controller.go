package rest

import (
	"errors"
	"log/slog"
	"net/http"

	"jeeves/search-event-service/internal/core/domain"
	"jeeves/search-event-service/internal/core/service"

	"github.com/gin-gonic/gin"
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
	router.GET("/search", c.Search)
	return router
}

func (c *Controller) Search(ctx *gin.Context) {
	res, err := c.srv.Search(ctx.Request.Context(), ctx.Query("name"))
	if err != nil {
		var invalid domain.InvalidSearchError
		if errors.As(err, &invalid) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": invalid.Reason})
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "searching events failed", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	ctx.JSON(http.StatusOK, res)
}
