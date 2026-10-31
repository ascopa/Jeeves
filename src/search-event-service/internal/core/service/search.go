package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"jeeves/search-event-service/internal/core/domain"
	"jeeves/search-event-service/internal/core/ports/outbound"
)

const (
	maxResults    = 20
	maxNameLength = 100
)

type Service struct {
	events outbound.EventRepository
}

func NewService(events outbound.EventRepository) *Service {
	return &Service{events: events}
}

func (s *Service) Search(ctx context.Context, name string) (domain.SearchResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.SearchResult{}, domain.InvalidSearchError{Reason: "name is required"}
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return domain.SearchResult{}, domain.InvalidSearchError{Reason: "name is too long"}
	}

	events, err := s.events.SearchByName(ctx, name, maxResults)
	if err != nil {
		return domain.SearchResult{}, err
	}
	if events == nil {
		events = []domain.Event{}
	}
	return domain.SearchResult{Events: events}, nil
}
