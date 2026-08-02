package shared

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMoneyAdd(t *testing.T) {
	tests := []struct {
		name string
		a, b Money
		want Money
	}{
		{"zero", NewMoney(0), NewMoney(0), NewMoney(0)},
		{"carry to cents", NewMoney(150), NewMoney(150), NewMoney(300)},
		{"cents overflow into reais", NewMoney(199), NewMoney(1), NewMoney(200)},
		{"large", NewMoney(123456), NewMoney(654321), NewMoney(777777)},
		{"negative", NewMoney(-50), NewMoney(20), NewMoney(-30)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Add(tt.b); got != tt.want {
				t.Fatalf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMoneyMul(t *testing.T) {
	tests := []struct {
		name    string
		m       Money
		qty     int
		want    Money
		wantErr error
	}{
		{"by one", NewMoney(123), 1, NewMoney(123), nil},
		{"by zero", NewMoney(999), 0, NewMoney(0), nil},
		{"by quantity", NewMoney(12345), 3, NewMoney(37035), nil},
		{"negative quantity rejected", NewMoney(100), -1, 0, ErrQuantityTooSmall},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.m.Mul(tt.qty)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Mul(%d, %d) error = %v, want %v", tt.m, tt.qty, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Mul(%d, %d) = %d, want %d", tt.m, tt.qty, got, tt.want)
			}
		})
	}
}

func TestMoneyString(t *testing.T) {
	tests := []struct {
		m    Money
		want string
	}{
		{NewMoney(0), "R$ 0,00"},
		{NewMoney(5), "R$ 0,05"},
		{NewMoney(50), "R$ 0,50"},
		{NewMoney(100), "R$ 1,00"},
		{NewMoney(123456), "R$ 1.234,56"},
		{NewMoney(1000000), "R$ 10.000,00"},
		{NewMoney(100000000), "R$ 1.000.000,00"},
		{NewMoney(-100), "R$ -1,00"},
		{NewMoney(99999999), "R$ 999.999,99"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.m.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMoneyNoFloats(t *testing.T) {
	// The arithmetic path must stay in integer land: a 1/3 of a cent is
	// impossible, and adding a third of a cent repeatedly must not drift.
	base := NewMoney(100)
	third, err := base.Mul(0)
	if err != nil {
		t.Fatalf("Mul(0): %v", err)
	}
	if third != NewMoney(0) {
		t.Fatalf("expected integer zero, got %d", third)
	}
	got := NewMoney(1)
	for range 1000 {
		got = got.Add(NewMoney(1))
	}
	if got != NewMoney(1001) {
		t.Fatalf("accumulated %d, want 1001", got)
	}
}

func TestParseEmail(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr error
	}{
		{"teacher@speako.com.br", "teacher@speako.com.br", nil},
		{"  Teacher@Speako.COM.BR  ", "teacher@speako.com.br", nil},
		{"a.b+c@example.co", "a.b+c@example.co", nil},
		{"", "", ErrInvalidEmail},
		{"   ", "", ErrInvalidEmail},
		{"missing-at", "", ErrInvalidEmail},
		{"@example.com", "", ErrInvalidEmail},
		{"user@", "", ErrInvalidEmail},
		{"user@@example.com", "", ErrInvalidEmail},
		{"user@example", "", ErrInvalidEmail},
		{"user name@example.com", "", ErrInvalidEmail},
		{"user@exa mple.com", "", ErrInvalidEmail},
		{"user..name@example.com", "", ErrInvalidEmail},
		{".user@example.com", "", ErrInvalidEmail},
		{"user.@example.com", "", ErrInvalidEmail},
		{"user@.example.com", "", ErrInvalidEmail},
		{"user@example.com.", "", ErrInvalidEmail},
		{"user@-example.com", "", ErrInvalidEmail},
		{"user@example-.com", "", ErrInvalidEmail},
		{"user@example.com.br", "user@example.com.br", nil},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseEmail(tt.raw)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParseEmail(%q) error = %v, want %v", tt.raw, err, tt.wantErr)
			}
			if err == nil && string(got) != tt.want {
				t.Fatalf("ParseEmail(%q) = %q, want %q", tt.raw, got, tt.want)
			}
			if err == nil && strings.ToLower(string(got)) != string(got) {
				t.Fatalf("ParseEmail(%q) not normalized: %q", tt.raw, got)
			}
		})
	}
}

func TestNewTimeSlot(t *testing.T) {
	start := time.Date(2026, 1, 5, 14, 0, 0, 0, time.UTC)
	after := start.Add(30 * time.Minute)
	tests := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantErr error
	}{
		{"valid", start, after, nil},
		{"empty start", time.Time{}, after, ErrInvalidTimeSlot},
		{"empty end", start, time.Time{}, ErrInvalidTimeSlot},
		{"zero length", start, start, ErrInvalidTimeSlot},
		{"backwards", after, start, ErrInvalidTimeSlot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTimeSlot(tt.start, tt.end)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewTimeSlot error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTimeSlotOverlaps(t *testing.T) {
	base := time.Date(2026, 1, 5, 14, 0, 0, 0, time.UTC)
	slot := TimeSlot{Start: base, End: base.Add(time.Hour)}
	tests := []struct {
		name string
		o    TimeSlot
		want bool
	}{
		{"identical", TimeSlot{Start: base, End: base.Add(time.Hour)}, true},
		{"inside", TimeSlot{Start: base.Add(10 * time.Minute), End: base.Add(50 * time.Minute)}, true},
		{"overlapping tail", TimeSlot{Start: base.Add(50 * time.Minute), End: base.Add(90 * time.Minute)}, true},
		{"touching at end", TimeSlot{Start: base.Add(time.Hour), End: base.Add(2 * time.Hour)}, false},
		{"touching at start", TimeSlot{Start: base.Add(-time.Hour), End: base}, false},
		{"disjoint before", TimeSlot{Start: base.Add(-2 * time.Hour), End: base.Add(-time.Hour)}, false},
		{"disjoint after", TimeSlot{Start: base.Add(2 * time.Hour), End: base.Add(3 * time.Hour)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slot.Overlaps(tt.o); got != tt.want {
				t.Fatalf("Overlaps(%v) = %v, want %v", tt.o, got, tt.want)
			}
		})
	}
}

func TestDomainErrorMatchesWithErrorsIs(t *testing.T) {
	err := ErrInvalidEmail
	wrapped := &errWrapper{err: err}
	if !errors.Is(wrapped, ErrInvalidEmail) {
		t.Fatal("errors.Is on a wrapped DomainError must match the sentinel")
	}
	if errors.Is(wrapped, ErrInvalidTimeSlot) {
		t.Fatal("different DomainError kinds must not match")
	}
	if err.Kind() != "email.invalid" {
		t.Fatalf("Kind() = %q, want email.invalid", err.Kind())
	}
}

type errWrapper struct {
	err error
}

func (w *errWrapper) Error() string { return "wrapped: " + w.err.Error() }
func (w *errWrapper) Unwrap() error { return w.err }
