package leads

import (
	"strings"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	// ErrMissingConsentIP is returned when capture records no consent IP.
	// LGPD requires the IP to be recorded alongside the consent timestamp.
	ErrMissingConsentIP = shared.NewDomainError("lead.consent_ip", "consent IP is required")
	// ErrMissingConsentTime is returned when capture records no consent time.
	ErrMissingConsentTime = shared.NewDomainError("lead.consent_time", "consent timestamp is required")
)

// Lead is a captured prospect awaiting double opt-in confirmation.
// ConfirmTokenHash is the hash of the token in the opt-in email. It lives on
// the lead rather than in the login-token store on purpose: a marketing
// confirmation must never be redeemable as a session.
type Lead struct {
	Email            shared.EmailAddress
	Source           string
	ConsentAt        time.Time
	ConsentIP        string
	ConfirmedAt      time.Time
	ConfirmTokenHash string
}

// Capture records a lead with LGPD consent. The consent IP must be non-empty;
// both it and the consent timestamp are evidence of a valid consent event.
func Capture(email shared.EmailAddress, source string, consentAt time.Time, consentIP, confirmTokenHash string) (Lead, error) {
	if strings.TrimSpace(consentIP) == "" {
		return Lead{}, ErrMissingConsentIP
	}
	if consentAt.IsZero() {
		return Lead{}, ErrMissingConsentTime
	}
	return Lead{
		Email:            email,
		Source:           source,
		ConsentAt:        consentAt,
		ConsentIP:        consentIP,
		ConfirmTokenHash: confirmTokenHash,
	}, nil
}

// Confirm completes the double opt-in at now. It is idempotent: the first
// confirmation time is kept.
func (l Lead) Confirm(now time.Time) Lead {
	if l.ConfirmedAt.IsZero() {
		l.ConfirmedAt = now
	}
	return l
}

// IsConfirmed reports whether double opt-in completed.
func (l Lead) IsConfirmed() bool {
	return !l.ConfirmedAt.IsZero()
}
