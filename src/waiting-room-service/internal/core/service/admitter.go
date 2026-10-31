package service

import (
	"context"
	"log/slog"
	"time"

	"jeeves/waiting-room-service/internal/core/domain"
)

func (s *Service) RunAdmitter(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var queued []domain.Event
	var refreshAt time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if s.now().After(refreshAt) {
			events, err := s.events.ListQueued(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "listing queued events failed", "error", err)
				continue
			}
			queued, refreshAt = events, s.now().Add(eventCacheTTL)
		}

		for _, event := range queued {
			if event.SoldOut {
				continue
			}
			if err := s.admit(ctx, event, interval); err != nil {
				slog.ErrorContext(ctx, "admitting users failed", "eventId", event.ID, "error", err)
			}
		}
	}
}

func (s *Service) admit(ctx context.Context, event domain.Event, interval time.Duration) error {
	claimed, err := s.store.ClaimTick(ctx, event.ID, s.now(), interval)
	if err != nil || !claimed {
		return err
	}
	available, err := s.events.CountAvailable(ctx, event.ID)
	if err != nil {
		return err
	}
	target := min(event.MaxActiveUsers, s.seatsFactor*available)
	_, err = s.store.Admit(ctx, event.ID, target, event.AdmitRatePerSec, s.now(), s.admissionTTL)
	return err
}
