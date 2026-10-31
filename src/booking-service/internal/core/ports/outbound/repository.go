package outbound

import (
	"context"

	"jeeves/booking-service/internal/core/domain"

	"github.com/google/uuid"
)

type TicketRepository interface {
	HoldWithinLimit(ctx context.Context, req domain.HoldRequest) (*domain.Booking, error)
	FindStatus(ctx context.Context, ticketID uuid.UUID) (*string, error)
	ListSeats(ctx context.Context, ticketIDs []uuid.UUID) ([]domain.Seat, error)
}

type EventRepository interface {
	FindByTicketID(ctx context.Context, ticketID uuid.UUID) (*domain.Event, error)
	FindByID(ctx context.Context, eventID uuid.UUID) (*domain.Event, error)
}
