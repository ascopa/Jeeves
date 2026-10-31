package service

import (
	"context"
	"time"

	"jeeves/booking-service/internal/core/domain"
	"jeeves/booking-service/internal/core/ports/outbound"

	"github.com/google/uuid"
)

type Config struct {
	HoldTTL              time.Duration
	MaxUserHoldsPerEvent int
}

type Service struct {
	tickets   outbound.TicketRepository
	events    outbound.EventRepository
	admission outbound.AdmissionVerifier
	notifier  outbound.Notifier
	cfg       Config
}

func NewService(
	tickets outbound.TicketRepository,
	events outbound.EventRepository,
	admission outbound.AdmissionVerifier,
	notifier outbound.Notifier,
	cfg Config,
) *Service {
	return &Service{tickets: tickets, events: events, admission: admission, notifier: notifier, cfg: cfg}
}

type BookInput struct {
	UserID         uuid.UUID
	TicketID       uuid.UUID
	AdmissionToken string
}

func (s *Service) Book(ctx context.Context, in BookInput) (domain.Booking, error) {
	event, err := s.events.FindByTicketID(ctx, in.TicketID)
	if err != nil {
		return domain.Booking{}, err
	}
	if event == nil {
		return domain.Booking{}, domain.ErrTicketNotFound
	}
	if event.QueueEnabled {
		if err := s.admission.Verify(in.AdmissionToken, in.UserID, event.ID); err != nil {
			return domain.Booking{}, err
		}
	}

	booking, err := s.tickets.HoldWithinLimit(ctx, domain.HoldRequest{
		TicketID: in.TicketID,
		UserID:   in.UserID,
		EventID:  event.ID,
		TTL:      s.cfg.HoldTTL,
		MaxHolds: s.cfg.MaxUserHoldsPerEvent,
	})
	if err != nil {
		return domain.Booking{}, err
	}
	if booking == nil {
		return domain.Booking{}, s.unavailable(ctx, in.TicketID)
	}

	s.notifier.BookingConfirmed(ctx, in.UserID, *event, *booking)
	return *booking, nil
}

func (s *Service) unavailable(ctx context.Context, ticketID uuid.UUID) error {
	status, err := s.tickets.FindStatus(ctx, ticketID)
	if err != nil {
		return err
	}
	if status == nil {
		return domain.ErrTicketNotFound
	}
	return domain.TicketUnavailableError{Status: *status}
}
