package service

import (
	"context"
	"math"
	"sync"
	"time"

	"jeeves/waiting-room-service/internal/core/domain"
	"jeeves/waiting-room-service/internal/core/ports/outbound"

	"github.com/google/uuid"
)

const (
	eventCacheTTL = 10 * time.Second
	maxRetryAfter = 30
)

type Service struct {
	store        outbound.QueueStore
	events       outbound.EventRepository
	issuer       outbound.TokenIssuer
	admissionTTL time.Duration
	seatsFactor  int
	now          func() time.Time
	mu           sync.Mutex
	eventCache   map[uuid.UUID]cachedEvent
}

type cachedEvent struct {
	event     *domain.Event
	expiresAt time.Time
}

func NewService(
	store outbound.QueueStore,
	events outbound.EventRepository,
	issuer outbound.TokenIssuer,
	admissionTTL time.Duration,
	seatsFactor int,
) *Service {
	return &Service{
		store:        store,
		events:       events,
		issuer:       issuer,
		admissionTTL: admissionTTL,
		seatsFactor:  seatsFactor,
		now:          time.Now,
		eventCache:   map[uuid.UUID]cachedEvent{},
	}
}

func (s *Service) Join(ctx context.Context, userID, eventID uuid.UUID) (domain.QueueStatus, error) {
	event, err := s.queuedEvent(ctx, eventID)
	if err != nil {
		return domain.QueueStatus{}, err
	}
	if event.SoldOut {
		return domain.QueueStatus{}, domain.ErrQueueClosed
	}

	position, err := s.store.Join(ctx, eventID, userID)
	if err != nil {
		return domain.QueueStatus{}, err
	}
	status, err := s.statusFor(ctx, *event, userID, position)
	if err != nil || status.Status != domain.QueueExpired {
		return status, err
	}

	if err := s.store.Leave(ctx, eventID, userID); err != nil {
		return domain.QueueStatus{}, err
	}
	if position, err = s.store.Join(ctx, eventID, userID); err != nil {
		return domain.QueueStatus{}, err
	}
	return s.statusFor(ctx, *event, userID, position)
}

func (s *Service) Status(ctx context.Context, userID, eventID uuid.UUID) (domain.QueueStatus, error) {
	event, err := s.queuedEvent(ctx, eventID)
	if err != nil {
		return domain.QueueStatus{}, err
	}

	if event.SoldOut {
		return domain.QueueStatus{EventID: eventID, Status: domain.QueueClosed}, nil
	}

	position, err := s.store.Position(ctx, eventID, userID)
	if err != nil {
		return domain.QueueStatus{}, err
	}
	if position == 0 {
		return domain.QueueStatus{}, domain.ErrNotInQueue
	}
	return s.statusFor(ctx, *event, userID, position)
}

func (s *Service) statusFor(ctx context.Context, event domain.Event, userID uuid.UUID, position int64) (domain.QueueStatus, error) {
	head, rate, err := s.store.Progress(ctx, event.ID)
	if err != nil {
		return domain.QueueStatus{}, err
	}
	status := domain.QueueStatus{EventID: event.ID, Position: position}

	if position > head {

		if rate == 0 {
			rate = float64(event.AdmitRatePerSec)
		}
		ahead := position - head
		wait := int64(math.Ceil(float64(ahead) / rate))
		status.Status = domain.QueueWaiting
		status.Ahead = ahead
		status.EstimatedWaitSeconds = wait
		status.RetryAfterSeconds = min(max(wait/2, 1), maxRetryAfter)
		return status, nil
	}

	expiresAt, err := s.store.AdmissionExpiry(ctx, event.ID, userID, position)
	if err != nil {
		return domain.QueueStatus{}, err
	}
	if !s.now().Before(expiresAt) {
		status.Status = domain.QueueExpired
		return status, nil
	}

	token, err := s.issuer.Issue(userID, event.ID, expiresAt)
	if err != nil {
		return domain.QueueStatus{}, err
	}
	status.Status = domain.QueueAdmitted
	status.AdmissionToken = token
	status.ExpiresAt = &expiresAt
	return status, nil
}

func (s *Service) Release(ctx context.Context, userID, eventID uuid.UUID) error {
	return s.store.Release(ctx, eventID, userID)
}

func (s *Service) queuedEvent(ctx context.Context, eventID uuid.UUID) (*domain.Event, error) {
	event, err := s.event(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, domain.ErrEventNotFound
	}
	if !event.QueueEnabled {
		return nil, domain.ErrQueueNotEnabled
	}
	return event, nil
}

func (s *Service) event(ctx context.Context, eventID uuid.UUID) (*domain.Event, error) {
	s.mu.Lock()
	cached, ok := s.eventCache[eventID]
	s.mu.Unlock()
	if ok && s.now().Before(cached.expiresAt) {
		return cached.event, nil
	}

	event, err := s.events.FindByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.eventCache[eventID] = cachedEvent{event: event, expiresAt: s.now().Add(eventCacheTTL)}
	s.mu.Unlock()
	return event, nil
}
