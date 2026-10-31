package outbound

import (
	"context"

	"jeeves/booking-service/internal/core/domain"

	"github.com/google/uuid"
)

type Notifier interface {
	BookingConfirmed(ctx context.Context, userID uuid.UUID, event domain.Event, booking domain.Booking)
}
