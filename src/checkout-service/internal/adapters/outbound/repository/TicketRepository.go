package repository

import (
	"context"
	"errors"

	"jeeves/checkout-service/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TicketRepository struct {
	pool *pgxpool.Pool
}

func NewTicketRepository(pool *pgxpool.Pool) *TicketRepository {
	return &TicketRepository{pool: pool}
}

func (r *TicketRepository) FindByID(ctx context.Context, ticketID uuid.UUID) (*domain.Ticket, error) {
	var t domain.Ticket
	err := conn(ctx, r.pool).QueryRow(ctx, `
		SELECT id, seat_number, price_cents, status::text, booked_by,
		       COALESCE(hold_expires_at > now(), false)
		FROM tickets
		WHERE id = $1`,
		ticketID,
	).Scan(&t.ID, &t.SeatNumber, &t.PriceCents, &t.Status, &t.BookedBy, &t.HoldActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TicketRepository) MarkSold(ctx context.Context, ticketID, userID uuid.UUID) (bool, error) {
	tag, err := conn(ctx, r.pool).Exec(ctx, `
		UPDATE tickets SET status = 'sold', sold_at = now()
		WHERE id = $1 AND status = 'booked' AND booked_by = $2`,
		ticketID, userID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
