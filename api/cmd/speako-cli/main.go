// Command speako-cli is the teacher's back door for the rare operations that do
// not justify a screen: migrations, seeding, manual credit grants and export.
//
// ponytail: a flat switch over os.Args is enough for a handful of verbs; reach
// for a CLI library only if subcommand flags start nesting.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/camiloclaure/speako/api/internal/adapters/devstub"
	"github.com/camiloclaure/speako/api/internal/adapters/password"
	"github.com/camiloclaure/speako/api/internal/adapters/sqlite"
	"github.com/camiloclaure/speako/api/internal/adapters/system"
	"github.com/camiloclaure/speako/api/internal/app"
	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/config"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

const usage = `speako-cli <command>

  migrate                        apply pending database migrations
  seed                           create the teacher, availability and demo products
  grant-credit <email> <n>       add credits to a student's ledger
  export-leads                   print confirmed leads as CSV
  login-link <email>             print a one-time sign-in link for an account
  set-password <email> <pass>    set an account's password
`

const teacherID = shared.ID("teacher-1")

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := dispatch(context.Background(), os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, cmd string, args []string) error {
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

	switch cmd {
	case "migrate":
		fmt.Println("migrations applied to", cfg.DBPath)
		return nil
	case "seed":
		return seed(ctx, db, cfg)
	case "grant-credit":
		if len(args) < 2 {
			return fmt.Errorf("usage: speako-cli grant-credit <email> <n>")
		}
		return grantCredit(ctx, db, args[0], args[1])
	case "export-leads":
		return exportLeads(ctx, db)
	case "login-link":
		if len(args) < 1 {
			return fmt.Errorf("usage: speako-cli login-link <email>")
		}
		return loginLink(ctx, db, cfg, args[0])
	case "set-password":
		if len(args) < 2 {
			return fmt.Errorf("usage: speako-cli set-password <email> <password>")
		}
		return setPassword(ctx, db, cfg, args[0], args[1])
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
		return nil
	}
}

func seed(ctx context.Context, db *sql.DB, cfg config.Config) error {
	users := sqlite.NewUserRepo(db)
	products := sqlite.NewProductRepo(db)
	availability := sqlite.NewAvailabilityRepo(db)

	email := cfg.TeacherEmail
	if email == "" {
		email = "professora@speako.com.br"
	}
	teacherEmail, err := shared.ParseEmail(email)
	if err != nil {
		return err
	}
	teacher, err := identity.NewUser(teacherID, teacherEmail, "Professora", identity.RoleTeacher, cfg.TeacherTimezone)
	if err != nil {
		return err
	}
	if err := users.Create(ctx, teacher); err != nil {
		fmt.Println("teacher already present, continuing")
	}

	// Weekday mornings and afternoons, 60-minute lessons.
	var rules []scheduling.AvailabilityRule
	for _, wd := range []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday} {
		for _, window := range [][2]int{{9 * 60, 12 * 60}, {14 * 60, 18 * 60}} {
			r, err := scheduling.NewAvailabilityRule(wd, window[0], window[1], 60)
			if err != nil {
				return err
			}
			rules = append(rules, r)
		}
	}
	if err := availability.SaveRules(ctx, rules); err != nil {
		return err
	}

	demo := []struct {
		id, slug, title string
		cents           int64
		kind            catalog.Kind
		fileKey         string
		credits, days   int
	}{
		{"prod-guia", "guia-de-pronuncia", "Guia de Pronúncia", 4700, catalog.KindMaterial, "materials/guia-de-pronuncia.pdf", 0, 0},
		{"prod-verbos", "verbos-essenciais", "200 Verbos Essenciais", 3900, catalog.KindMaterial, "materials/verbos-essenciais.pdf", 0, 0},
		{"prod-p4", "pacote-4-aulas", "Pacote 4 aulas", 39900, catalog.KindPack, "", 4, 90},
		{"prod-p8", "pacote-8-aulas", "Pacote 8 aulas", 75900, catalog.KindPack, "", 8, 120},
		{"prod-p12", "pacote-12-aulas", "Pacote 12 aulas", 109900, catalog.KindPack, "", 12, 180},
	}
	for _, d := range demo {
		p, err := catalog.NewProduct(shared.ID(d.id), d.slug, d.title, shared.NewMoney(d.cents), d.kind, d.fileKey, d.credits, d.days)
		if err != nil {
			return err
		}
		if err := products.Create(ctx, p); err != nil {
			if err := products.Update(ctx, p); err != nil {
				return err
			}
		}
	}

	fmt.Printf("seeded %s: teacher, %d availability rules, %d products\n", cfg.DBPath, len(rules), len(demo))
	return nil
}

