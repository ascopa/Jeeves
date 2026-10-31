package domain

import "errors"

var (
	ErrTicketNotFound    = errors.New("ticket not found")
	ErrEventNotFound     = errors.New("event not found")
	ErrUnknownUser       = errors.New("user not found")
	ErrAdmissionRequired = errors.New("a valid admission token for this event is required")
	ErrHoldLimitReached  = errors.New("hold limit reached for this event")
	ErrTicketBusy        = errors.New("ticket is busy")
)

type TicketUnavailableError struct {
	Status string
}

func (e TicketUnavailableError) Error() string {
	return "ticket is " + e.Status
}
