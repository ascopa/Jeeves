package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const userIDKey = "userID"

func RequireUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userID, err := uuid.Parse(ctx.GetHeader("X-User-Id"))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-User-Id header must be a user id"})
			return
		}
		ctx.Set(userIDKey, userID)
		ctx.Next()
	}
}

func userID(ctx *gin.Context) uuid.UUID {
	return ctx.MustGet(userIDKey).(uuid.UUID)
}
