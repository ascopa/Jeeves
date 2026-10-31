package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusAvailable = "available"
	StatusBooked    = "booked"
	StatusSold      = "sold"
	PaymentPending  = "pending"
	PaymentApproved = "approved"
	PaymentDeclined = "declined"
	PaymentRefunded = "refunded"
	PaymentError    = "error"
	ChargeApproved  = "approved"
	ChargeDeclined  = "declined"
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
}

type Ticket struct {
	ID         uuid.UUID
	SeatNumber string
	PriceCents int64
	Status     string
	BookedBy   *uuid.UUID
	HoldActive bool
}

type Payment struct {
	ID               uuid.UUID
	TicketID         uuid.UUID
	UserID           uuid.UUID
	AmountCents      int64
	ProviderChargeID *string
	Status           string
	IdempotencyKey   string
}

type ChargeResult struct {
	ID     string
	Status string
}

type Purchase struct {
	TicketID    uuid.UUID
	UserID      uuid.UUID
	Status      string
	PaymentID   uuid.UUID
	AmountCents int64
}
