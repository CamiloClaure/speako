package scheduling

import (
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func saoPaulo(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("load America/Sao_Paulo: %v", err)
	}
	return loc
}

func mondayRule(startMin, endMin, slotMin int) AvailabilityRule {
	r, err := NewAvailabilityRule(time.Monday, startMin, endMin, slotMin)
	if err != nil {
		panic(err)
	}
	return r
}

func assertSlots(t *testing.T, got []shared.TimeSlot, want ...shared.TimeSlot) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d slots, want %d\n got: %v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].Start.Equal(want[i].Start) || !got[i].End.Equal(want[i].End) {
			t.Fatalf("slot %d = [%v, %v], want [%v, %v]", i, got[i].Start, got[i].End, want[i].Start, want[i].End)
		}
		if loc := got[i].Start.Location(); loc.String() != "UTC" {
			t.Fatalf("slot %d not in UTC, got location %v", i, loc)
		}
	}
}

func TestGenerateSlotsBasic(t *testing.T) {
	loc := saoPaulo(t)
	got := GenerateSlots(
		[]AvailabilityRule{mondayRule(14*60, 16*60, 60)},
		nil, nil,
		utc(2026, time.January, 5, 0, 0), // Monday
		utc(2026, time.January, 12, 0, 0),
		loc,
	)
	// 14:00 and 15:00 local (UTC-3) on Monday Jan 5.
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 5, 17, 0), End: utc(2026, time.January, 5, 18, 0)},
		shared.TimeSlot{Start: utc(2026, time.January, 5, 18, 0), End: utc(2026, time.January, 5, 19, 0)},
	)
}

func TestGenerateSlotsPartialWindowsDropped(t *testing.T) {
	loc := saoPaulo(t)
	// 14:00-15:50 with 45-min slots: 14:00 and 14:45 fit, 15:30 would end at
	// 16:15 > 15:50, so the final partial slot is dropped.
	got := GenerateSlots(
		[]AvailabilityRule{mondayRule(14*60, 14*60+110, 45)},
		nil, nil,
		utc(2026, time.January, 5, 0, 0),
		utc(2026, time.January, 12, 0, 0),
		loc,
	)
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 5, 17, 0), End: utc(2026, time.January, 5, 17, 45)},
		shared.TimeSlot{Start: utc(2026, time.January, 5, 17, 45), End: utc(2026, time.January, 5, 18, 30)},
	)
}

func TestGenerateSlotsAcrossMonthBoundary(t *testing.T) {
	loc := saoPaulo(t)
	friday, err := NewAvailabilityRule(time.Friday, 14*60, 15*60, 60)
	if err != nil {
		t.Fatal(err)
	}
	got := GenerateSlots(
		[]AvailabilityRule{friday, mondayRule(14*60, 15*60, 60)},
		nil, nil,
		utc(2026, time.January, 30, 0, 0), // Friday
		utc(2026, time.February, 3, 0, 0),
		loc,
	)
	// Slots on Jan 30 (Friday) and Feb 2 (Monday) - across the month seam.
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 30, 17, 0), End: utc(2026, time.January, 30, 18, 0)},
		shared.TimeSlot{Start: utc(2026, time.February, 2, 17, 0), End: utc(2026, time.February, 2, 18, 0)},
	)
}

func TestGenerateSlotsBlockedExceptionSubtractsDay(t *testing.T) {
	loc := saoPaulo(t)
	blocked, err := NewException(ExceptionDate(2026, time.January, 5, loc), true, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := GenerateSlots(
		[]AvailabilityRule{mondayRule(14*60, 16*60, 60)},
		[]Exception{blocked},
		nil,
		utc(2026, time.January, 5, 0, 0), // Monday, blocked
		utc(2026, time.January, 13, 0, 0),
		loc,
	)
	// Jan 5 is subtracted; the following Monday Jan 12 still happens.
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 12, 17, 0), End: utc(2026, time.January, 12, 18, 0)},
		shared.TimeSlot{Start: utc(2026, time.January, 12, 18, 0), End: utc(2026, time.January, 12, 19, 0)},
	)
}

