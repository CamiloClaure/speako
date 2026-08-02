package scheduling

import (
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	// ErrInvalidRule is returned when an availability rule is malformed.
	ErrInvalidRule = shared.NewDomainError("rule.invalid", "invalid availability rule")
	// ErrInvalidException is returned when an exception is malformed.
	ErrInvalidException = shared.NewDomainError("exception.invalid", "invalid availability exception")
)

// AvailabilityRule is one weekly availability window, authored in the
// teacher's local timezone. Weekday follows time.Weekday (Sunday is 0);
// StartMin/EndMin are minutes since local midnight, with EndMin exclusive and
// at most 1440. SlotMin is the lesson length in minutes.
type AvailabilityRule struct {
	Weekday  time.Weekday
	StartMin int
	EndMin   int
	SlotMin  int
}

// NewAvailabilityRule validates a rule.
func NewAvailabilityRule(weekday time.Weekday, startMin, endMin, slotMin int) (AvailabilityRule, error) {
	if weekday < time.Sunday || weekday > time.Saturday {
		return AvailabilityRule{}, ErrInvalidRule
	}
	if startMin < 0 || endMin <= startMin || endMin > 1440 {
		return AvailabilityRule{}, ErrInvalidRule
	}
	if slotMin <= 0 || slotMin > endMin-startMin {
		return AvailabilityRule{}, ErrInvalidRule
	}
	return AvailabilityRule{Weekday: weekday, StartMin: startMin, EndMin: endMin, SlotMin: slotMin}, nil
}

// Exception overrides a single calendar date. Date must carry the intended
// calendar date in the teacher's timezone (build it with ExceptionDate).
// Blocked removes the whole day; otherwise the day is replaced by the
// [StartMin, EndMin) window.
type Exception struct {
	Date     time.Time
	Blocked  bool
	StartMin int
	EndMin   int
}

// ExceptionDate builds a calendar date in loc, ready to store in an
// Exception. Build the date in loc so that comparing it with a slot's local
// date is unambiguous regardless of the stored timezone.
func ExceptionDate(year int, month time.Month, day int, loc *time.Location) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// NewException validates an exception.
func NewException(date time.Time, blocked bool, startMin, endMin int) (Exception, error) {
	if date.IsZero() {
		return Exception{}, ErrInvalidException
	}
	if !blocked {
		if startMin < 0 || endMin <= startMin || endMin > 1440 {
			return Exception{}, ErrInvalidException
		}
	}
	return Exception{Date: date, Blocked: blocked, StartMin: startMin, EndMin: endMin}, nil
}

// GenerateSlots produces every available slot in [from, to), as UTC times.
//
// Rules are authored in loc; each calendar day in the range contributes the
// windows of the rules matching its weekday. A blocked exception subtracts the
// day, a replacing exception replaces it with a single window whose slot
// length comes from the weekday rule (if the weekday has no rule, the
// exception produces no slots). Slots that overlap any booked slot are
// removed.
func GenerateSlots(rules []AvailabilityRule, exceptions []Exception, booked []shared.TimeSlot, from, to time.Time, loc *time.Location) []shared.TimeSlot {
	var out []shared.TimeSlot
	first := midnight(from, loc)
	last := midnight(to, loc)
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		exc, hasExc := exceptionOn(exceptions, day, loc)
		switch {
		case hasExc && exc.Blocked:
			continue
		case hasExc:
			rule, hasRule := ruleFor(rules, day.Weekday())
			if !hasRule {
				continue
			}
			out = append(out, windowSlots(day, exc.StartMin, exc.EndMin, rule.SlotMin)...)
		default:
			for _, r := range rules {
				if r.Weekday == day.Weekday() {
					out = append(out, windowSlots(day, r.StartMin, r.EndMin, r.SlotMin)...)
				}
			}
		}
	}

	kept := out[:0]
	for _, s := range out {
		s = shared.TimeSlot{Start: s.Start.UTC(), End: s.End.UTC()}
		if s.Start.Before(from) || s.End.After(to) {
			continue
		}
		if overlapsAny(s, booked) {
			continue
		}
		kept = append(kept, s)
	}
	return kept
}

// midnight returns the local midnight of t's calendar date in loc.
func midnight(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// windowSlots lays SlotMin-long slots over [startMin, endMin) minutes past
// local midnight, dropping any final partial slot.
func windowSlots(localMidnight time.Time, startMin, endMin, slotMin int) []shared.TimeSlot {
	var out []shared.TimeSlot
	for start := startMin; start+slotMin <= endMin; start += slotMin {
		s := localMidnight.Add(time.Duration(start) * time.Minute)
		out = append(out, shared.TimeSlot{Start: s, End: s.Add(time.Duration(slotMin) * time.Minute)})
	}
	return out
}

func exceptionOn(exceptions []Exception, dayMidnight time.Time, loc *time.Location) (Exception, bool) {
	dy, dm, dd := dayMidnight.Date()
	var replacement Exception
	hasReplacement := false
	for _, e := range exceptions {
		ey, em, ed := e.Date.In(loc).Date()
		if ey != dy || em != dm || ed != dd {
			continue
		}
		if e.Blocked {
			return e, true
		}
		replacement = e
		hasReplacement = true
	}
	return replacement, hasReplacement
}

func ruleFor(rules []AvailabilityRule, weekday time.Weekday) (AvailabilityRule, bool) {
	for _, r := range rules {
		if r.Weekday == weekday {
			return r, true
		}
	}
	return AvailabilityRule{}, false
}

func overlapsAny(s shared.TimeSlot, booked []shared.TimeSlot) bool {
	for _, b := range booked {
		if s.Overlaps(b) {
			return true
		}
	}
	return false
}
