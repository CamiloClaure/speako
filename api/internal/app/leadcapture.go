package app

import (
	"context"
	"errors"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/leads"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// Leads captures prospects from the free-material page and confirms their
// double opt-in.
type Leads struct {
	leads      ports.LeadRepo
	mailer     ports.Mailer
	clock      ports.Clock
	tokengen   ports.TokenGen
	confirmURL string
}

// NewLeads wires lead capture to its ports.
func NewLeads(leadRepo ports.LeadRepo, mailer ports.Mailer, clock ports.Clock, tokengen ports.TokenGen, confirmURL string) *Leads {
	return &Leads{leads: leadRepo, mailer: mailer, clock: clock, tokengen: tokengen, confirmURL: confirmURL}
}

// CaptureLead records consent and sends the opt-in email. consentIP is
// required: under LGPD the consent event is only evidence if it carries when
// and from where it was given.
//
// Re-submitting an address that is already captured re-sends the opt-in mail
// rather than erroring, because from the visitor's side the first mail simply
// never arrived.
func (uc *Leads) CaptureLead(ctx context.Context, rawEmail, source, consentIP string) error {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return err
	}
	existing, err := uc.leads.GetByEmail(ctx, email)
	isNew := errors.Is(err, ports.ErrNotFound)
	switch {
	case err == nil && existing.IsConfirmed():
		return nil // already on the list; nothing to confirm
	case err != nil && !isNew:
		return err
	}

	raw, hash, err := uc.tokengen.NewToken()
	if err != nil {
		return err
	}
	lead, err := leads.Capture(email, source, uc.clock.Now(), consentIP, hash)
	if err != nil {
		return err
	}
	if isNew {
		err = uc.leads.Create(ctx, lead)
	} else {
		err = uc.leads.Save(ctx, lead)
	}
	if err != nil {
		return err
	}
	return uc.mailer.LeadOptIn(ctx, email, uc.confirmURL+raw)
}

// ConfirmLead completes the double opt-in. It is idempotent: clicking the
// link twice keeps the first confirmation time.
func (uc *Leads) ConfirmLead(ctx context.Context, rawToken string) error {
	lead, err := uc.leads.GetByConfirmTokenHash(ctx, uc.tokengen.Hash(rawToken))
	if err != nil {
		return err
	}
	if lead.IsConfirmed() {
		return nil
	}
	return uc.leads.Save(ctx, lead.Confirm(uc.clock.Now()))
}

// ExportLeads returns confirmed leads only. An unconfirmed address has not
// given usable consent and must never reach a mailing list.
func (uc *Leads) ExportLeads(ctx context.Context) ([]leads.Lead, error) {
	all, err := uc.leads.List(ctx)
	if err != nil {
		return nil, err
	}
	confirmed := make([]leads.Lead, 0, len(all))
	for _, l := range all {
		if l.IsConfirmed() {
			confirmed = append(confirmed, l)
		}
	}
	return confirmed, nil
}
