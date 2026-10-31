package domain

import (
	"time"

	"github.com/google/uuid"
)

type Venue struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Size int       `json:"size"`
}

type Seat struct {
	ID         uuid.UUID `json:"id"`
	SeatNumber string    `json:"seatNumber"`
	Type       string    `json:"type"`
	PriceCents int64     `json:"priceCents"`
	Status     string    `json:"status"`
}

type SeatMap struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Date           time.Time `json:"date"`
	Venue          Venue     `json:"venue"`
	QueueEnabled   bool      `json:"queueEnabled"`
	TotalTickets   int       `json:"totalTickets"`
	AvailableCount int       `json:"availableCount"`
	Tickets        []Seat    `json:"tickets"`
}
