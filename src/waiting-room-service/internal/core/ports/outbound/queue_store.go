package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type QueueStore interface {
	Join(ctx context.Context, eventID, userID uuid.UUID) (int64, error)
	Position(ctx context.Context, eventID, userID uuid.UUID) (int64, error)
	Leave(ctx context.Context, eventID, userID uuid.UUID) error
	Progress(ctx context.Context, eventID uuid.UUID) (head int64, rate float64, err error)
	ClaimTick(ctx context.Context, eventID uuid.UUID, now time.Time, interval time.Duration) (bool, error)
	Admit(ctx context.Context, eventID uuid.UUID, target, ceiling int, now time.Time, window time.Duration) (int, error)
	AdmissionExpiry(ctx context.Context, eventID, userID uuid.UUID, position int64) (time.Time, error)
	Release(ctx context.Context, eventID, userID uuid.UUID) error
}
