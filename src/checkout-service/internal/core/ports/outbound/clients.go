package outbound

import (
	"context"

	"jeeves/checkout-service/internal/core/domain"

	"github.com/google/uuid"
)

type CardProvider interface {
	Charge(ctx context.Context, paymentToken string, amountCents int64, idempotencyKey string) (domain.ChargeResult, error)
	Refund(ctx context.Context, chargeID string) error
}

type Notifier interface {
	PurchaseConfirmed(ctx context.Context, user domain.User, event *domain.Event, purchase domain.Purchase) error
}

type AdmissionReleaser interface {
	Release(ctx context.Context, eventID, userID uuid.UUID) error
}
