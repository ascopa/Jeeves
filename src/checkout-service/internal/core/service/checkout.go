package service

import (
	"context"
	"errors"
	"log/slog"

	"jeeves/checkout-service/internal/core/domain"
	"jeeves/checkout-service/internal/core/ports/outbound"

	"github.com/google/uuid"
)

type Service struct {
	tickets    outbound.TicketRepository
	payments   outbound.PaymentRepository
	users      outbound.UserRepository
	events     outbound.EventRepository
	tx         outbound.TxRunner
	provider   outbound.CardProvider
	notifier   outbound.Notifier
	admissions outbound.AdmissionReleaser
}

func NewService(
	tickets outbound.TicketRepository,
	payments outbound.PaymentRepository,
	users outbound.UserRepository,
	events outbound.EventRepository,
	tx outbound.TxRunner,
	provider outbound.CardProvider,
	notifier outbound.Notifier,
	admissions outbound.AdmissionReleaser,
) *Service {
	return &Service{
		tickets: tickets, payments: payments, users: users, events: events,
		tx: tx, provider: provider, notifier: notifier, admissions: admissions,
	}
}

type CheckoutInput struct {
	UserID         uuid.UUID
	TicketID       uuid.UUID
	PaymentToken   string
	IdempotencyKey string
}

func (s *Service) Checkout(ctx context.Context, in CheckoutInput) (domain.Purchase, error) {
	user, err := s.users.FindByID(ctx, in.UserID)
	if err != nil {
		return domain.Purchase{}, err
	}
	if user == nil {
		return domain.Purchase{}, domain.ErrUnknownUser
	}

	existing, err := s.payments.FindByIdempotencyKey(ctx, in.IdempotencyKey)
	if err != nil {
		return domain.Purchase{}, err
	}
	if existing != nil {
		return replay(*existing, in)
	}

	ticket, err := s.tickets.FindByID(ctx, in.TicketID)
	if err != nil {
		return domain.Purchase{}, err
	}
	if ticket == nil {
		return domain.Purchase{}, domain.ErrTicketNotFound
	}
	if err := checkHold(*ticket, in.UserID); err != nil {
		return domain.Purchase{}, err
	}

	payment, err := s.payments.Create(ctx, domain.Payment{
		TicketID:       ticket.ID,
		UserID:         in.UserID,
		AmountCents:    ticket.PriceCents,
		Status:         domain.PaymentPending,
		IdempotencyKey: in.IdempotencyKey,
	})
	if errors.Is(err, domain.ErrDuplicateIdempotencyKey) {
		return domain.Purchase{}, domain.ErrPaymentInProgress
	}
	if err != nil {
		return domain.Purchase{}, err
	}

	ctx = context.WithoutCancel(ctx)

	charge, err := s.provider.Charge(ctx, in.PaymentToken, payment.AmountCents, payment.IdempotencyKey)
	if errors.Is(err, domain.ErrChargeOutcomeUnknown) {

		slog.WarnContext(ctx, "charge outcome unknown, payment left pending", "paymentId", payment.ID, "error", err)
		return domain.Purchase{}, domain.ErrPaymentPending
	}
	if err != nil {
		s.setStatus(ctx, payment.ID, domain.PaymentError, nil)
		return domain.Purchase{}, err
	}
	if charge.Status != domain.ChargeApproved {
		s.setStatus(ctx, payment.ID, domain.PaymentDeclined, &charge.ID)
		return domain.Purchase{}, domain.ErrPaymentDeclined
	}

	sold := false
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if sold, err = s.tickets.MarkSold(ctx, ticket.ID, in.UserID); err != nil || !sold {
			return err
		}
		return s.payments.UpdateStatus(ctx, payment.ID, domain.PaymentApproved, &charge.ID)
	})
	if err != nil {
		s.refund(ctx, payment.ID, charge.ID)
		return domain.Purchase{}, err
	}
	if !sold {
		s.refund(ctx, payment.ID, charge.ID)
		return domain.Purchase{}, domain.PurchaseConflictError{Reason: domain.ReasonHoldLost}
	}

	purchase := domain.Purchase{
		TicketID:    ticket.ID,
		UserID:      in.UserID,
		Status:      domain.StatusSold,
		PaymentID:   payment.ID,
		AmountCents: payment.AmountCents,
	}
	s.afterSale(ctx, user, purchase)
	return purchase, nil
}

