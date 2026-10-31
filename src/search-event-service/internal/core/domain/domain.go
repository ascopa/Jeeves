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

type Event struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Date           time.Time `json:"date"`
	Venue          Venue     `json:"venue"`
	TotalTickets   int       `json:"totalTickets"`
	AvailableCount int       `json:"availableCount"`
	QueueEnabled   bool      `json:"queueEnabled"`
}

type SearchResult struct {
	Events []Event `json:"events"`
}
