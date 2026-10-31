package service

import (
	"context"

	"jeeves/booking-service/internal/core/domain"

	"github.com/google/uuid"
)

type SeatMapInput struct {
	UserID         uuid.UUID
	EventID        uuid.UUID
	AdmissionToken string
}

func (s *Service) SeatMap(ctx context.Context, in SeatMapInput) (domain.SeatMap, error) {
	event, err := s.events.FindByID(ctx, in.EventID)
	if err != nil {
		return domain.SeatMap{}, err
	}
	if event == nil {
		return domain.SeatMap{}, domain.ErrEventNotFound
	}
	if event.QueueEnabled {
		if err := s.admission.Verify(in.AdmissionToken, in.UserID, event.ID); err != nil {
			return domain.SeatMap{}, err
		}
	}

	seats, err := s.tickets.ListSeats(ctx, event.TicketIDs)
	if err != nil {
		return domain.SeatMap{}, err
	}
	if seats == nil {
		seats = []domain.Seat{}
	}

	seatMap := domain.SeatMap{
		ID:           event.ID,
		Name:         event.Name,
		Date:         event.Date,
		Venue:        event.Venue,
		QueueEnabled: event.QueueEnabled,
		TotalTickets: len(seats),
		Tickets:      seats,
	}
	for _, seat := range seats {
		if seat.Status == domain.StatusAvailable {
			seatMap.AvailableCount++
		}
	}
	return seatMap, nil
}
