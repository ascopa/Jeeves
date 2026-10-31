package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusAvailable = "available"
	StatusBooked    = "booked"
	StatusSold      = "sold"
)

type User struct {
	ID    uuid.UUID
	Name  string
	Email string
}

type Event struct {
	ID           uuid.UUID
	Name         string
	Date         time.Time
	QueueEnabled bool
	Venue        Venue
	TicketIDs    []uuid.UUID
}

type HoldRequest struct {
	TicketID uuid.UUID
	UserID   uuid.UUID
	EventID  uuid.UUID
	TTL      time.Duration
	MaxHolds int
}

type Booking struct {
	TicketID      uuid.UUID
	SeatNumber    string
	PriceCents    int64
	Status        string
	HoldExpiresAt time.Time
}
