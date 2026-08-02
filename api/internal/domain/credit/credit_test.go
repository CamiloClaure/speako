package credit

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func utc(y int, m time.Month, d int, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
}

func jan(day, hour int) time.Time { return utc(2026, time.January, day, hour) }
func feb(day, hour int) time.Time { return utc(2026, time.February, day, hour) }

func TestBalanceSumsNonExpired(t *testing.T) {
	l := NewLedger()
	l = l.Append(Entry{Delta: 4, Reason: "pack", RefID: "o1", ExpiresAt: feb(1, 0)})
	l = l.Append(Entry{Delta: 2, Reason: "pack", RefID: "o2", ExpiresAt: jan(31, 0)})
	if got := l.Balance(jan(15, 0)); got != 6 {
		t.Fatalf("Balance = %d, want 6", got)
	}
}

func TestExpiryBoundarySecond(t *testing.T) {
	expires := jan(15, 12)
	l := NewLedger(Entry{Delta: 1, Reason: "pack", RefID: "o1", ExpiresAt: expires})
	tests := []struct {
		name string
		now  time.Time
		want int
	}{
		{"one second before expiry", expires.Add(-time.Second), 1},
		{"at the exact expiry second", expires, 0},
		{"one second after expiry", expires.Add(time.Second), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := l.Balance(tt.now); got != tt.want {
				t.Fatalf("Balance(%v) = %d, want %d", tt.now, got, tt.want)
			}
		})
	}
}

func TestNeverExpiringCreditsAlwaysCount(t *testing.T) {
	l := NewLedger(Entry{Delta: 3, Reason: "bonus", RefID: "o1"})
	if got := l.Balance(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); got != 3 {
		t.Fatalf("never-expiring credits must always count, got %d", got)
	}
}

func TestDebitInsufficient(t *testing.T) {
	l := NewLedger(Entry{Delta: 2, Reason: "pack", RefID: "o1", ExpiresAt: feb(1, 0)})
	got, err := l.Debit(3, "booking", "b1", jan(15, 0))
	if !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("Debit(3) error = %v, want %v", err, ErrInsufficientCredits)
	}
	if !slices.Equal(got.Entries(), l.Entries()) {
		t.Fatal("failed debit must leave the ledger unchanged")
	}
}

func TestDebitInvalidAmount(t *testing.T) {
	l := NewLedger()
	for _, amount := range []int{0, -1} {
		if _, err := l.Debit(amount, "booking", "b1", jan(1, 0)); !errors.Is(err, ErrInvalidDebit) {
			t.Fatalf("Debit(%d) error = %v, want %v", amount, err, ErrInvalidDebit)
		}
	}
}

func TestDebitConsumesSoonestExpiringFirst(t *testing.T) {
	l := NewLedger(
		Entry{Delta: 2, Reason: "pack", RefID: "o1", ExpiresAt: jan(31, 0)},
		Entry{Delta: 2, Reason: "pack", RefID: "o2", ExpiresAt: feb(28, 0)},
	)

	// Two credits on Jan 15 must eat the soonest-expiring lot (Jan 31).
	l, err := l.Debit(2, "booking", "b1", jan(15, 0))
	if err != nil {
		t.Fatal(err)
	}

	// The consumption line inherits the consumed lot's expiry.
	consumed := Entry{Delta: -2, Reason: "booking", RefID: "b1", ExpiresAt: jan(31, 0)}
	if !slices.Contains(l.Entries(), consumed) {
		t.Fatalf("expected consumption entry %+v, got %+v", consumed, l.Entries())
	}

	// If the wrong lot had been consumed, the remaining Jan-31 credits would
	// expire and the Feb-28 lot would be spent: balance at Feb 1 would be 0.
	if got := l.Balance(feb(1, 0)); got != 2 {
		t.Fatalf("Balance(Feb 1) = %d, want 2 (soonest-expiring credits were consumed)", got)
	}
}

