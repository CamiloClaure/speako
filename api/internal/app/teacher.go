package app

import (
	"context"
	"errors"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	// ErrInvalidAttendance is returned when MarkAttendance is given a status
	// that is not a real attendance outcome.
	ErrInvalidAttendance = shared.NewDomainError("attendance.invalid", "attendance status must be completed or no-show")
)

// Teacher runs the single-teacher side of the platform: the availability
// students book against, the attendance of every lesson, and the products and
// orders that make up the shop.
type Teacher struct {
	products     ports.ProductRepo
	orders       ports.OrderRepo
	bookings     ports.BookingRepo
	availability ports.AvailabilityRepo
	ledger       ports.CreditLedgerRepo
	users        ports.UserRepo
	tx           ports.TxManager
	clock        ports.Clock
	idgen        ports.IDGen
}

// NewTeacher wires the teacher use cases to their ports.
func NewTeacher(
	products ports.ProductRepo,
	orders ports.OrderRepo,
	bookings ports.BookingRepo,
	availability ports.AvailabilityRepo,
	ledger ports.CreditLedgerRepo,
	users ports.UserRepo,
	tx ports.TxManager,
	clock ports.Clock,
	idgen ports.IDGen,
) *Teacher {
	return &Teacher{
		products:     products,
		orders:       orders,
		bookings:     bookings,
		availability: availability,
		ledger:       ledger,
		users:        users,
		tx:           tx,
		clock:        clock,
		idgen:        idgen,
	}
}

// SetAvailability replaces the weekly availability rules wholesale.
func (uc *Teacher) SetAvailability(ctx context.Context, rules []scheduling.AvailabilityRule) error {
	return uc.availability.SaveRules(ctx, rules)
}

// ListAvailability returns the current weekly availability rules.
func (uc *Teacher) ListAvailability(ctx context.Context) ([]scheduling.AvailabilityRule, error) {
	return uc.availability.ListRules(ctx)
}

// AddException saves a per-date override of the weekly rules.
func (uc *Teacher) AddException(ctx context.Context, e scheduling.Exception) error {
	return uc.availability.SaveException(ctx, e)
}

// RemoveException clears the exception covering date.
func (uc *Teacher) RemoveException(ctx context.Context, date time.Time) error {
	return uc.availability.RemoveException(ctx, date)
}

// ListBookings returns every lesson that overlaps [from, to).
func (uc *Teacher) ListBookings(ctx context.Context, from, to time.Time) ([]scheduling.Booking, error) {
	return uc.bookings.ListBetween(ctx, from, to)
}

// MarkAttendance records the outcome of a lesson. Only a real attendance
// outcome is accepted: a lesson is either completed or a no-show, anything
// else is a caller bug and must not overwrite the booking.
func (uc *Teacher) MarkAttendance(ctx context.Context, bookingID shared.ID, status scheduling.BookingStatus) error {
	if status != scheduling.StatusCompleted && status != scheduling.StatusNoShow {
		return ErrInvalidAttendance
	}
	booking, err := uc.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return err
	}
	// Attendance only means something for a lesson that was still on. Marking
	// a cancelled booking "completed" would quietly rewrite history.
	if booking.Status != scheduling.StatusScheduled {
		return ErrInvalidAttendance
	}
	booking.Status = status
	return uc.bookings.Save(ctx, booking)
}

// CancelBookingAsTeacher always refunds the lesson's credit, even one minute
// before the slot starts. A teacher cancellation is never the student's fault,
// so the policy that consumes credits on late student cancellations does not
// apply here.
func (uc *Teacher) CancelBookingAsTeacher(ctx context.Context, bookingID shared.ID) error {
	return uc.tx.InTx(ctx, func(ctx context.Context) error {
		booking, err := uc.bookings.GetByID(ctx, bookingID)
		if err != nil {
			return err
		}
		// Refund exactly once. Without this a double-clicked Cancel button
		// hands the student two credits for one lesson.
		if booking.Status != scheduling.StatusScheduled {
			return nil
		}
		// ponytail: a refunded credit never expires; restoring the original
		// lot's expiry would need tracking which lot the booking consumed.
		entry := credit.Entry{Delta: 1, Reason: "teacher_cancel", RefID: booking.ID.String()}
		if err := uc.ledger.Append(ctx, booking.StudentID, entry); err != nil {
			return err
		}
		booking.Status = scheduling.StatusCancelledByTeacher
		return uc.bookings.Save(ctx, booking)
	})
}

// NewProductID mints a fresh product ID. The admin "create product" handler
// calls this when the request omits one - catalog.NewProduct refuses a zero
// ID, so the server has to supply it rather than trust the client to invent
// a primary key.
func (uc *Teacher) NewProductID() shared.ID {
	return uc.idgen.NewID()
}

// ListProducts returns every product regardless of Active, so the teacher's
// screen can show what is off sale as well as what is on it. Unlike
// Catalog.ListProducts, this is not filtered - the point of the admin view
// is seeing everything.
func (uc *Teacher) ListProducts(ctx context.Context) ([]catalog.Product, error) {
	return uc.products.List(ctx)
}

// UpsertProduct creates the product or, when one with the same slug already
// exists, updates it in place so references to the old ID keep working.
func (uc *Teacher) UpsertProduct(ctx context.Context, p catalog.Product) error {
	existing, err := uc.products.GetBySlug(ctx, p.Slug)
	if errors.Is(err, ports.ErrNotFound) {
		return uc.products.Create(ctx, p)
	}
	if err != nil {
		return err
	}
	p.ID = existing.ID
	return uc.products.Update(ctx, p)
}

// ListOrders returns every order placed in the shop, newest store writes last
// in insertion order.
func (uc *Teacher) ListOrders(ctx context.Context) ([]ordering.Order, error) {
	return uc.orders.ListAll(ctx)
}