func TestGenerateSlotsReplacingException(t *testing.T) {
	loc := saoPaulo(t)
	replaced, err := NewException(ExceptionDate(2026, time.January, 12, loc), false, 10*60, 11*60)
	if err != nil {
		t.Fatal(err)
	}
	got := GenerateSlots(
		[]AvailabilityRule{mondayRule(14*60, 16*60, 60)},
		[]Exception{replaced},
		nil,
		utc(2026, time.January, 5, 0, 0), // Monday
		utc(2026, time.January, 13, 0, 0),
		loc,
	)
	// Jan 5 keeps the rule's two slots; Jan 12 is replaced by a single 10:00 slot.
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 5, 17, 0), End: utc(2026, time.January, 5, 18, 0)},
		shared.TimeSlot{Start: utc(2026, time.January, 5, 18, 0), End: utc(2026, time.January, 5, 19, 0)},
		shared.TimeSlot{Start: utc(2026, time.January, 12, 13, 0), End: utc(2026, time.January, 12, 14, 0)},
	)
}

func TestGenerateSlotsReplacingExceptionWithoutWeekdayRule(t *testing.T) {
	loc := saoPaulo(t)
	// Tuesday has no rule, so a replacing exception on it has no slot length
	// to draw from and must produce nothing.
	replaced, err := NewException(ExceptionDate(2026, time.January, 6, loc), false, 10*60, 12*60)
	if err != nil {
		t.Fatal(err)
	}
	got := GenerateSlots(
		[]AvailabilityRule{mondayRule(14*60, 16*60, 60)},
		[]Exception{replaced},
		nil,
		utc(2026, time.January, 5, 0, 0),
		utc(2026, time.January, 12, 0, 0),
		loc,
	)
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 5, 17, 0), End: utc(2026, time.January, 5, 18, 0)},
		shared.TimeSlot{Start: utc(2026, time.January, 5, 18, 0), End: utc(2026, time.January, 5, 19, 0)},
	)
}

func TestGenerateSlotsBlockedExceptionBeatsReplacement(t *testing.T) {
	loc := saoPaulo(t)
	blocked, _ := NewException(ExceptionDate(2026, time.January, 12, loc), true, 0, 0)
	replaced, _ := NewException(ExceptionDate(2026, time.January, 12, loc), false, 10*60, 12*60)
	got := GenerateSlots(
		[]AvailabilityRule{mondayRule(14*60, 16*60, 60)},
		[]Exception{replaced, blocked},
		nil,
		utc(2026, time.January, 5, 0, 0),
		utc(2026, time.January, 13, 0, 0),
		loc,
	)
	// Jan 12 is fully removed; only Jan 5 survives.
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 5, 17, 0), End: utc(2026, time.January, 5, 18, 0)},
		shared.TimeSlot{Start: utc(2026, time.January, 5, 18, 0), End: utc(2026, time.January, 5, 19, 0)},
	)
}

func TestGenerateSlotsRemovesBooked(t *testing.T) {
	loc := saoPaulo(t)
	booked := []shared.TimeSlot{
		{Start: utc(2026, time.January, 5, 17, 0), End: utc(2026, time.January, 5, 18, 0)},
	}
	got := GenerateSlots(
		[]AvailabilityRule{mondayRule(14*60, 16*60, 60)},
		nil, booked,
		utc(2026, time.January, 5, 0, 0),
		utc(2026, time.January, 12, 0, 0),
		loc,
	)
	// The 14:00 slot is booked; only the 15:00 slot remains.
	assertSlots(t, got,
		shared.TimeSlot{Start: utc(2026, time.January, 5, 18, 0), End: utc(2026, time.January, 5, 19, 0)},
	)
}

