package repository

import (
	"context"
	"strings"

	"jeeves/search-event-service/internal/core/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

const effectiveStatus = `CASE WHEN t.status = 'booked' AND t.hold_expires_at <= now() THEN 'available' ELSE t.status::text END`

const eventColumns = `
	e.id, e.name, e.date, e.queue_enabled, v.id, v.name, v.size,
	cardinality(e.ticket_ids),
	(SELECT count(*) FROM tickets t WHERE t.id = ANY(e.ticket_ids) AND ` + effectiveStatus + ` = 'available')`

type EventRepository struct {
	pool *pgxpool.Pool
}

func NewEventRepository(pool *pgxpool.Pool) *EventRepository {
	return &EventRepository{pool: pool}
}

func (r *EventRepository) SearchByName(ctx context.Context, name string, limit int) ([]domain.Event, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+eventColumns+`
		FROM events e
		JOIN venues v ON v.id = e.venue_id
		WHERE e.name ILIKE '%' || $1 || '%' ESCAPE '\'
		ORDER BY e.date, e.id
		LIMIT $2`,
		likeEscaper.Replace(name), limit,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Event, error) {
		return scanEvent(row)
	})
}

func scanEvent(row pgx.Row) (domain.Event, error) {
	var e domain.Event
	err := row.Scan(&e.ID, &e.Name, &e.Date, &e.QueueEnabled, &e.Venue.ID, &e.Venue.Name, &e.Venue.Size, &e.TotalTickets, &e.AvailableCount)
	return e, err
}
