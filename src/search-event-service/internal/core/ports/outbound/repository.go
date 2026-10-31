package outbound

import (
	"context"

	"jeeves/search-event-service/internal/core/domain"
)

type EventRepository interface {
	SearchByName(ctx context.Context, name string, limit int) ([]domain.Event, error)
}
