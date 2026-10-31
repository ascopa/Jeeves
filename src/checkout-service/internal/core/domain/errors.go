package domain

import "errors"

var (
	ErrUnknownUser             = errors.New("user not found")
	ErrTicketNotFound          = errors.New("ticket not found")
	ErrPaymentDeclined         = errors.New("payment declined")
	ErrInvalidPaymentToken     = errors.New("invalid payment token")
	ErrPaymentInProgress       = errors.New("a payment with this idempotency key is in progress")
	ErrIdempotencyKeyReused    = errors.New("idempotency key was used for another purchase")
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
	ErrPaymentFailed           = errors.New("payment failed")
	ErrPaymentPending          = errors.New("payment outcome pending")
	ErrChargeOutcomeUnknown    = errors.New("charge outcome unknown")
)

const (
	ReasonNotBooked      = "not_booked"
	ReasonNotYourBooking = "not_your_booking"
	ReasonHoldExpired    = "hold_expired"
	ReasonAlreadySold    = "already_sold"
	ReasonHoldLost       = "hold_lost"
)

type PurchaseConflictError struct {
	Reason string
}

func (e PurchaseConflictError) Error() string {
	return "cannot buy ticket: " + e.Reason
}
