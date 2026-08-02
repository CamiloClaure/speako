package scheduling

import (
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// ErrTooLateToBook is returned when a booking is attempted inside the minimum
// notice window.
var ErrTooLateToBook = shared.NewDomainError("booking.too_late", "too late to book this slot")

// ErrSlotTaken reports that someone else booked the slot first. Storage
// detects this, not the domain: only the database can settle a race between
// two students confirming the same time.
var ErrSlotTaken = shared.NewDomainError("booking.slot_taken", "this slot has just been taken")

// BookingStatus is the lifecycle of a lesson booking.
type BookingStatus string

const (
	StatusScheduled          BookingStatus = "scheduled"
	StatusCompleted          BookingStatus = "completed"
	StatusCancelledByStudent BookingStatus = "cancelled_by_student"
	StatusCancelledByTeacher BookingStatus = "cancelled_by_teacher"
	StatusNoShow             BookingStatus = "no_show"
)

// Booking is one scheduled lesson in a teacher slot. The video call itself
// happens outside Speako; only the slot is stored here.
type Booking struct {
	ID        shared.ID
	StudentID shared.ID
	Slot      shared.TimeSlot
	Status    BookingStatus
}

// NewBooking books a slot, refusing to do so inside the minimum-notice
// window.
func NewBooking(id, studentID shared.ID, slot shared.TimeSlot, now time.Time, minNotice time.Duration) (Booking, error) {
	if err := CanBook(slot.Start, now, minNotice); err != nil {
		return Booking{}, err
	}
	return Booking{ID: id, StudentID: studentID, Slot: slot, Status: StatusScheduled}, nil
}

// CanBook reports whether a slot starting at slotStart can still be booked at
// now, given minNotice. Booking at exactly minNotice before the start is
// allowed; one second later is too late.
func CanBook(slotStart, now time.Time, minNotice time.Duration) error {
	if now.After(slotStart.Add(-minNotice)) {
		return ErrTooLateToBook
	}
	return nil
}

// CancelActor is who requests a cancellation.
type CancelActor string

const (
	CancelledByStudent CancelActor = "student"
	CancelledByTeacher CancelActor = "teacher"
)

// CancelDecision is what a cancellation does to the booked credit.
type CancelDecision string

const (
	// RefundCredit gives the credit back.
	RefundCredit CancelDecision = "refund_credit"
	// ConsumeCredit burns the credit.
	ConsumeCredit CancelDecision = "consume_credit"
)

// CancelOutcome decides whether a cancellation refunds or consumes the credit
// spent on the booking. A teacher cancellation always refunds. A student
// cancellation refunds while at least freeCancelHours remain before the slot
// starts; at exactly freeCancelHours it still refunds, one second later it
// consumes.
func CancelOutcome(b Booking, now time.Time, by CancelActor, freeCancelHours time.Duration) CancelDecision {
	if by == CancelledByTeacher {
		return RefundCredit
	}
	if now.After(b.Slot.Start.Add(-freeCancelHours)) {
		return ConsumeCredit
	}
	return RefundCredit
}
