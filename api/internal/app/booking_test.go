package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// The availability rule below means 17:00 in the teacher's timezone. São Paulo
// is UTC-3 year round (Brazil dropped DST in 2019), so the instant that slot
// actually starts at is 20:00 UTC. Writing 17:00 UTC here is the same mistake
// that made every real booking fail.
var monday17 = time.Date(2026, time.January, 5, 20, 0, 0, 0, time.UTC)

type bookingHarness struct {
	uc           *Booking
	ledger       *fakes.CreditLedgerRepo
	bookings     *fakes.BookingRepo
	availability *fakes.AvailabilityRepo
	tx           *fakes.TxManager
	clock        *fakes.Clock
	idgen        *fakes.IDGen
}

func newBookingHarness(t *testing.T) bookingHarness {
	t.Helper()
	h := bookingHarness{
		ledger:       &fakes.CreditLedgerRepo{},
		bookings:     &fakes.BookingRepo{},
		availability: &fakes.AvailabilityRepo{},
		tx:           &fakes.TxManager{},
		clock:        &fakes.Clock{},
		idgen:        &fakes.IDGen{},
	}
	h.uc = NewBooking(h.ledger, h.bookings, h.availability, h.tx, h.clock, h.idgen, DefaultBookingPolicy())
	return h
}

func seedMondayRule(t *testing.T, h bookingHarness) {
	t.Helper()
	rule, err := scheduling.NewAvailabilityRule(time.Monday, 17*60, 18*60, 60)
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	if err := h.availability.SaveRules(context.Background(), []scheduling.AvailabilityRule{rule}); err != nil {
		t.Fatal(err)
	}
}

func seedCredits(t *testing.T, h bookingHarness, userID shared.ID, n int) {
	t.Helper()
	if err := h.ledger.Append(context.Background(), userID, credit.Entry{Delta: n, Reason: "seed"}); err != nil {
		t.Fatal(err)
	}
}

func TestListAvailableSlots(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	seedMondayRule(t, h)
	from := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	slots, err := h.uc.ListAvailableSlots(ctx, from, to)
	if err != nil {
		t.Fatal(err)
	}
	want := shared.TimeSlot{Start: monday17, End: monday17.Add(time.Hour)}
	if len(slots) != 1 || !slots[0].Start.Equal(want.Start) || !slots[0].End.Equal(want.End) {
		t.Fatalf("slots = %+v, want [%v]", slots, want)
	}
}

func TestListAvailableSlotsExcludesBooked(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	seedMondayRule(t, h)
	booked, err := shared.NewTimeSlot(monday17, monday17.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := h.bookings.Create(ctx, scheduling.Booking{ID: shared.ID("b1"), StudentID: shared.ID("u1"), Slot: booked, Status: scheduling.StatusScheduled}); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)

	slots, err := h.uc.ListAvailableSlots(ctx, from, from.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 0 {
		t.Fatalf("got %d slots, want none", len(slots))
	}
}

func TestBookLessonDebitsOneCredit(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")
	seedMondayRule(t, h)
	seedCredits(t, h, userID, 5)
	h.clock.NowTime = monday17.Add(-48 * time.Hour)

	if err := h.uc.BookLesson(ctx, userID, monday17); err != nil {
		t.Fatalf("BookLesson: %v", err)
	}

	ledger, err := h.ledger.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got := ledger.Balance(h.clock.NowTime); got != 4 {
		t.Fatalf("balance = %d, want 4", got)
	}
	entries := ledger.Entries()
	if len(entries) != 2 {
		t.Fatalf("got %d ledger entries, want seed + one debit", len(entries))
	}
	debit := entries[len(entries)-1]
	if debit.Delta != -1 || debit.Reason != "booking" {
		t.Fatalf("debit = %+v, want a single -1 booking entry", debit)
	}

	bookings, err := h.bookings.ListByStudentID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bookings) != 1 {
		t.Fatalf("got %d bookings, want 1", len(bookings))
	}
	b := bookings[0]
	if !b.Slot.Start.Equal(monday17) || !b.Slot.End.Equal(monday17.Add(time.Hour)) {
		t.Fatalf("slot = %+v, want 17:00-18:00 UTC", b.Slot)
	}
	if b.Status != scheduling.StatusScheduled || b.StudentID != userID {
		t.Fatalf("booking = %+v", b)
	}
}

func TestBookLessonZeroCreditsWritesNothing(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")
	seedMondayRule(t, h)
	h.clock.NowTime = monday17.Add(-48 * time.Hour)

	err := h.uc.BookLesson(ctx, userID, monday17)
	if !errors.Is(err, credit.ErrInsufficientCredits) {
		t.Fatalf("err = %v, want ErrInsufficientCredits", err)
	}

	bookings, err := h.bookings.ListByStudentID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bookings) != 0 {
		t.Fatalf("got %d bookings, want none written", len(bookings))
	}
	ledger, err := h.ledger.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Entries()) != 0 {
		t.Fatalf("ledger changed: %+v", ledger.Entries())
	}
}