func TestNewAvailabilityRuleValidation(t *testing.T) {
	tests := []struct {
		name                      string
		weekday                   time.Weekday
		startMin, endMin, slotMin int
		wantErr                   error
	}{
		{"valid", time.Monday, 9 * 60, 10 * 60, 30, nil},
		{"sunday valid", time.Sunday, 0, 30, 30, nil},
		{"saturday valid", time.Saturday, 9 * 60, 10 * 60, 60, nil},
		{"weekday too high", time.Weekday(7), 9 * 60, 10 * 60, 30, ErrInvalidRule},
		{"weekday too low", time.Weekday(-1), 9 * 60, 10 * 60, 30, ErrInvalidRule},
		{"negative start", time.Monday, -1, 10 * 60, 30, ErrInvalidRule},
		{"empty window", time.Monday, 10 * 60, 10 * 60, 30, ErrInvalidRule},
		{"inverted window", time.Monday, 11 * 60, 10 * 60, 30, ErrInvalidRule},
		{"past midnight", time.Monday, 9 * 60, 25 * 60, 30, ErrInvalidRule},
		{"zero slot length", time.Monday, 9 * 60, 10 * 60, 0, ErrInvalidRule},
		{"slot longer than window", time.Monday, 9 * 60, 10 * 60, 61, ErrInvalidRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAvailabilityRule(tt.weekday, tt.startMin, tt.endMin, tt.slotMin)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewAvailabilityRule error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewExceptionValidation(t *testing.T) {
	loc := saoPaulo(t)
	date := ExceptionDate(2026, time.January, 5, loc)
	tests := []struct {
		name    string
		date    time.Time
		blocked bool
		start   int
		end     int
		wantErr error
	}{
		{"blocked valid", date, true, 0, 0, nil},
		{"replacement valid", date, false, 10 * 60, 11 * 60, nil},
		{"zero date", time.Time{}, false, 10 * 60, 11 * 60, ErrInvalidException},
		{"replacement empty window", date, false, 11 * 60, 11 * 60, ErrInvalidException},
		{"replacement inverted window", date, false, 12 * 60, 10 * 60, ErrInvalidException},
		{"replacement past midnight", date, false, 10 * 60, 25 * 60, ErrInvalidException},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewException(tt.date, tt.blocked, tt.start, tt.end)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewException error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCanBook(t *testing.T) {
	slotStart := utc(2026, time.January, 5, 14, 0)
	const minNotice = 12 * time.Hour
	tests := []struct {
		name string
		now  time.Time
		want error
	}{
		{"well in advance", slotStart.Add(-24 * time.Hour), nil},
		{"at exactly min notice", slotStart.Add(-minNotice), nil},
		{"one second early is fine", slotStart.Add(-minNotice - time.Second), nil},
		{"one second late", slotStart.Add(-minNotice + time.Second), ErrTooLateToBook},
		{"after the start", slotStart.Add(time.Hour), ErrTooLateToBook},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CanBook(slotStart, tt.now, minNotice); !errors.Is(err, tt.want) {
				t.Fatalf("CanBook error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewBookingEnforcesNotice(t *testing.T) {
	slot := shared.TimeSlot{Start: utc(2026, time.January, 5, 14, 0), End: utc(2026, time.January, 5, 15, 0)}
	if _, err := NewBooking(shared.ID("b1"), shared.ID("u1"), slot, slot.Start.Add(-2*time.Hour), 12*time.Hour); !errors.Is(err, ErrTooLateToBook) {
		t.Fatalf("NewBooking error = %v, want %v", err, ErrTooLateToBook)
	}
	ok, err := NewBooking(shared.ID("b1"), shared.ID("u1"), slot, slot.Start.Add(-24*time.Hour), 12*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if ok.Status != StatusScheduled {
		t.Fatalf("Status = %q, want scheduled", ok.Status)
	}
}

func TestCancelOutcome(t *testing.T) {
	slot := shared.TimeSlot{Start: utc(2026, time.January, 5, 14, 0), End: utc(2026, time.January, 5, 15, 0)}
	b := Booking{ID: shared.ID("b1"), StudentID: shared.ID("u1"), Slot: slot, Status: StatusScheduled}
	deadline := slot.Start.Add(-24 * time.Hour)

	tests := []struct {
		name string
		now  time.Time
		by   CancelActor
		want CancelDecision
	}{
		{"student at exactly 24h refunds", deadline, CancelledByStudent, RefundCredit},
		{"student at 24h and a second ago refunds", deadline.Add(-time.Second), CancelledByStudent, RefundCredit},
		{"student at 23h59m59s consumes", deadline.Add(time.Second), CancelledByStudent, ConsumeCredit},
		{"student after the slot consumes", slot.Start.Add(time.Hour), CancelledByStudent, ConsumeCredit},
		{"teacher at 23h59m59s refunds", deadline.Add(time.Second), CancelledByTeacher, RefundCredit},
		{"teacher after the slot refunds", slot.Start.Add(time.Hour), CancelledByTeacher, RefundCredit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CancelOutcome(b, tt.now, tt.by, 24*time.Hour); got != tt.want {
				t.Fatalf("CancelOutcome = %q, want %q", got, tt.want)
			}
		})
	}
}
