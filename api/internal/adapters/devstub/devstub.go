// Package devstub provides stand-ins for the external services so the API can
// run locally without a Mercado Pago account, an R2 bucket or a mail provider.
//
// These are development aids and nothing else. Wiring them in production would
// silently take payments that never happen and mail that never arrives, so the
// composition root refuses to use them outside development.
package devstub

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	_ ports.Mailer         = Mailer{}
	_ ports.PaymentGateway = PaymentGateway{}
	_ ports.FileStorage    = FileStorage{}
)

// Mailer logs the message that would have been sent. It logs the magic-link
// and opt-in URLs in full, which is exactly what makes local sign-in possible
// and exactly why it must never run in production.
type Mailer struct{}

func (Mailer) MagicLink(_ context.Context, to shared.EmailAddress, link string) error {
	slog.Warn("devstub mail: magic link", "to", to, "link", link)
	return nil
}

func (Mailer) PurchaseConfirmed(_ context.Context, to shared.EmailAddress, title string, amount shared.Money) error {
	slog.Info("devstub mail: purchase confirmed", "to", to, "product", title, "amount", amount.String())
	return nil
}

func (Mailer) BookingConfirmed(_ context.Context, to shared.EmailAddress, slot shared.TimeSlot) error {
	slog.Info("devstub mail: booking confirmed", "to", to, "starts_at", slot.Start)
	return nil
}

func (Mailer) BookingCancelled(_ context.Context, to shared.EmailAddress, slot shared.TimeSlot) error {
	slog.Info("devstub mail: booking cancelled", "to", to, "starts_at", slot.Start)
	return nil
}

func (Mailer) LessonReminder(_ context.Context, to shared.EmailAddress, slot shared.TimeSlot) error {
	slog.Info("devstub mail: lesson reminder", "to", to, "starts_at", slot.Start)
	return nil
}

func (Mailer) LeadOptIn(_ context.Context, to shared.EmailAddress, confirmURL string) error {
	slog.Warn("devstub mail: lead opt-in", "to", to, "link", confirmURL)
	return nil
}

// PaymentGateway pretends to open a checkout. The URL it returns points at a
// local page that posts the webhook back, so the whole purchase path can be
// exercised without money moving.
type PaymentGateway struct {
	APIURL string
}

func (g PaymentGateway) CreateCheckout(_ context.Context, order ordering.Order) (string, string, error) {
	ref := "devstub-" + string(order.ID)
	link := fmt.Sprintf("%s/dev/checkout?order=%s&amount_cents=%d",
		g.APIURL, url.QueryEscape(string(order.ID)), order.Amount.Cents())
	slog.Warn("devstub payment: checkout opened", "order", order.ID, "url", link)
	return link, ref, nil
}

func (PaymentGateway) FetchPayment(_ context.Context, ref string) (ports.PaymentStatus, error) {
	slog.Info("devstub payment: status queried", "ref", ref)
	return ports.PaymentPending, nil
}

// FileStorage returns a URL shaped like a signed one but backed by nothing.
type FileStorage struct {
	APIURL string
}

func (s FileStorage) SignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("%s/dev/files/%s?expires_in=%d", s.APIURL, url.PathEscape(key), int(ttl.Seconds())), nil
}
