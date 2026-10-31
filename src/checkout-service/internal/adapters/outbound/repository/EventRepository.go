package repository

import (
	"context"
	"errors"

	"jeeves/checkout-service/internal/core/domain"

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
	err := conn(ctx, r.pool).QueryRow(ctx, `
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
