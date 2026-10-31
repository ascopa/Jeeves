package domain

import "errors"

var (
	ErrUnknownToken        = errors.New("unknown payment token")
	ErrChargeNotFound      = errors.New("charge not found")
	ErrChargeNotRefundable = errors.New("only approved charges can be refunded")
)

type InvalidRequestError struct {
	Reason string
}

func (e InvalidRequestError) Error() string {
	return e.Reason
}
