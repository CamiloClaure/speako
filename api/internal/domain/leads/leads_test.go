package leads

import (
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

func testEmail(t *testing.T) shared.EmailAddress {
	t.Helper()
	e, err := shared.ParseEmail("Aluna@Exemplo.com.br")
	if err != nil {
		t.Fatalf("ParseEmail: %v", err)
	}
	return e
}

func TestCaptureRecordsConsentEvidence(t *testing.T) {
	at := time.Date(2026, time.March, 2, 15, 4, 5, 0, time.UTC)

	lead, err := Capture(testEmail(t), "landing-gratis", at, "203.0.113.7", "hash")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !lead.ConsentAt.Equal(at) {
		t.Errorf("ConsentAt = %v, want %v", lead.ConsentAt, at)
	}
	if lead.ConsentIP != "203.0.113.7" {
		t.Errorf("ConsentIP = %q, want %q", lead.ConsentIP, "203.0.113.7")
	}
	if lead.Source != "landing-gratis" {
		t.Errorf("Source = %q, want %q", lead.Source, "landing-gratis")
	}
	if lead.IsConfirmed() {
		t.Error("a freshly captured lead must not be confirmed - double opt-in has not happened")
	}
}

func TestCaptureRejectsMissingConsentEvidence(t *testing.T) {
	at := time.Date(2026, time.March, 2, 15, 4, 5, 0, time.UTC)

	tests := []struct {
		name      string
		consentAt time.Time
		consentIP string
		want      error
	}{
		{"empty ip", at, "", ErrMissingConsentIP},
		{"blank ip", at, "   ", ErrMissingConsentIP},
		{"zero timestamp", time.Time{}, "203.0.113.7", ErrMissingConsentTime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lead, err := Capture(testEmail(t), "landing-gratis", tt.consentAt, tt.consentIP, "hash")
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if lead != (Lead{}) {
				t.Errorf("a rejected capture must return the zero Lead, got %+v", lead)
			}
		})
	}
}

func TestConfirmIsIdempotent(t *testing.T) {
	at := time.Date(2026, time.March, 2, 15, 4, 5, 0, time.UTC)
	lead, err := Capture(testEmail(t), "landing-gratis", at, "203.0.113.7", "hash")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	first := at.Add(10 * time.Minute)
	confirmed := lead.Confirm(first)
	if !confirmed.IsConfirmed() {
		t.Fatal("lead should be confirmed after Confirm")
	}
	if !confirmed.ConfirmedAt.Equal(first) {
		t.Fatalf("ConfirmedAt = %v, want %v", confirmed.ConfirmedAt, first)
	}

	// A second confirmation - a re-clicked email link - keeps the original time.
	again := confirmed.Confirm(first.Add(2 * time.Hour))
	if !again.ConfirmedAt.Equal(first) {
		t.Errorf("re-confirming moved ConfirmedAt to %v, want it pinned at %v", again.ConfirmedAt, first)
	}

	// Confirm must not mutate the receiver.
	if lead.IsConfirmed() {
		t.Error("Confirm mutated the original lead")
	}
}
