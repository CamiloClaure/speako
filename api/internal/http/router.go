package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/camiloclaure/speako/api/internal/app"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// errUnauthenticated marks a request with no live session in context. It is
// transport-level (no domain error covers "no session was even presented")
// and maps to 401 unauthenticated in errors.go.
var errUnauthenticated = errors.New("authentication required")

// errNotImplemented marks an endpoint the contract declares but the app layer
// cannot serve yet.
var errNotImplemented = errors.New("not implemented")

const maxBodyBytes = 1 << 20

// Config is the transport configuration the router needs. Values that never
// change are constants, not config; everything here varies per environment.
type Config struct {
	PublicURL    string
	CookieSecure bool
	SessionTTL   time.Duration
}

// Router wires the use cases to HTTP routes.
type Router struct {
	auth      *app.Auth
	purchase  *app.Purchase
	booking   *app.Booking
	materials *app.Materials
	catalog   *app.Catalog
	leads     *app.Leads
	config    Config
}

// NewRouter builds the full HTTP stack: the route table wrapped in the
// middleware chain. Routes use Go 1.22 method patterns; no third-party router.
func NewRouter(
	auth *app.Auth,
	purchase *app.Purchase,
	booking *app.Booking,
	materials *app.Materials,
	catalog *app.Catalog,
	leads *app.Leads,
	config Config,
) *http.ServeMux {
	r := &Router{
		auth:      auth,
		purchase:  purchase,
		booking:   booking,
		materials: materials,
		catalog:   catalog,
		leads:     leads,
		config:    config,
	}
	routes := http.NewServeMux()
	r.register(routes)
	outer := http.NewServeMux()
	outer.Handle("/", r.chain(routes))
	return outer
}

func (r *Router) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", r.handleHealthz)

	mux.HandleFunc("GET /api/v1/products", r.handleListProducts)
	mux.HandleFunc("GET /api/v1/products/{slug}", r.handleGetProduct)
	mux.HandleFunc("GET /api/v1/slots", r.handleListSlots)

	mux.HandleFunc("POST /api/v1/leads", r.handleCaptureLead)
	mux.HandleFunc("GET /api/v1/leads/confirm", r.handleConfirmLead)

	mux.HandleFunc("POST /api/v1/auth/magic-link", r.handleRequestMagicLink)
	mux.HandleFunc("POST /api/v1/auth/magic-link/verify", r.handleVerifyMagicLink)
	mux.HandleFunc("POST /api/v1/auth/logout", r.handleLogout)

	mux.HandleFunc("GET /api/v1/me", r.requireUser(r.handleMe))
	mux.HandleFunc("GET /api/v1/me/credits", r.requireUser(r.handleCredits))
	mux.HandleFunc("GET /api/v1/me/orders", r.requireUser(r.handleOrders))
	mux.HandleFunc("GET /api/v1/me/materials", r.requireUser(r.handleMaterials))
	mux.HandleFunc("GET /api/v1/me/materials/{slug}/download", r.requireUser(r.handleMaterialDownload))
	mux.HandleFunc("GET /api/v1/me/bookings", r.requireUser(r.handleBookings))

	mux.HandleFunc("POST /api/v1/checkout", r.requireUser(r.handleCheckout))
	mux.HandleFunc("POST /api/v1/bookings", r.requireUser(r.handleBookLesson))
	mux.HandleFunc("DELETE /api/v1/bookings/{id}", r.requireUser(r.handleCancelBooking))

	mux.HandleFunc("POST /api/v1/webhooks/mercadopago", r.handleMercadoPagoWebhook)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(req *http.Request, dst any) error {
	if err := json.NewDecoder(io.LimitReader(req.Body, maxBodyBytes)).Decode(dst); err != nil {
		return validationErrorf("invalid request body")
	}
	return nil
}

func validationErrorf(format string, args ...any) error {
	return shared.NewDomainError("validation_failed", fmt.Sprintf(format, args...))
}
