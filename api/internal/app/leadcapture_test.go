package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/app/ports"
)

type leadHarness struct {
	uc       *Leads
	leads    *fakes.LeadRepo
	mailer   *fakes.Mailer
	clock    *fakes.Clock
	tokengen *fakes.TokenGen
}

func newLeadHarness(t *testing.T) leadHarness {
	t.Helper()
	h := leadHarness{
		leads:    &fakes.LeadRepo{},
		mailer:   &fakes.Mailer{},
		clock:    &fakes.Clock{NowTime: time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)},
		tokengen: &fakes.TokenGen{},
	}
	h.uc = NewLeads(h.leads, h.mailer, h.clock, h.tokengen, "https://speako.com.br/confirmar?t=")
	return h
}

func TestCaptureLeadRecordsConsentAndMails(t *testing.T) {
	h := newLeadHarness(t)
	ctx := context.Background()

	if err := h.uc.CaptureLead(ctx, "Ana@Exemplo.com.br", "landing-gratis", "203.0.113.7"); err != nil {
		t.Fatalf("CaptureLead: %v", err)
	}
	if len(h.mailer.LeadOptInCalls) != 1 {
		t.Fatalf("sent %d opt-in mails, want 1", len(h.mailer.LeadOptInCalls))
	}

	stored, err := h.leads.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored %d leads, want 1", len(stored))
	}
	if stored[0].ConsentIP != "203.0.113.7" || !stored[0].ConsentAt.Equal(h.clock.NowTime) {
		t.Errorf("consent evidence not recorded: %+v", stored[0])
	}
	if stored[0].IsConfirmed() {
		t.Error("a captured lead must start unconfirmed")
	}
	// The raw token must never be what is stored.
	if stored[0].ConfirmTokenHash == "" {
		t.Error("no confirm token hash stored")
	}
}

func TestConfirmLeadIsIdempotent(t *testing.T) {
	h := newLeadHarness(t)
	ctx := context.Background()

	if err := h.uc.CaptureLead(ctx, "ana@exemplo.com.br", "landing-gratis", "203.0.113.7"); err != nil {
		t.Fatal(err)
	}
	link := h.mailer.LeadOptInCalls[0].ConfirmURL
	raw := link[len("https://speako.com.br/confirmar?t="):]

	if err := h.uc.ConfirmLead(ctx, raw); err != nil {
		t.Fatalf("first ConfirmLead: %v", err)
	}
	first, err := h.leads.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	confirmedAt := first[0].ConfirmedAt

	h.clock.NowTime = h.clock.NowTime.Add(2 * time.Hour)
	if err := h.uc.ConfirmLead(ctx, raw); err != nil {
		t.Fatalf("second ConfirmLead: %v", err)
	}
	again, err := h.leads.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !again[0].ConfirmedAt.Equal(confirmedAt) {
		t.Errorf("re-confirming moved ConfirmedAt to %v, want it pinned at %v", again[0].ConfirmedAt, confirmedAt)
	}
}

func TestConfirmLeadRejectsUnknownToken(t *testing.T) {
	h := newLeadHarness(t)
	if err := h.uc.ConfirmLead(context.Background(), "not-a-real-token"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("err = %v, want ports.ErrNotFound", err)
	}
}

// An unconfirmed address has not given usable consent under LGPD and must
// never reach a mailing list, however tempting the bigger number is.
func TestExportLeadsOnlyReturnsConfirmed(t *testing.T) {
	h := newLeadHarness(t)
	ctx := context.Background()

	for _, e := range []string{"ana@exemplo.com.br", "bruno@exemplo.com.br"} {
		if err := h.uc.CaptureLead(ctx, e, "landing-gratis", "203.0.113.7"); err != nil {
			t.Fatal(err)
		}
	}
	raw := h.mailer.LeadOptInCalls[0].ConfirmURL[len("https://speako.com.br/confirmar?t="):]
	if err := h.uc.ConfirmLead(ctx, raw); err != nil {
		t.Fatal(err)
	}

	got, err := h.uc.ExportLeads(ctx)
	if err != nil {
		t.Fatalf("ExportLeads: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("exported %d leads, want only the confirmed one", len(got))
	}
}
