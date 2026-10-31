package outbound

import (
	"context"

	"jeeves/card-provider-mock/internal/core/domain"
)

type CardStore interface {
	Save(ctx context.Context, card domain.Card) error
	Find(ctx context.Context, token string) (*domain.Card, error)
}

type ChargeStore interface {
	CreateOnce(ctx context.Context, charge domain.Charge) (domain.Charge, error)
	Find(ctx context.Context, chargeID string) (*domain.Charge, error)
	Update(ctx context.Context, charge domain.Charge) error
}
