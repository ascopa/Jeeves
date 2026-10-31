package outbound

import (
	"time"

	"github.com/google/uuid"
)

type TokenIssuer interface {
	Issue(userID, eventID uuid.UUID, expiresAt time.Time) (string, error)
}