func TestDebitSpansLotsSoonestFirst(t *testing.T) {
	l := NewLedger(
		Entry{Delta: 1, Reason: "pack", RefID: "o1", ExpiresAt: jan(31, 0)},
		Entry{Delta: 1, Reason: "pack", RefID: "o2", ExpiresAt: feb(28, 0)},
		Entry{Delta: 1, Reason: "bonus", RefID: "o3"},
	)
	l, err := l.Debit(2, "booking", "b1", jan(15, 0))
	if err != nil {
		t.Fatal(err)
	}
	entries := l.Entries()
	if !slices.Contains(entries, Entry{Delta: -1, Reason: "booking", RefID: "b1", ExpiresAt: jan(31, 0)}) {
		t.Fatalf("debit must consume the Jan lot first, got %+v", entries)
	}
	if !slices.Contains(entries, Entry{Delta: -1, Reason: "booking", RefID: "b1", ExpiresAt: feb(28, 0)}) {
		t.Fatalf("debit must then consume the Feb lot, got %+v", entries)
	}
	// The never-expiring credit must be the last one standing.
	if got := l.Balance(feb(1, 0)); got != 1 {
		t.Fatalf("Balance(Feb 1) = %d, want 1 (only the never-expiring credit remains)", got)
	}
}

func TestDebitPartialLots(t *testing.T) {
	l := NewLedger(Entry{Delta: 5, Reason: "pack", RefID: "o1", ExpiresAt: feb(1, 0)})
	l, err := l.Debit(2, "booking", "b1", jan(15, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Balance(jan(16, 0)); got != 3 {
		t.Fatalf("Balance = %d, want 3", got)
	}
	// A second debit consumes from the same lot.
	l, err = l.Debit(3, "booking", "b2", jan(17, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Balance(jan(18, 0)); got != 0 {
		t.Fatalf("Balance = %d, want 0", got)
	}
	if _, err := l.Debit(1, "booking", "b3", jan(18, 0)); !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("debit beyond balance error = %v, want %v", err, ErrInsufficientCredits)
	}
}

func TestDebitOnlyAgainstLiveCredits(t *testing.T) {
	l := NewLedger(
		Entry{Delta: 1, Reason: "pack", RefID: "o1", ExpiresAt: jan(31, 0)},
		Entry{Delta: 1, Reason: "pack", RefID: "o2", ExpiresAt: feb(28, 0)},
	)
	// At Feb 1 the Jan lot is dead; only one credit is available.
	if _, err := l.Debit(2, "booking", "b1", feb(1, 0)); !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("error = %v, want %v", err, ErrInsufficientCredits)
	}
	l, err := l.Debit(1, "booking", "b1", feb(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Balance(feb(2, 0)); got != 0 {
		t.Fatalf("Balance = %d, want 0", got)
	}
}

func TestBalanceNeverNegativeAcrossSequence(t *testing.T) {
	l := NewLedger()
	var check = func(now time.Time, want int) {
		t.Helper()
		if got := l.Balance(now); got != want {
			t.Fatalf("Balance(%v) = %d, want %d", now, got, want)
		}
		if got := l.Balance(now); got < 0 {
			t.Fatalf("balance went negative at %v: %d", now, got)
		}
	}
	l = l.Append(Entry{Delta: 4, Reason: "pack", RefID: "o1", ExpiresAt: jan(31, 0)})
	l = l.Append(Entry{Delta: 4, Reason: "pack", RefID: "o2", ExpiresAt: feb(28, 0)})
	check(jan(1, 0), 8)

	l, err := l.Debit(6, "booking", "b1", jan(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	check(jan(15, 0), 2)

	// Six of eight credits were consumed, so the four Jan credits are gone;
	// the remaining two live in the Feb lot.
	check(feb(1, 0), 2)

	l, err = l.Debit(2, "booking", "b2", feb(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	check(feb(3, 0), 0)
}

func TestAppendCopies(t *testing.T) {
	now := jan(1, 0)
	src := NewLedger()
	out := src.Append(Entry{Delta: 1, Reason: "pack", RefID: "o1"})
	if got := src.Balance(now); got != 0 {
		t.Fatal("Append leaked entries into the source ledger")
	}
	if got := out.Balance(now); got != 1 {
		t.Fatalf("appended ledger balance = %d, want 1", got)
	}
}
