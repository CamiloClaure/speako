package app

import (
	"context"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	// ErrSlotUnavailable is returned when BookLesson is asked for a start time
	// that no availability rule turns into a slot.
	ErrSlotUnavailable = shared.NewDomainError("slot.unavailable", "slot is not available")
)

// BookingPolicy carries the tunable half of the scheduling rules. The domain
// owns what the rules mean; these are the numbers the teacher chooses, and
// they come from configuration so the deployed value and the documented one
// cannot silently disagree.
type BookingPolicy struct {
	// MinNotice is the minimum lead time to book a lesson.
	MinNotice time.Duration
	// FreeCancel is how long before a slot a student may cancel and keep the credit.
	FreeCancel time.Duration
	// MeetingURL is the external video room attached to every booking.
	MeetingURL string
}

// DefaultBookingPolicy matches the documented defaults in .env.example.
func DefaultBookingPolicy() BookingPolicy {
	return BookingPolicy{MinNotice: 12 * time.Hour, FreeCancel: 24 * time.Hour}
}

// Booking runs the scheduling lifecycle: showing what is free, atomically
// spending a credit to take it, and applying the domain's cancellation policy.
type Booking struct {
	ledger       ports.CreditLedgerRepo
	bookings     ports.BookingRepo
	availability ports.AvailabilityRepo
	tx           ports.TxManager
	clock        ports.Clock
	idgen        ports.IDGen
	policy       BookingPolicy
}

// NewBooking wires the scheduling use cases to their ports.
func NewBooking(
	ledger ports.CreditLedgerRepo,
	bookings ports.BookingRepo,
	availability ports.AvailabilityRepo,
	tx ports.TxManager,
	clock ports.Clock,
	idgen ports.IDGen,
	policy BookingPolicy,
) *Booking {
	return &Booking{
		ledger:       ledger,
		bookings:     bookings,
		availability: availability,
		tx:           tx,
		clock:        clock,
		idgen:        idgen,
		policy:       policy,
	}
}

// ListAvailableSlots returns every slot in [from, to) that the availability
// rules and exceptions open and existing bookings do not already occupy,
// converted to the requester's timezone for slot generation.
func (uc *Booking) ListAvailableSlots(ctx context.Context, from, to time.Time, tz string) ([]shared.TimeSlot, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}
	rules, err := uc.availability.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	exceptions, err := uc.availability.ListExceptions(ctx)
	if err != nil {
		return nil, err
	}
	bookings, err := uc.bookings.ListBetween(ctx, from, to)
	if err != nil {
		return nil, err
	}
	booked := make([]shared.TimeSlot, 0, len(bookings))
	for _, b := range bookings {
		booked = append(booked, b.Slot)
	}
	return scheduling.GenerateSlots(rules, exceptions, booked, from, to, loc), nil
}

// BookLesson spends exactly one credit to hold a slot. Everything happens
// inside one transaction: the balance check, the domain CanBook rule, the
// debit and the insert, so any failure leaves nothing written.
func (uc *Booking) BookLesson(ctx context.Context, userID shared.ID, slotStart time.Time) error {
	return uc.tx.InTx(ctx, func(ctx context.Context) error {
		now := uc.clock.Now()
		slot, err := uc.slotFor(ctx, slotStart)
		if err != nil {
			return err
		}
		if err := scheduling.CanBook(slotStart, now, uc.policy.MinNotice); err != nil {
			return err
		}
		ledger, err := uc.ledger.Get(ctx, userID)
		if err != nil {
			return err
		}
		bookingID := uc.idgen.NewID()
		debited, err := ledger.Debit(1, "booking", bookingID.String(), now)
		if err != nil {
			return err
		}
		// A debit of one always appends exactly one consumption entry.
		entries := debited.Entries()
		if err := uc.ledger.Append(ctx, userID, entries[len(entries)-1]); err != nil {
			return err
		}
		booking, err := scheduling.NewBooking(bookingID, userID, slot, now, uc.policy.MinNotice)
		if err != nil {
			return err
		}
		return uc.bookings.Create(ctx, booking)
	})
}

// CancelBooking asks the domain which way a cancellation goes and then applies
// it: a refund appends a credit entry, a consumption adds nothing. The policy
// itself lives in scheduling.CancelOutcome, never here.
func (uc *Booking) CancelBooking(ctx context.Context, userID shared.ID, bookingID shared.ID) error {
	booking, err := uc.bookings.GetByID(ctx, bookingID)
	if err != nil {
		return err
	}
	if booking.Status != scheduling.StatusScheduled {
		return nil // a cancellation is applied once; redeliveries change nothing
	}
	decision := scheduling.CancelOutcome(booking, uc.clock.Now(), scheduling.CancelledByStudent, uc.policy.FreeCancel)
	return uc.tx.InTx(ctx, func(ctx context.Context) error {
		if decision == scheduling.RefundCredit {
			// ponytail: a refunded credit never expires; restoring the original
			// lot's expiry would need tracking which lot the booking consumed.
			entry := credit.Entry{Delta: 1, Reason: "refund", RefID: booking.ID.String()}
			if err := uc.ledger.Append(ctx, booking.StudentID, entry); err != nil {
				return err
			}
		}
		booking.Status = scheduling.StatusCancelledByStudent
		return uc.bookings.Save(ctx, booking)
	})
}

// ListMyBookings returns the user's bookings in creation order.
func (uc *Booking) ListMyBookings(ctx context.Context, userID shared.ID) ([]scheduling.Booking, error) {
	return uc.bookings.ListByStudentID(ctx, userID)
}

// slotFor turns a requested start instant into the full lesson slot. It uses
// the availability rule for that day and refuses start times no rule covers.
//
// ponytail: weekday and window are evaluated in UTC; the slot is only as real
// as the rule list that produced it. A later session that resolves the
// teacher's timezone replaces time.UTC with the teacher's location.
func (uc *Booking) slotFor(ctx context.Context, slotStart time.Time) (shared.TimeSlot, error) {
	rules, err := uc.availability.ListRules(ctx)
	if err != nil {
		return shared.TimeSlot{}, err
	}
	for _, r := range rules {
		if r.Weekday != slotStart.Weekday() {
			continue
		}
		mins := slotStart.Hour()*60 + slotStart.Minute()
		if mins < r.StartMin || mins+r.SlotMin > r.EndMin {
			continue
		}
		return shared.NewTimeSlot(slotStart, slotStart.Add(time.Duration(r.SlotMin)*time.Minute))
	}
	return shared.TimeSlot{}, ErrSlotUnavailable
}
