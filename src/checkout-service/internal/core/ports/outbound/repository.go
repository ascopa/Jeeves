package outbound

import (
	"context"

	"jeeves/checkout-service/internal/core/domain"

	"github.com/google/uuid"
)

type TicketRepository interface {
	FindByID(ctx context.Context, ticketID uuid.UUID) (*domain.Ticket, error)
	MarkSold(ctx context.Context, ticketID, userID uuid.UUID) (bool, error)
}

type PaymentRepository interface {
	FindByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error)
	Create(ctx context.Context, payment domain.Payment) (domain.Payment, error)
	UpdateStatus(ctx context.Context, paymentID uuid.UUID, status string, providerChargeID *string) error
}

type UserRepository interface {
	FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

type EventRepository interface {
	FindByTicketID(ctx context.Context, ticketID uuid.UUID) (*domain.Event, error)
}

type TxRunner interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
