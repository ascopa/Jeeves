package notifier

import (
	"context"
	"log/slog"

	"jeeves/checkout-service/internal/core/domain"
)

type LogNotifier struct{}

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{}
}

func (n *LogNotifier) PurchaseConfirmed(ctx context.Context, user domain.User, event *domain.Event, purchase domain.Purchase) error {
	attrs := []any{
		"channel", "email",
		"template", "purchase_confirmed",
		"to", user.Email,
		"ticketId", purchase.TicketID,
		"paymentId", purchase.PaymentID,
		"amountCents", purchase.AmountCents,
	}
	if event != nil {
		attrs = append(attrs, "event", event.Name, "eventDate", event.Date)
	}
	slog.InfoContext(ctx, "notification sent", attrs...)
	return nil
}
