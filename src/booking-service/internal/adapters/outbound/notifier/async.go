package notifier

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"jeeves/booking-service/internal/core/domain"

	"github.com/google/uuid"
)

const jobTimeout = 5 * time.Second

type userFinder interface {
	FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

type bookingJob struct {
	userID  uuid.UUID
	event   domain.Event
	booking domain.Booking
}

type Async struct {
	jobs   chan bookingJob
	users  userFinder
	sender *LogNotifier
	wg     sync.WaitGroup
}

func NewAsync(users userFinder, sender *LogNotifier, queueSize, workers int) *Async {
	a := &Async{jobs: make(chan bookingJob, queueSize), users: users, sender: sender}
	for range workers {
		a.wg.Add(1)
		go a.work()
	}
	return a
}

func (a *Async) BookingConfirmed(ctx context.Context, userID uuid.UUID, event domain.Event, booking domain.Booking) {
	select {
	case a.jobs <- bookingJob{userID: userID, event: event, booking: booking}:
	default:
		slog.WarnContext(ctx, "notification queue full, dropping notification", "ticketId", booking.TicketID)
	}
}

func (a *Async) Close() {
	close(a.jobs)
	a.wg.Wait()
}

func (a *Async) work() {
	defer a.wg.Done()
	for job := range a.jobs {
		a.send(job)
	}
}

func (a *Async) send(job bookingJob) {
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()

	user, err := a.users.FindByID(ctx, job.userID)
	if err != nil || user == nil {
		slog.WarnContext(ctx, "loading user for notification failed", "userId", job.userID, "error", err)
		return
	}
	if err := a.sender.BookingConfirmed(ctx, *user, job.event, job.booking); err != nil {
		slog.WarnContext(ctx, "booking notification failed", "ticketId", job.booking.TicketID, "error", err)
	}
}
