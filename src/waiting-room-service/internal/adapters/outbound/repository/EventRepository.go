package repository

import (
	"context"
	"errors"

	"jeeves/waiting-room-service/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const eventColumns = `e.id, e.queue_enabled, e.admit_rate_per_sec, e.max_active_users,
	NOT EXISTS (SELECT 1 FROM tickets t WHERE t.id = ANY(e.ticket_ids) AND t.status <> 'sold')`

type EventRepository struct {
	pool *pgxpool.Pool
}

func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

func (r *EventRepository) FindByID(ctx context.Context, eventID uuid.UUID) (*domain.Event, error) {
	e, err := scanEvent(r.pool.QueryRow(ctx, `SELECT `+eventColumns+` FROM events e WHERE e.id = $1`, eventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EventRepository) ListQueued(ctx context.Context) ([]domain.Event, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+eventColumns+` FROM events e WHERE e.queue_enabled`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Event, error) {
		return scanEvent(row)
	})
}

func (r *EventRepository) CountAvailable(ctx context.Context, eventID uuid.UUID) (int, error) {
	var available int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM events e JOIN tickets t ON t.id = ANY(e.ticket_ids)
		WHERE e.id = $1 AND (t.status = 'available' OR (t.status = 'booked' AND t.hold_expires_at <= now()))`,
		eventID,
	).Scan(&available)
	return available, err
}

func scanEvent(row pgx.Row) (domain.Event, error) {
	var e domain.Event
	err := row.Scan(&e.ID, &e.QueueEnabled, &e.AdmitRatePerSec, &e.MaxActiveUsers, &e.SoldOut)
	return e, err
}
