package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

type teacherHarness struct {
	uc           *Teacher
	products     *fakes.ProductRepo
	orders       *fakes.OrderRepo
	bookings     *fakes.BookingRepo
	availability *fakes.AvailabilityRepo
	ledger       *fakes.CreditLedgerRepo
	users        *fakes.UserRepo
	tx           *fakes.TxManager
	clock        *fakes.Clock
	idgen        *fakes.IDGen
}

func newTeacherHarness(t *testing.T) teacherHarness {
	t.Helper()
	h := teacherHarness{
		products:     &fakes.ProductRepo{},
		orders:       &fakes.OrderRepo{},
		bookings:     &fakes.BookingRepo{},
		availability: &fakes.AvailabilityRepo{},
		ledger:       &fakes.CreditLedgerRepo{},
		users:        &fakes.UserRepo{},
		tx:           &fakes.TxManager{},
		clock:        &fakes.Clock{},
		idgen:        &fakes.IDGen{},
	}
	h.uc = NewTeacher(h.products, h.orders, h.bookings, h.availability, h.ledger, h.users, h.tx, h.clock, h.idgen)
	return h
}

func seedTeacherBooking(t *testing.T, h teacherHarness, bookingID, studentID shared.ID, slot shared.TimeSlot) {
	t.Helper()
	b := scheduling.Booking{ID: bookingID, StudentID: studentID, Slot: slot, Status: scheduling.StatusScheduled}
	if err := h.bookings.Create(context.Background(), b); err != nil {
		t.Fatal(err)
	}
}

func TestMarkAttendanceRejectsScheduledAndCancelled(t *testing.T) {
	h := newTeacherHarness(t)
	ctx := context.Background()
	slot, err := shared.NewTimeSlot(monday17, monday17.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	bookingID := shared.ID("b1")
	seedTeacherBooking(t, h, bookingID, shared.ID("u1"), slot)

	for _, status := range []scheduling.BookingStatus{
		scheduling.StatusScheduled,
		scheduling.StatusCancelledByStudent,
		scheduling.StatusCancelledByTeacher,
		scheduling.BookingStatus("made_up"),
	} {
		err := h.uc.MarkAttendance(ctx, bookingID, status)
		if !errors.Is(err, ErrInvalidAttendance) {
			t.Fatalf("MarkAttendance(%q) err = %v, want ErrInvalidAttendance", status, err)
		}
	}

	b, err := h.bookings.GetByID(ctx, bookingID)
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != scheduling.StatusScheduled {
		t.Fatalf("status = %q, want scheduled unchanged", b.Status)
	}
}

func TestMarkAttendanceRecordsOutcome(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status scheduling.BookingStatus
	}{
		{"completed", scheduling.StatusCompleted},
		{"no_show", scheduling.StatusNoShow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTeacherHarness(t)
			ctx := context.Background()
			slot, err := shared.NewTimeSlot(monday17, monday17.Add(time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			bookingID := shared.ID("b1")
			seedTeacherBooking(t, h, bookingID, shared.ID("u1"), slot)

			if err := h.uc.MarkAttendance(ctx, bookingID, tc.status); err != nil {
				t.Fatalf("MarkAttendance: %v", err)
			}

			b, err := h.bookings.GetByID(ctx, bookingID)
			if err != nil {
				t.Fatal(err)
			}
			if b.Status != tc.status {
				t.Fatalf("status = %q, want %q", b.Status, tc.status)
			}
		})
	}
}

func TestCancelBookingAsTeacherRefundsOneMinuteBefore(t *testing.T) {
	h := newTeacherHarness(t)
	ctx := context.Background()
	studentID := shared.ID("u1")
	bookingID := shared.ID("b1")
	h.clock.NowTime = monday17.Add(-time.Minute)
	slot, err := shared.NewTimeSlot(monday17, monday17.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	seedTeacherBooking(t, h, bookingID, studentID, slot)

	if err := h.uc.CancelBookingAsTeacher(ctx, bookingID); err != nil {
		t.Fatalf("CancelBookingAsTeacher: %v", err)
	}

	ledger, err := h.ledger.Get(ctx, studentID)
	if err != nil {
		t.Fatal(err)
	}
	entries := ledger.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d ledger entries, want exactly one refund", len(entries))
	}
	refund := entries[0]
	if refund.Delta != 1 || refund.Reason != "teacher_cancel" || refund.RefID != bookingID.String() {
		t.Fatalf("refund = %+v, want a +1 teacher_cancel entry for %s", refund, bookingID)
	}
	if !refund.ExpiresAt.IsZero() {
		t.Fatalf("refunded credit expiring at %v, want never-expiring", refund.ExpiresAt)
	}
	if got := ledger.Balance(h.clock.NowTime); got != 1 {
		t.Fatalf("balance = %d, want 1", got)
	}

	b, err := h.bookings.GetByID(ctx, bookingID)
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != scheduling.StatusCancelledByTeacher {
		t.Fatalf("status = %q, want cancelled_by_teacher", b.Status)
	}
}

func TestUpsertProductCreatesThenUpdates(t *testing.T) {
	h := newTeacherHarness(t)
	ctx := context.Background()

	p1, err := catalog.NewProduct(h.idgen.NewID(), "pack-basico", "Pacote Básico", shared.NewMoney(19900), catalog.KindPack, "", 4, 90)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.uc.UpsertProduct(ctx, p1); err != nil {
		t.Fatalf("first UpsertProduct: %v", err)
	}
	all, err := h.products.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != p1.ID {
		t.Fatalf("after create got %+v, want exactly %s", all, p1.ID)
	}

	p2, err := catalog.NewProduct(h.idgen.NewID(), "pack-basico", "Pacote Básico (reajuste)", shared.NewMoney(24900), catalog.KindPack, "", 4, 90)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.uc.UpsertProduct(ctx, p2); err != nil {
		t.Fatalf("second UpsertProduct: %v", err)
	}
	all, err = h.products.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("after update got %d products, want 1", len(all))
	}
	got := all[0]
	if got.ID != p1.ID {
		t.Fatalf("updated product ID = %s, want the original %s preserved", got.ID, p1.ID)
	}
	if got.Title != p2.Title || got.Price != p2.Price {
		t.Fatalf("updated product = %+v, want %q at %d", got, p2.Title, p2.Price)
	}
}

