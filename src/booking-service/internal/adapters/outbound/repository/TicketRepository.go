package repository

import (
	"context"
	"errors"

	"jeeves/booking-service/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	foreignKeyViolation = "23503"
	lockNotAvailable    = "55P03"
)

const effectiveStatus = `CASE WHEN t.status = 'booked' AND t.hold_expires_at <= now() THEN 'available' ELSE t.status::text END`

type TicketRepository struct {
	pool *pgxpool.Pool
}

func NewTicketRepository(pool *pgxpool.Pool) *TicketRepository {
	return &TicketRepository{pool: pool}
}

func (r *TicketRepository) HoldWithinLimit(ctx context.Context, req domain.HoldRequest) (*domain.Booking, error) {
	var booking *domain.Booking
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {

		if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '500ms'`); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`SELECT pg_advisory_xact_lock(hashtextextended($1::uuid::text || $2::uuid::text, 0))`,
			req.UserID, req.EventID,
		); err != nil {
			return err
		}

		var active int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM tickets
			WHERE booked_by = $1 AND status = 'booked' AND hold_expires_at > now() AND id <> $2
			  AND id = ANY((SELECT ticket_ids FROM events WHERE id = $3)::uuid[])`,
			req.UserID, req.TicketID, req.EventID,
		).Scan(&active); err != nil {
			return err
		}
		if active >= req.MaxHolds {
			return domain.ErrHoldLimitReached
		}

		var b domain.Booking
		err := tx.QueryRow(ctx, `
			UPDATE tickets
			SET status = 'booked', booked_by = $2, booked_at = now(),
			    hold_expires_at = now() + make_interval(secs => $3)
			WHERE id = $1
			  AND (status = 'available'
			       OR (status = 'booked' AND (hold_expires_at <= now() OR booked_by = $2)))
			RETURNING id, seat_number, price_cents, status::text, hold_expires_at`,
			req.TicketID, req.UserID, req.TTL.Seconds(),
		).Scan(&b.TicketID, &b.SeatNumber, &b.PriceCents, &b.Status, &b.HoldExpiresAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		booking = &b
		return nil
	})
	return booking, translate(err)
}

func (r *TicketRepository) FindStatus(ctx context.Context, ticketID uuid.UUID) (*string, error) {
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status::text FROM tickets WHERE id = $1`, ticketID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *TicketRepository) ListSeats(ctx context.Context, ticketIDs []uuid.UUID) ([]domain.Seat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.seat_number, t.type::text, t.price_cents, `+effectiveStatus+`
		FROM tickets t
		WHERE t.id = ANY($1)
		ORDER BY t.seat_number`,
		ticketIDs,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Seat, error) {
		var s domain.Seat
		err := row.Scan(&s.ID, &s.SeatNumber, &s.Type, &s.PriceCents, &s.Status)
		return s, err
	})
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case lockNotAvailable:
		return domain.ErrTicketBusy
	case foreignKeyViolation:
		return domain.ErrUnknownUser
	default:
		return err
	}
}
