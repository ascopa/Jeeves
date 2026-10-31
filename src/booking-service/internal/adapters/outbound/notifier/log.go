package notifier

import (
	"context"
	"log/slog"

	"jeeves/booking-service/internal/core/domain"
)

type LogNotifier struct{}

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{}
}

func (n *LogNotifier) BookingConfirmed(ctx context.Context, user domain.User, event domain.Event, booking domain.Booking) error {
	slog.InfoContext(ctx, "notification sent",
		"channel", "email",
		"template", "booking_confirmed",
		"to", user.Email,
		"ticketId", booking.TicketID,
		"seatNumber", booking.SeatNumber,
		"priceCents", booking.PriceCents,
		"holdExpiresAt", booking.HoldExpiresAt,
		"nextStep", "POST /checkout",
		"event", event.Name,
		"eventDate", event.Date,
	)
	return nil
}
