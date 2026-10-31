package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	QueueWaiting  = "waiting"
	QueueAdmitted = "admitted"
	QueueExpired  = "expired"
	QueueClosed   = "closed"
)

type Event struct {
	ID              uuid.UUID
	QueueEnabled    bool
	AdmitRatePerSec int
	MaxActiveUsers  int
	SoldOut         bool
}

type QueueStatus struct {
	EventID              uuid.UUID  `json:"eventId"`
	Status               string     `json:"status"`
	Position             int64      `json:"position,omitempty"`
	Ahead                int64      `json:"ahead,omitempty"`
	EstimatedWaitSeconds int64      `json:"estimatedWaitSeconds,omitempty"`
	AdmissionToken       string     `json:"admissionToken,omitempty"`
	ExpiresAt            *time.Time `json:"expiresAt,omitempty"`
	RetryAfterSeconds    int64      `json:"-"`
}