func TestBookLessonTooLate(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")
	seedMondayRule(t, h)
	seedCredits(t, h, userID, 1)
	h.clock.NowTime = monday17.Add(-time.Hour)

	err := h.uc.BookLesson(ctx, userID, monday17)
	if !errors.Is(err, scheduling.ErrTooLateToBook) {
		t.Fatalf("err = %v, want ErrTooLateToBook", err)
	}
	bookings, err := h.bookings.ListByStudentID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bookings) != 0 {
		t.Fatalf("got %d bookings, want none", len(bookings))
	}
	ledger, err := h.ledger.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got := ledger.Balance(h.clock.NowTime); got != 1 {
		t.Fatalf("balance = %d, want 1 (nothing debited)", got)
	}
}

func bookedLesson(t *testing.T, h bookingHarness, userID shared.ID) shared.ID {
	t.Helper()
	seedMondayRule(t, h)
	seedCredits(t, h, userID, 1)
	h.clock.NowTime = monday17.Add(-48 * time.Hour)
	if err := h.uc.BookLesson(context.Background(), userID, monday17); err != nil {
		t.Fatalf("BookLesson: %v", err)
	}
	bookings, err := h.bookings.ListByStudentID(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bookings) != 1 {
		t.Fatalf("got %d bookings, want 1", len(bookings))
	}
	return bookings[0].ID
}

func TestCancelBookingRefundsAt25h(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")
	bookingID := bookedLesson(t, h, userID)
	h.clock.NowTime = monday17.Add(-25 * time.Hour)

	if err := h.uc.CancelBooking(ctx, userID, bookingID); err != nil {
		t.Fatalf("CancelBooking: %v", err)
	}

	ledger, err := h.ledger.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got := ledger.Balance(h.clock.NowTime); got != 1 {
		t.Fatalf("balance = %d, want 1 (credit refunded)", got)
	}
	booking, err := h.bookings.GetByID(ctx, bookingID)
	if err != nil {
		t.Fatal(err)
	}
	if booking.Status != scheduling.StatusCancelledByStudent {
		t.Fatalf("status = %q, want cancelled_by_student", booking.Status)
	}

	if err := h.uc.CancelBooking(ctx, userID, bookingID); err != nil {
		t.Fatalf("second CancelBooking: %v", err)
	}
	if got := ledger.Balance(h.clock.NowTime); got != 1 {
		t.Fatalf("balance after second cancel = %d, want 1 (refunded once)", got)
	}
}

func TestCancelBookingConsumesAt23h(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")
	bookingID := bookedLesson(t, h, userID)
	h.clock.NowTime = monday17.Add(-23 * time.Hour)

	if err := h.uc.CancelBooking(ctx, userID, bookingID); err != nil {
		t.Fatalf("CancelBooking: %v", err)
	}

	ledger, err := h.ledger.Get(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got := ledger.Balance(h.clock.NowTime); got != 0 {
		t.Fatalf("balance = %d, want 0 (credit consumed)", got)
	}
	if len(ledger.Entries()) != 2 {
		t.Fatalf("got %d entries, want seed + debit, no refund line", len(ledger.Entries()))
	}
	booking, err := h.bookings.GetByID(ctx, bookingID)
	if err != nil {
		t.Fatal(err)
	}
	if booking.Status != scheduling.StatusCancelledByStudent {
		t.Fatalf("status = %q, want cancelled_by_student", booking.Status)
	}
}

func TestListMyBookings(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")
	bookingID := bookedLesson(t, h, userID)

	bookings, err := h.uc.ListMyBookings(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bookings) != 1 || bookings[0].ID != bookingID {
		t.Fatalf("got %+v, want [%v]", bookings, bookingID)
	}
}

// Availability is authored in the teacher's local time; a slot start is a UTC
// instant. Matching them by raw clock fields rejected every slot the calendar
// offered, so booking was impossible. This pins the two together.
func TestBookLessonAcceptsASlotTheCalendarOffered(t *testing.T) {
	h := newBookingHarness(t)
	ctx := context.Background()

	// 09:00-12:00 São Paulo on weekdays, 60-minute lessons.
	rules := []scheduling.AvailabilityRule{}
	for _, wd := range []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday} {
		r, err := scheduling.NewAvailabilityRule(wd, 9*60, 12*60, 60)
		if err != nil {
			t.Fatal(err)
		}
		rules = append(rules, r)
	}
	if err := h.availability.SaveRules(ctx, rules); err != nil {
		t.Fatal(err)
	}

	from := h.clock.NowTime.Add(48 * time.Hour).Truncate(24 * time.Hour)
	offered, err := h.uc.ListAvailableSlots(ctx, from, from.Add(72*time.Hour))
	if err != nil {
		t.Fatalf("ListAvailableSlots: %v", err)
	}
	if len(offered) == 0 {
		t.Fatal("the calendar offered no slots to book")
	}

	if err := h.ledger.Append(ctx, shared.ID("student-1"), credit.Entry{Delta: 1, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.BookLesson(ctx, shared.ID("student-1"), offered[0].Start); err != nil {
		t.Fatalf("booking a slot the calendar offered failed: %v", err)
	}
}
