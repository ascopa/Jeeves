package repository

import (
	"context"
	"errors"

	"jeeves/checkout-service/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolation = "23505"

type PaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	var p domain.Payment
	err := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, ticket_id, user_id, amount_cents, provider_charge_id, status, idempotency_key
		FROM payments
		WHERE idempotency_key = $1`,
		key,
	).Scan(&p.ID, &p.TicketID, &p.UserID, &p.AmountCents, &p.ProviderChargeID, &p.Status, &p.IdempotencyKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PaymentRepository) Create(ctx context.Context, payment domain.Payment) (domain.Payment, error) {
	err := conn(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO payments (ticket_id, user_id, amount_cents, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		payment.TicketID, payment.UserID, payment.AmountCents, payment.Status, payment.IdempotencyKey,
	).Scan(&payment.ID)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return domain.Payment{}, domain.ErrDuplicateIdempotencyKey
	}
	if err != nil {
		return domain.Payment{}, err
	}
	return payment, nil
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, paymentID uuid.UUID, status string, providerChargeID *string) error {
	_, err := conn(ctx, r.pool).Exec(ctx, `
		UPDATE payments
		SET status = $2, provider_charge_id = COALESCE($3, provider_charge_id), updated_at = now()
		WHERE id = $1`,
		paymentID, status, providerChargeID,
	)
	return err
}