func checkHold(ticket domain.Ticket, userID uuid.UUID) error {
	switch {
	case ticket.Status == domain.StatusSold:
		return domain.PurchaseConflictError{Reason: domain.ReasonAlreadySold}
	case ticket.Status != domain.StatusBooked:
		return domain.PurchaseConflictError{Reason: domain.ReasonNotBooked}
	case ticket.BookedBy == nil || *ticket.BookedBy != userID:
		if ticket.HoldActive {
			return domain.PurchaseConflictError{Reason: domain.ReasonNotYourBooking}
		}
		return domain.PurchaseConflictError{Reason: domain.ReasonNotBooked}
	case !ticket.HoldActive:
		return domain.PurchaseConflictError{Reason: domain.ReasonHoldExpired}
	}
	return nil
}

func replay(payment domain.Payment, in CheckoutInput) (domain.Purchase, error) {
	if payment.UserID != in.UserID || payment.TicketID != in.TicketID {
		return domain.Purchase{}, domain.ErrIdempotencyKeyReused
	}
	switch payment.Status {
	case domain.PaymentApproved:
		return domain.Purchase{
			TicketID:    payment.TicketID,
			UserID:      payment.UserID,
			Status:      domain.StatusSold,
			PaymentID:   payment.ID,
			AmountCents: payment.AmountCents,
		}, nil
	case domain.PaymentDeclined:
		return domain.Purchase{}, domain.ErrPaymentDeclined
	case domain.PaymentRefunded:
		return domain.Purchase{}, domain.PurchaseConflictError{Reason: domain.ReasonHoldLost}
	case domain.PaymentPending:
		return domain.Purchase{}, domain.ErrPaymentInProgress
	default:
		return domain.Purchase{}, domain.ErrPaymentFailed
	}
}

func (s *Service) refund(ctx context.Context, paymentID uuid.UUID, chargeID string) {
	status := domain.PaymentRefunded
	if err := s.provider.Refund(ctx, chargeID); err != nil {
		slog.ErrorContext(ctx, "refund failed, needs manual reconciliation", "paymentId", paymentID, "chargeId", chargeID, "error", err)
		status = domain.PaymentError
	}
	s.setStatus(ctx, paymentID, status, &chargeID)
}

func (s *Service) setStatus(ctx context.Context, paymentID uuid.UUID, status string, chargeID *string) {
	if err := s.payments.UpdateStatus(ctx, paymentID, status, chargeID); err != nil {
		slog.ErrorContext(ctx, "updating payment status failed", "paymentId", paymentID, "status", status, "error", err)
	}
}

func (s *Service) afterSale(ctx context.Context, user *domain.User, purchase domain.Purchase) {
	event, err := s.events.FindByTicketID(ctx, purchase.TicketID)
	if err != nil {
		slog.WarnContext(ctx, "loading event after sale failed", "ticketId", purchase.TicketID, "error", err)
	}
	if event != nil && event.QueueEnabled {
		if err := s.admissions.Release(ctx, event.ID, purchase.UserID); err != nil {
			slog.WarnContext(ctx, "releasing waiting room slot failed", "eventId", event.ID, "userId", purchase.UserID, "error", err)
		}
	}
	if user == nil {
		return
	}
	if err := s.notifier.PurchaseConfirmed(ctx, *user, event, purchase); err != nil {
		slog.WarnContext(ctx, "purchase notification failed", "ticketId", purchase.TicketID, "error", err)
	}
}