func grantCredit(ctx context.Context, db *sql.DB, rawEmail, rawN string) error {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return err
	}
	var n int
	if _, err := fmt.Sscanf(rawN, "%d", &n); err != nil || n <= 0 {
		return fmt.Errorf("credits must be a positive whole number, got %q", rawN)
	}
	users := sqlite.NewUserRepo(db)
	user, err := users.GetByEmail(ctx, email)
	if err != nil {
		return err
	}
	ledger := sqlite.NewCreditLedgerRepo(db)
	entry := credit.Entry{Delta: n, Reason: "manual_grant", RefID: "cli", ExpiresAt: time.Now().UTC().AddDate(0, 6, 0)}
	if err := ledger.Append(ctx, user.ID, entry); err != nil {
		return err
	}
	fmt.Printf("granted %d credits to %s\n", n, email)
	return nil
}

func exportLeads(ctx context.Context, db *sql.DB) error {
	all, err := sqlite.NewLeadRepo(db).List(ctx)
	if err != nil {
		return err
	}
	fmt.Println("email,source,consent_at,confirmed_at")
	for _, l := range all {
		if !l.IsConfirmed() {
			continue // unconfirmed consent must never reach a mailing list
		}
		fmt.Printf("%s,%s,%s,%s\n", l.Email, l.Source,
			l.ConsentAt.UTC().Format(time.RFC3339), l.ConfirmedAt.UTC().Format(time.RFC3339))
	}
	return nil
}

// stdoutMailer is the dev mailer with its magic-link method redirected to
// stdout: the operator running the CLI over SSH needs the link on the stream
// they can read, not on the stderr/slog stream the dev mailer logs to. Every
// other method is inherited unchanged - the CLI never sends those.
type stdoutMailer struct {
	devstub.Mailer
}

func (stdoutMailer) MagicLink(_ context.Context, _ shared.EmailAddress, link string) error {
	fmt.Println(link)
	return nil
}

// newAuth builds an app.Auth the same way cmd/api/main.go does, except the
// mailer prints to stdout instead of sending mail, and Google sign-in stays
// off - the CLI never needs to verify a Google token.
func newAuth(db *sql.DB, cfg config.Config) *app.Auth {
	return app.NewAuth(
		sqlite.NewUserRepo(db),
		sqlite.NewSessionRepo(db),
		sqlite.NewLoginTokenRepo(db),
		stdoutMailer{},
		nil,
		sqlite.NewTxManager(db),
		system.Clock{},
		system.IDGen{},
		system.TokenGen{},
		password.Bcrypt{},
		cfg.PublicURL+"/entrar?token=",
		cfg.SessionTTL,
	)
}

// lookupUser finds the account behind email, turning ports.ErrNotFound into a
// message that names the address - login-link and set-password must both
// refuse a vanished account before touching it, not after: see
// sqlite.UserRepo.SetPasswordHash, which cannot itself tell "updated" from
// "matched nothing".
func lookupUser(ctx context.Context, db *sql.DB, email shared.EmailAddress) (identity.User, error) {
	user, err := sqlite.NewUserRepo(db).GetByEmail(ctx, email)
	if errors.Is(err, ports.ErrNotFound) {
		return identity.User{}, fmt.Errorf("no account for %s", email)
	}
	return user, err
}

// loginLink prints a one-time sign-in URL for email to stdout. It refuses an
// unknown address up front rather than letting RequestMagicLink's "always
// succeeds" behaviour (by design - see its doc comment) mint a link for
// nobody.
func loginLink(ctx context.Context, db *sql.DB, cfg config.Config, rawEmail string) error {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return err
	}
	if _, err := lookupUser(ctx, db, email); err != nil {
		return err
	}
	return newAuth(db, cfg).RequestMagicLink(ctx, string(email))
}

// setPassword sets email's password, going through app.Auth.SetPassword so
// validation, hashing and storage stay in the one place that already does it
// for the HTTP registration path.
func setPassword(ctx context.Context, db *sql.DB, cfg config.Config, rawEmail, plain string) error {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return err
	}
	user, err := lookupUser(ctx, db, email)
	if err != nil {
		return err
	}
	if err := newAuth(db, cfg).SetPassword(ctx, user.ID, plain); err != nil {
		return err
	}
	fmt.Printf("password set for %s\n", email)
	return nil
}
