// Command api is the composition root: it reads configuration, constructs the
// concrete adapters, injects them into the use cases and serves HTTP.
//
// This is the ONLY place allowed to name concrete adapter types. Everything
// below it depends on interfaces.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/camiloclaure/speako/api/internal/adapters/devstub"
	"github.com/camiloclaure/speako/api/internal/adapters/sqlite"
	"github.com/camiloclaure/speako/api/internal/adapters/system"
	"github.com/camiloclaure/speako/api/internal/app"
	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/config"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
	speakohttp "github.com/camiloclaure/speako/api/internal/http"
)

// teacherID is the single teacher this deployment serves. A second teacher is
// new rows, not a schema change.
const teacherID = shared.ID("teacher-1")

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(context.Background()); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := sqlite.Migrate(db); err != nil {
		return err
	}

	// Driven adapters.
	users := sqlite.NewUserRepo(db)
	sessions := sqlite.NewSessionRepo(db)
	loginTokens := sqlite.NewLoginTokenRepo(db)
	products := sqlite.NewProductRepo(db)
	orders := sqlite.NewOrderRepo(db)
	events := sqlite.NewPaymentEventRepo(db)
	entitlements := sqlite.NewEntitlementRepo(db)
	ledger := sqlite.NewCreditLedgerRepo(db)
	bookings := sqlite.NewBookingRepo(db, teacherID)
	availability := sqlite.NewAvailabilityRepo(db)
	leadRepo := sqlite.NewLeadRepo(db)
	tx := sqlite.NewTxManager(db)

	clock := system.Clock{}
	idgen := system.IDGen{}
	tokengen := system.TokenGen{}

	mailer, payments, files, err := externals(cfg)
	if err != nil {
		return err
	}

	// Use cases.
	auth := app.NewAuth(users, sessions, loginTokens, mailer, clock, idgen, tokengen,
		cfg.PublicURL+"/entrar?token=", cfg.SessionTTL)
	purchase := app.NewPurchase(products, orders, users, entitlements, ledger, events,
		payments, mailer, tx, clock, idgen)
	booking := app.NewBooking(ledger, bookings, availability, tx, clock, idgen, app.BookingPolicy{
		MinNotice:  cfg.MinNotice,
		FreeCancel: cfg.FreeCancel,
		MeetingURL: cfg.MeetingURL,
		Timezone:   cfg.TeacherTimezone,
	})
	materials := app.NewMaterials(products, entitlements, files)
	catalog := app.NewCatalog(products)
	leads := app.NewLeads(leadRepo, mailer, clock, tokengen, cfg.PublicURL+"/confirmar?token=")
	teacher := app.NewTeacher(products, orders, bookings, availability, ledger, users, tx, clock, idgen)

	handler := speakohttp.NewRouter(auth, purchase, booking, materials, catalog, leads, teacher, speakohttp.Config{
		PublicURL:    cfg.PublicURL,
		CookieSecure: cfg.CookieSecure,
		SessionTTL:   cfg.SessionTTL,
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "env", cfg.Env, "public_url", cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// externals picks the real integrations or the development stand-ins. Refusing
// the stubs outside development is deliberate: a production process that
// "takes payment" through a stub would confirm orders nobody paid for.
func externals(cfg config.Config) (ports.Mailer, ports.PaymentGateway, ports.FileStorage, error) {
	if cfg.IsProduction() {
		return nil, nil, nil, errors.New(
			"production integrations are not implemented yet: Mercado Pago (S6), R2 and Resend (S8). " +
				"Refusing to start rather than serve customers with development stubs")
	}
	slog.Warn("running with development stubs: no real payments, files or email will be processed")
	return devstub.Mailer{},
		devstub.PaymentGateway{APIURL: cfg.APIURL},
		devstub.FileStorage{APIURL: cfg.APIURL},
		nil
}
