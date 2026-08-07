package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app"
	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
	apihttp "github.com/camiloclaure/speako/api/internal/http"
	"github.com/camiloclaure/speako/api/internal/http/dto"
)

const (
	magicBase    = "https://app.speako.test/magic?token="
	studentEmail = "student@test.com"
)

type testRig struct {
	mux          *http.ServeMux
	mailer       *fakes.Mailer
	users        *fakes.UserRepo
	availability *fakes.AvailabilityRepo
}

func newTestRig(t *testing.T) *testRig {
	t.Helper()
	now := time.Date(2026, time.January, 5, 12, 0, 0, 0, time.UTC)
	clock := &fakes.Clock{NowTime: now}
	tokengen := &fakes.TokenGen{}
	idgen := &fakes.IDGen{}
	mailer := &fakes.Mailer{}
	tx := &fakes.TxManager{}
	availability := &fakes.AvailabilityRepo{}
	users := &fakes.UserRepo{}

	auth := app.NewAuth(
		users, &fakes.SessionRepo{}, &fakes.LoginTokenRepo{},
		mailer, clock, idgen, tokengen, &fakes.PasswordHasher{}, magicBase, 30*24*time.Hour,
	)
	catalog := app.NewCatalog(&fakes.ProductRepo{})
	booking := app.NewBooking(&fakes.CreditLedgerRepo{}, &fakes.BookingRepo{}, availability, tx, clock, idgen, app.DefaultBookingPolicy())
	purchase := app.NewPurchase(
		&fakes.ProductRepo{}, &fakes.OrderRepo{}, &fakes.UserRepo{},
		&fakes.EntitlementRepo{}, &fakes.CreditLedgerRepo{}, &fakes.PaymentEventRepo{},
		&fakes.PaymentGateway{}, mailer, tx, clock, idgen,
	)
	materials := app.NewMaterials(&fakes.ProductRepo{}, &fakes.EntitlementRepo{}, &fakes.FileStorage{})
	leads := app.NewLeads(&fakes.LeadRepo{}, mailer, clock, tokengen, "https://app.speako.test/leads/confirm?token=")
	teacher := app.NewTeacher(&fakes.ProductRepo{}, &fakes.OrderRepo{}, &fakes.BookingRepo{},
		availability, &fakes.CreditLedgerRepo{}, users, tx, clock, idgen)

	config := apihttp.Config{
		PublicURL:    "https://app.speako.test",
		CookieSecure: false,
		SessionTTL:   30 * 24 * time.Hour,
	}
	mux := apihttp.NewRouter(auth, purchase, booking, materials, catalog, leads, teacher, config)
	return &testRig{mux: mux, mailer: mailer, users: users, availability: availability}
}

// signIn drives the full magic-link flow through the router and returns the
// session cookie for a fresh account.
func signIn(t *testing.T, rig *testRig) *http.Cookie {
	t.Helper()
	user, err := identity.NewUser(shared.ID("00000000-0000-4000-8000-000000000001"),
		shared.EmailAddress(studentEmail), "Student", identity.RoleStudent, "America/Sao_Paulo")
	if err != nil {
		t.Fatalf("build student: %v", err)
	}
	if err := rig.users.Create(context.Background(), user); err != nil {
		t.Fatalf("seed student: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link",
		strings.NewReader(`{"email":"`+studentEmail+`"}`))
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("magic-link request: status %d, want %d; body=%s", rr.Code, http.StatusAccepted, rr.Body.String())
	}
	if len(rig.mailer.MagicLinkCalls) == 0 {
		t.Fatal("magic-link request sent no email")
	}
	raw := strings.TrimPrefix(rig.mailer.MagicLinkCalls[0].URL, magicBase)
	if raw == rig.mailer.MagicLinkCalls[0].URL {
		t.Fatalf("magic link URL %q does not start with base %q", rig.mailer.MagicLinkCalls[0].URL, magicBase)
	}
	vreq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/magic-link/verify",
		strings.NewReader(`{"token":"`+raw+`"}`))
	vrr := httptest.NewRecorder()
	rig.mux.ServeHTTP(vrr, vreq)
	if vrr.Code != http.StatusOK {
		t.Fatalf("magic-link verify: status %d, want %d; body=%s", vrr.Code, http.StatusOK, vrr.Body.String())
	}
	for _, c := range vrr.Result().Cookies() {
		if c.Name == "speako_session" {
			return c
		}
	}
	t.Fatal("verify response carried no session cookie")
	return nil
}

func TestMeRequiresAuthentication(t *testing.T) {
	rig := newTestRig(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
	var envelope dto.ErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if envelope.Error.Code != "unauthenticated" {
		t.Errorf("code = %q, want %q", envelope.Error.Code, "unauthenticated")
	}
}

func TestErrorBodyShapeIsExactlyTheEnvelope(t *testing.T) {
	rig := newTestRig(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)

	const want = `{"error":{"code":"unauthenticated","message":"authentication required"}}`
	if got := strings.TrimSpace(rr.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestBookingWithoutCreditsReturnsInsufficientCredits(t *testing.T) {
	rig := newTestRig(t)
	// 13:00-15:00 in the teacher's timezone (São Paulo, UTC-3) means the first
	// slot really starts at 16:00 UTC. Booking 14:00Z would not be a slot.
	rule, err := scheduling.NewAvailabilityRule(time.Wednesday, 13*60, 15*60, 60)
	if err != nil {
		t.Fatalf("build availability rule: %v", err)
	}
	if err := rig.availability.SaveRules(context.Background(), []scheduling.AvailabilityRule{rule}); err != nil {
		t.Fatalf("seed availability: %v", err)
	}

	cookie := signIn(t, rig)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bookings",
		strings.NewReader(`{"starts_at":"2026-01-07T16:00:00Z"}`))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	var envelope dto.ErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if envelope.Error.Code != "insufficient_credits" {
		t.Errorf("code = %q, want %q", envelope.Error.Code, "insufficient_credits")
	}
}

func TestCheckoutUnknownProductReturnsNotFound(t *testing.T) {
	rig := newTestRig(t)
	cookie := signIn(t, rig)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout",
		strings.NewReader(`{"product_slug":"no-such-product","quantity":1}`))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	var envelope dto.ErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if envelope.Error.Code != "not_found" {
		t.Errorf("code = %q, want %q", envelope.Error.Code, "not_found")
	}
}

// A signed-in student reaching for the admin surface must be refused by the
// API, not merely by the SPA's router. The client-side guard is convenience.
func TestAdminRoutesRejectAStudent(t *testing.T) {
	rig := newTestRig(t)
	cookie := signIn(t, rig)

	for _, path := range []string{
		"/api/v1/admin/availability",
		"/api/v1/admin/bookings",
		"/api/v1/admin/orders",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		rig.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403; body=%s", path, rr.Code, rr.Body.String())
		}
		var envelope dto.ErrorEnvelope
		if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err == nil && envelope.Error.Code != "forbidden" {
			t.Errorf("%s: code = %q, want forbidden", path, envelope.Error.Code)
		}
	}
}

// And an anonymous caller gets 401, not 403 - the two are different answers.
func TestAdminRoutesRejectAnonymous(t *testing.T) {
	rig := newTestRig(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}
