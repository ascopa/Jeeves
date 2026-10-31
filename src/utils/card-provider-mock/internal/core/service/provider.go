package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"jeeves/card-provider-mock/internal/core/domain"
	"jeeves/card-provider-mock/internal/core/ports/outbound"
)

type Service struct {
	cards   outbound.CardStore
	charges outbound.ChargeStore
	now     func() time.Time
}

func NewService(cards outbound.CardStore, charges outbound.ChargeStore) *Service {
	return &Service{cards: cards, charges: charges, now: time.Now}
}

type CardInput struct {
	PAN      string
	ExpMonth int
	ExpYear  int
	CVV      string
}

func (s *Service) Tokenize(ctx context.Context, in CardInput) (domain.Card, error) {
	pan := strings.ReplaceAll(in.PAN, " ", "")
	if err := s.validate(pan, in); err != nil {
		return domain.Card{}, err
	}

	card := domain.Card{
		Token:    "tok_" + randomHex(12),
		Last4:    pan[len(pan)-4:],
		Brand:    brand(pan),
		Declines: pan == domain.DeclinedTestPAN,
	}
	if err := s.cards.Save(ctx, card); err != nil {
		return domain.Card{}, err
	}
	return card, nil
}

func (s *Service) Charge(ctx context.Context, token string, amountCents int64, idempotencyKey string) (domain.Charge, error) {
	if amountCents <= 0 {
		return domain.Charge{}, domain.InvalidRequestError{Reason: "amountCents must be positive"}
	}
	if idempotencyKey == "" {
		return domain.Charge{}, domain.InvalidRequestError{Reason: "idempotencyKey is required"}
	}

	card, err := s.cards.Find(ctx, token)
	if err != nil {
		return domain.Charge{}, err
	}
	if card == nil {
		return domain.Charge{}, domain.ErrUnknownToken
	}

	status := domain.ChargeApproved
	if card.Declines {
		status = domain.ChargeDeclined
	}
	return s.charges.CreateOnce(ctx, domain.Charge{
		ID:             "ch_" + randomHex(12),
		Token:          token,
		AmountCents:    amountCents,
		IdempotencyKey: idempotencyKey,
		Status:         status,
	})
}

func (s *Service) Refund(ctx context.Context, chargeID string) (domain.Charge, error) {
	charge, err := s.charges.Find(ctx, chargeID)
	if err != nil {
		return domain.Charge{}, err
	}
	if charge == nil {
		return domain.Charge{}, domain.ErrChargeNotFound
	}
	switch charge.Status {
	case domain.ChargeRefunded:
		return *charge, nil
	case domain.ChargeApproved:
		charge.Status = domain.ChargeRefunded
		return *charge, s.charges.Update(ctx, *charge)
	default:
		return domain.Charge{}, domain.ErrChargeNotRefundable
	}
}

func (s *Service) validate(pan string, in CardInput) error {
	if len(pan) < 12 || len(pan) > 19 || !digitsOnly(pan) || !luhnValid(pan) {
		return domain.InvalidRequestError{Reason: "invalid card number"}
	}
	if len(in.CVV) < 3 || len(in.CVV) > 4 || !digitsOnly(in.CVV) {
		return domain.InvalidRequestError{Reason: "invalid cvv"}
	}
	if in.ExpMonth < 1 || in.ExpMonth > 12 {
		return domain.InvalidRequestError{Reason: "invalid expiry month"}
	}

	expiresAt := time.Date(in.ExpYear, time.Month(in.ExpMonth)+1, 1, 0, 0, 0, 0, time.UTC)
	if !s.now().Before(expiresAt) {
		return domain.InvalidRequestError{Reason: "card expired"}
	}
	return nil
}

func luhnValid(pan string) bool {
	sum := 0
	double := false
	for i := len(pan) - 1; i >= 0; i-- {
		d := int(pan[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func brand(pan string) string {
	switch {
	case strings.HasPrefix(pan, "4"):
		return "visa"
	case pan[0] == '5' && pan[1] >= '1' && pan[1] <= '5':
		return "mastercard"
	case strings.HasPrefix(pan, "34"), strings.HasPrefix(pan, "37"):
		return "amex"
	default:
		return "unknown"
	}
}

func digitsOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