// A double-clicked Cancel button must not hand out two credits for one lesson.
func TestCancelBookingAsTeacherRefundsOnlyOnce(t *testing.T) {
	h := newTeacherHarness(t)
	ctx := context.Background()
	student := shared.ID("student-1")

	start := h.clock.NowTime.Add(2 * time.Hour)
	booking := scheduling.Booking{
		ID:        shared.ID("b1"),
		StudentID: student,
		Slot:      shared.TimeSlot{Start: start, End: start.Add(time.Hour)},
		Status:    scheduling.StatusScheduled,
	}
	if err := h.bookings.Create(ctx, booking); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if err := h.uc.CancelBookingAsTeacher(ctx, booking.ID); err != nil {
			t.Fatalf("cancel %d: %v", i, err)
		}
	}

	ledger, err := h.ledger.Get(ctx, student)
	if err != nil {
		t.Fatal(err)
	}
	if got := ledger.Balance(h.clock.NowTime); got != 1 {
		t.Fatalf("balance = %d after three cancels of one booking, want 1", got)
	}
}

// Attendance is only meaningful for a lesson that was still scheduled.
func TestMarkAttendanceRejectsACancelledBooking(t *testing.T) {
	h := newTeacherHarness(t)
	ctx := context.Background()

	start := h.clock.NowTime.Add(2 * time.Hour)
	booking := scheduling.Booking{
		ID:        shared.ID("b2"),
		StudentID: shared.ID("student-1"),
		Slot:      shared.TimeSlot{Start: start, End: start.Add(time.Hour)},
		Status:    scheduling.StatusCancelledByStudent,
	}
	if err := h.bookings.Create(ctx, booking); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.MarkAttendance(ctx, booking.ID, scheduling.StatusCompleted); err == nil {
		t.Fatal("a cancelled booking was marked completed")
	}
}
