package outbound

import (
	"context"

	"jeeves/waiting-room-service/internal/core/domain"

	"github.com/google/uuid"
)

type EventRepository interface {
	FindByID(ctx context.Context, eventID uuid.UUID) (*domain.Event, error)
	ListQueued(ctx context.Context) ([]domain.Event, error)
	CountAvailable(ctx context.Context, eventID uuid.UUID) (int, error)
}
