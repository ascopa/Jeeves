package memory

import (
	"context"
	"sync"

	"jeeves/card-provider-mock/internal/core/domain"
)

type CardStore struct {
	mu    sync.RWMutex
	cards map[string]domain.Card
}

func NewCardStore() *CardStore {
	return &CardStore{cards: map[string]domain.Card{}}
}

func (s *CardStore) Save(_ context.Context, card domain.Card) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cards[card.Token] = card
	return nil
}

func (s *CardStore) Find(_ context.Context, token string) (*domain.Card, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	card, ok := s.cards[token]
	if !ok {
		return nil, nil
	}
	return &card, nil
}

type ChargeStore struct {
	mu    sync.Mutex
	byID  map[string]domain.Charge
	byKey map[string]string
}

func NewChargeStore() *ChargeStore {
	return &ChargeStore{byID: map[string]domain.Charge{}, byKey: map[string]string{}}
}

func (s *ChargeStore) CreateOnce(_ context.Context, charge domain.Charge) (domain.Charge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byKey[charge.IdempotencyKey]; ok {
		return s.byID[id], nil
	}
	s.byID[charge.ID] = charge
	s.byKey[charge.IdempotencyKey] = charge.ID
	return charge, nil
}

func (s *ChargeStore) Find(_ context.Context, chargeID string) (*domain.Charge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	charge, ok := s.byID[chargeID]
	if !ok {
		return nil, nil
	}
	return &charge, nil
}

func (s *ChargeStore) Update(_ context.Context, charge domain.Charge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[charge.ID] = charge
	return nil
}
