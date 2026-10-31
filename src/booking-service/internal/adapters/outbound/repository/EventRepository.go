package repository

import (
	"context"
	"errors"

	"jeeves/booking-service/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct {
	pool *pgxpool.Pool
}

func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

func (r *EventRepository) FindByTicketID(ctx context.Context, ticketID uuid.UUID) (*domain.Event, error) {
	var e domain.Event
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, date, queue_enabled FROM events
		WHERE ticket_ids @> ARRAY[$1::uuid]
		LIMIT 1`,
		ticketID,
	).Scan(&e.ID, &e.Name, &e.Date, &e.QueueEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EventRepository) FindByID(ctx context.Context, eventID uuid.UUID) (*domain.Event, error) {
	var e domain.Event
	err := r.pool.QueryRow(ctx, `
		SELECT e.id, e.name, e.date, e.queue_enabled, e.ticket_ids, v.id, v.name, v.size
		FROM events e
		JOIN venues v ON v.id = e.venue_id
		WHERE e.id = $1`,
		eventID,
	).Scan(&e.ID, &e.Name, &e.Date, &e.QueueEnabled, &e.TicketIDs, &e.Venue.ID, &e.Venue.Name, &e.Venue.Size)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}
