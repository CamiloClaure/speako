package shared

import (
	"strconv"
	"strings"
	"time"
)

// DomainError is the sentinel base type for every business-rule failure.
// Packages define their own sentinel values with NewDomainError so callers
// can match them with errors.Is after wrapping with %w, and the HTTP layer
// can map them to status codes by Kind in exactly one file.
type DomainError struct {
	kind    string
	message string
}

// NewDomainError builds a sentinel value. kind is a stable machine-readable
// code; message is human-readable text for logs and responses.
func NewDomainError(kind, message string) DomainError {
	return DomainError{kind: kind, message: message}
}

func (e DomainError) Error() string { return e.message }

// Kind returns the stable machine-readable code for this error.
func (e DomainError) Kind() string { return e.kind }

// Is lets errors.Is match any DomainError carrying the same kind, even after
// wrapping with %w.
func (e DomainError) Is(target error) bool {
	t, ok := target.(DomainError)
	return ok && t.kind == e.kind
}

var (
	ErrInvalidEmail     = NewDomainError("email.invalid", "invalid email address")
	ErrInvalidTimeSlot  = NewDomainError("timeslot.invalid", "time slot end must be after start")
	ErrInvalidMoney     = NewDomainError("money.invalid", "invalid money amount")
	ErrQuantityTooSmall = NewDomainError("money.quantity", "quantity must be positive")
)

// Money is an amount in Brazilian reais expressed as integer cents.
// Floats are banned: every operation stays in int64.
type Money int64

// NewMoney builds an amount from its cent value. Pass integer cents, never a
// float literal.
func NewMoney(cents int64) Money { return Money(cents) }

// Add returns the sum of two amounts.
func (m Money) Add(o Money) Money { return m + o }

// Mul returns the amount multiplied by a whole quantity.
func (m Money) Mul(qty int) (Money, error) {
	if qty < 0 {
		return 0, ErrQuantityTooSmall
	}
	return m * Money(qty), nil
}

// Cents returns the underlying value in cents.
func (m Money) Cents() int64 { return int64(m) }

func (m Money) IsZero() bool     { return m == 0 }
func (m Money) IsNegative() bool { return m < 0 }

// String formats the amount as BRL currency text, e.g. "R$ 1.234,56".
func (m Money) String() string {
	v := int64(m)
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	intPart := v / 100
	decPart := v % 100
	digits := strconv.FormatInt(intPart, 10)
	var b strings.Builder
	b.WriteString("R$ ")
	b.WriteString(sign)
	for i := 0; i < len(digits); i++ {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteByte(digits[i])
	}
	b.WriteByte(',')
	if decPart < 10 {
		b.WriteByte('0')
	}
	b.WriteString(strconv.FormatInt(decPart, 10))
	return b.String()
}

// EmailAddress is a normalized, validated email address. It is always
// lowercase and trimmed.
type EmailAddress string

// ParseEmail validates raw and returns its normalized form.
func ParseEmail(raw string) (EmailAddress, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || len(s) > 254 {
		return "", ErrInvalidEmail
	}
	local, domain, ok := strings.Cut(s, "@")
	if !ok || local == "" || domain == "" {
		return "", ErrInvalidEmail
	}
	if strings.Contains(domain, "@") {
		return "", ErrInvalidEmail
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return "", ErrInvalidEmail
	}
	if strings.Contains(s, "..") {
		return "", ErrInvalidEmail
	}
	if local[0] == '.' || local[len(local)-1] == '.' {
		return "", ErrInvalidEmail
	}
	if !strings.Contains(domain, ".") {
		return "", ErrInvalidEmail
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrInvalidEmail
		}
	}
	return EmailAddress(s), nil
}

func (e EmailAddress) String() string { return string(e) }

// ID is an opaque public identifier. It is generated outside the domain via
// the IDGen port; the domain only carries and compares it.
type ID string

func (id ID) String() string { return string(id) }
func (id ID) IsZero() bool   { return id == "" }

// TimeSlot is a half-open [Start, End) interval in UTC.
type TimeSlot struct {
	Start time.Time
	End   time.Time
}

// NewTimeSlot validates that the slot is non-empty and well ordered.
func NewTimeSlot(start, end time.Time) (TimeSlot, error) {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return TimeSlot{}, ErrInvalidTimeSlot
	}
	return TimeSlot{Start: start, End: end}, nil
}

// Overlaps reports whether the two slots share any instant. Slots that merely
// touch at a boundary do not overlap.
func (s TimeSlot) Overlaps(o TimeSlot) bool {
	return s.Start.Before(o.End) && o.Start.Before(s.End)
}

// Contains reports whether t falls inside the slot.
func (s TimeSlot) Contains(t time.Time) bool {
	return !t.Before(s.Start) && t.Before(s.End)
}

// Duration returns the slot length.
func (s TimeSlot) Duration() time.Duration { return s.End.Sub(s.Start) }
