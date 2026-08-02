package credit

import (
	"slices"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	// ErrInsufficientCredits is returned when a debit would drive the balance
	// below zero.
	ErrInsufficientCredits = shared.NewDomainError("credit.insufficient", "not enough non-expired credits")
	// ErrInvalidDebit is returned for a debit of zero or negative credits.
	ErrInvalidDebit = shared.NewDomainError("credit.debit", "debit must be a positive amount")
)

// Entry is one append-only line in the ledger. A positive Delta grants
// credits; a negative Delta consumes them. ExpiresAt is the expiry of the
// credit lot involved (zero means never expires). A consumption entry carries
// the same ExpiresAt as the lot it consumed, so the two cancel out and then
// fall off the balance together.
type Entry struct {
	Delta     int
	Reason    string
	RefID     string
	ExpiresAt time.Time
}

// Ledger is the append-only credit ledger. Balance is the sum of non-expired
// entries and is never negative. Debits consume the soonest-expiring credits
// first.
type Ledger struct {
	entries []Entry
}

// NewLedger returns an empty ledger. entries may seed it, for example when a
// stored ledger is reconstructed; the slice is copied.
func NewLedger(entries ...Entry) Ledger {
	return Ledger{entries: append([]Entry(nil), entries...)}
}

// Entries returns a copy of the ledger lines.
func (l Ledger) Entries() []Entry {
	return append([]Entry(nil), l.entries...)
}

// Append adds one line and returns the new ledger.
func (l Ledger) Append(e Entry) Ledger {
	out := append(append([]Entry(nil), l.entries...), e)
	return Ledger{entries: out}
}

// Balance is the sum of entries whose ExpiresAt is zero or after now. An
// entry expiring exactly at now is already excluded.
func (l Ledger) Balance(now time.Time) int {
	sum := 0
	for _, e := range l.entries {
		if e.ExpiresAt.IsZero() || e.ExpiresAt.After(now) {
			sum += e.Delta
		}
	}
	return sum
}

// Debit consumes amount credits, starting with the soonest-expiring lots and
// finishing with never-expiring ones. It fails with ErrInsufficientCredits
// when the balance at now cannot cover the amount; on failure the ledger is
// returned unchanged. On success the original ledger is also unchanged, and
// the returned ledger gains one consumption entry per lot touched.
func (l Ledger) Debit(amount int, reason, refID string, now time.Time) (Ledger, error) {
	if amount <= 0 {
		return l, ErrInvalidDebit
	}
	if l.Balance(now) < amount {
		return l, ErrInsufficientCredits
	}
	out := append([]Entry(nil), l.entries...)
	remaining := amount
	for _, exp := range lotExpiries(l.entries, now) {
		if remaining == 0 {
			break
		}
		available := 0
		for _, e := range l.entries {
			if !e.ExpiresAt.Equal(exp) {
				continue
			}
			if !e.ExpiresAt.IsZero() && !e.ExpiresAt.After(now) {
				continue
			}
			available += e.Delta
		}
		if available <= 0 {
			continue
		}
		take := min(remaining, available)
		out = append(out, Entry{Delta: -take, Reason: reason, RefID: refID, ExpiresAt: exp})
		remaining -= take
	}
	// Balance said the credits were there but the lots could not supply them.
	// Reachable only if a lot was driven negative by a raw Append, and a silent
	// under-debit would hand out a free lesson - so refuse instead.
	if remaining > 0 {
		return l, ErrInsufficientCredits
	}
	return Ledger{entries: out}, nil
}

// lotExpiries lists the distinct non-expired expiry values present in entries,
// soonest first, with never-expiring (zero) lots last.
func lotExpiries(entries []Entry, now time.Time) []time.Time {
	seen := map[time.Time]bool{}
	var exps []time.Time
	for _, e := range entries {
		if !e.ExpiresAt.IsZero() && !e.ExpiresAt.After(now) {
			continue
		}
		if !seen[e.ExpiresAt] {
			seen[e.ExpiresAt] = true
			exps = append(exps, e.ExpiresAt)
		}
	}
	slices.SortFunc(exps, func(a, b time.Time) int {
		aZero, bZero := a.IsZero(), b.IsZero()
		switch {
		case aZero && bZero:
			return 0
		case aZero:
			return 1
		case bZero:
			return -1
		default:
			return a.Compare(b)
		}
	})
	return exps
}
