package domain

import "errors"

var (
	ErrEventNotFound   = errors.New("event not found")
	ErrQueueNotEnabled = errors.New("event has no waiting room")
	ErrNotInQueue      = errors.New("user has not joined the queue")
	ErrQueueClosed     = errors.New("event is sold out and its queue is closed")
)
