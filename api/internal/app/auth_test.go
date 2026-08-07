package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

const (
	authLoginURLBase = "https://speako.app/auth/complete?token="
	authSessionTTL   = 30 * 24 * time.Hour
)

type authHarness struct {
	uc       *Auth
	users    *fakes.UserRepo
	sessions *fakes.SessionRepo
	tokens   *fakes.LoginTokenRepo
	tokengen *fakes.TokenGen
	hasher   *fakes.PasswordHasher
	mailer   *fakes.Mailer
	clock    *fakes.Clock
}

func newAuthHarness(t *testing.T) authHarness {
	t.Helper()
	h := authHarness{
		users:    &fakes.UserRepo{},
		sessions: &fakes.SessionRepo{},
		tokens:   &fakes.LoginTokenRepo{},
		tokengen: &fakes.TokenGen{},
		hasher:   &fakes.PasswordHasher{},
		mailer:   &fakes.Mailer{},
		clock:    &fakes.Clock{NowTime: time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)},
	}
	h.uc = NewAuth(h.users, h.sessions, h.tokens, h.mailer, h.clock, &fakes.IDGen{}, h.tokengen, h.hasher, authLoginURLBase, authSessionTTL)
	return h
}

// Sign-up IS the magic link: a first-time address must receive one, or the
// platform can never acquire a customer. Anti-enumeration comes from the
// response being identical, not from refusing to send.
func TestRequestMagicLinkUnknownEmailStillSendsSoSignUpWorks(t *testing.T) {
	h := newAuthHarness(t)
	if err := h.uc.RequestMagicLink(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("RequestMagicLink: %v", err)
	}
	if len(h.mailer.MagicLinkCalls) != 1 {
		t.Fatalf("sent %d mails to a new address, want 1 - nobody could sign up", len(h.mailer.MagicLinkCalls))
	}
	if err := h.uc.RequestMagicLink(context.Background(), "not-an-email"); err == nil {
		t.Fatal("RequestMagicLink must reject an invalid email")
	}
}

func TestMagicLinkStoresOnlyHashes(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.RequestMagicLink(ctx, "ana@test.com"); err != nil {
		t.Fatal(err)
	}
	if len(h.mailer.MagicLinkCalls) != 1 {
		t.Fatalf("got %d magic link mails, want 1", len(h.mailer.MagicLinkCalls))
	}
	call := h.mailer.MagicLinkCalls[0]
	if !strings.HasPrefix(call.URL, authLoginURLBase) {
		t.Fatalf("magic link URL %q does not carry the base prefix", call.URL)
	}
	raw := strings.TrimPrefix(call.URL, authLoginURLBase)

	// Consume looks the token up by hash(raw), so redeeming the link proves
	// the login token was stored under its hash and never as the raw value.
	if _, err := h.uc.CompleteMagicLink(ctx, raw); err != nil {
		t.Fatalf("redeeming the link should succeed when the token is hashed: %v", err)
	}

	sess, err := h.uc.SignInWithGoogle(ctx, "ana@test.com", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	// GetMe hashes the cookie value before lookup, so a session that resolves
	// must have been stored under its hash, never as the raw token.
	if _, err := h.uc.GetMe(ctx, sess.Token.String()); err != nil {
		t.Fatalf("session stored under its hash should resolve: %v", err)
	}
}

func TestMagicLinkIsSingleUse(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := h.uc.RequestMagicLink(ctx, "ana@test.com"); err != nil {
		t.Fatal(err)
	}
	raw := strings.TrimPrefix(h.mailer.MagicLinkCalls[0].URL, authLoginURLBase)
	if _, err := h.uc.CompleteMagicLink(ctx, raw); err != nil {
		t.Fatalf("first redemption should succeed: %v", err)
	}
	if _, err := h.uc.CompleteMagicLink(ctx, raw); !errors.Is(err, fakes.ErrNotFound) {
		t.Fatalf("second redemption err = %v, want ErrNotFound", err)
	}
}

func TestCompleteMagicLinkCreatesAccountForFirstTimeEmail(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	const raw = "welcome-link"
	if err := h.tokens.Create(ctx, ports.LoginToken{
		TokenHash: h.tokengen.Hash(raw),
		Email:     shared.EmailAddress("new@example.com"),
		ExpiresAt: h.clock.NowTime.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	sess, err := h.uc.CompleteMagicLink(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	user, err := h.users.GetByEmail(ctx, shared.EmailAddress("new@example.com"))
	if err != nil {
		t.Fatalf("account not created: %v", err)
	}
	if user.Role != identity.RoleStudent {
		t.Fatalf("role = %q, want student", user.Role)
	}
	if user.Timezone != "America/Sao_Paulo" {
		t.Fatalf("timezone = %q, want America/Sao_Paulo", user.Timezone)
	}
	if user.Name != "New" {
		t.Fatalf("name = %q, want a default derived from the email", user.Name)
	}
	if sess.UserID != user.ID {
		t.Fatalf("session user = %v, want %v", sess.UserID, user.ID)
	}
	if sess.Token == "" {
		t.Fatal("session carries no raw cookie token")
	}
	if _, err := h.uc.GetMe(ctx, sess.Token.String()); err != nil {
		t.Fatalf("fresh session should resolve: %v", err)
	}
}

func TestSignInWithGoogleCreatesAccountOnFirstSignIn(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	sess, err := h.uc.SignInWithGoogle(ctx, "google@example.com", "Bruna")
	if err != nil {
		t.Fatal(err)
	}
	user, err := h.users.GetByEmail(ctx, shared.EmailAddress("google@example.com"))
	if err != nil {
		t.Fatalf("account not created: %v", err)
	}
	if user.Name != "Bruna" || user.Role != identity.RoleStudent {
		t.Fatalf("user = %+v, want Bruna / student", user)
	}
	if sess.UserID != user.ID {
		t.Fatalf("session user = %v, want %v", sess.UserID, user.ID)
	}
}

func TestGetMeRejectsExpiredSession(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	sess, err := h.uc.SignInWithGoogle(ctx, "ana@test.com", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.uc.GetMe(ctx, sess.Token.String()); err != nil {
		t.Fatalf("live session should resolve: %v", err)
	}
	h.clock.NowTime = h.clock.NowTime.Add(authSessionTTL + time.Hour)
	if _, err := h.uc.GetMe(ctx, sess.Token.String()); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expired session err = %v, want ErrSessionExpired", err)
	}
}

func TestSignOutInvalidatesSession(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	sess, err := h.uc.SignInWithGoogle(ctx, "ana@test.com", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.uc.SignOut(ctx, sess.Token.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.uc.GetMe(ctx, sess.Token.String()); !errors.Is(err, fakes.ErrNotFound) {
		t.Fatalf("GetMe after SignOut err = %v, want ErrNotFound", err)
	}
}

// "Unknown address" and "the database is down" must not look the same. The
// first is a silent nil to avoid leaking which accounts exist; the second has
// to surface, or a student gets a success screen and no email, forever.
func TestSignInWithGoogleDoesNotCreateUserOnStorageFailure(t *testing.T) {
	h := newAuthHarness(t)
	h.users.Err = errors.New("database is down")

	if _, err := h.uc.SignInWithGoogle(context.Background(), "ana@test.com", "Ana"); err == nil {
		t.Fatal("a storage failure minted a session instead of erroring")
	}
	if len(h.sessions.Deleted) != 0 {
		t.Fatalf("unexpected session activity: %v", h.sessions.Deleted)
	}
}

func TestRegisterThenSignInWithPasswordSucceeds(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()

	regSess, err := h.uc.RegisterWithPassword(ctx, "ana@test.com", "Ana", "correct-horse")
	if err != nil {
		t.Fatalf("RegisterWithPassword: %v", err)
	}
	if _, err := h.uc.GetMe(ctx, regSess.Token.String()); err != nil {
		t.Fatalf("fresh session from register should resolve: %v", err)
	}

	signInSess, err := h.uc.SignInWithPassword(ctx, "ana@test.com", "correct-horse")
	if err != nil {
		t.Fatalf("SignInWithPassword with the right password: %v", err)
	}
	if _, err := h.uc.GetMe(ctx, signInSess.Token.String()); err != nil {
		t.Fatalf("session from sign-in should resolve: %v", err)
	}
}

func TestSignInWithPasswordWrongPasswordFails(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	if _, err := h.uc.RegisterWithPassword(ctx, "ana@test.com", "Ana", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.uc.SignInWithPassword(ctx, "ana@test.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password err = %v, want ErrInvalidCredentials", err)
	}
}

// The whole point of ErrInvalidCredentials is that a caller cannot tell "no
// such account" from "wrong password" apart - not by error identity, and not
// by the error text either.
func TestSignInWithPasswordUnknownEmailReturnsSameErrorAsWrongPassword(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	if _, err := h.uc.RegisterWithPassword(ctx, "ana@test.com", "Ana", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	_, unknownErr := h.uc.SignInWithPassword(ctx, "nobody@test.com", "whatever123")
	_, wrongErr := h.uc.SignInWithPassword(ctx, "ana@test.com", "wrong-password")
	if !errors.Is(unknownErr, ErrInvalidCredentials) {
		t.Fatalf("unknown email err = %v, want ErrInvalidCredentials", unknownErr)
	}
	if !errors.Is(wrongErr, ErrInvalidCredentials) {
		t.Fatalf("wrong password err = %v, want ErrInvalidCredentials", wrongErr)
	}
	if unknownErr.Error() != wrongErr.Error() {
		t.Fatalf("error text differs: unknown=%q wrong=%q - a caller could tell the two cases apart",
			unknownErr.Error(), wrongErr.Error())
	}
}

// A login attempt against an unknown email must still pay for one bcrypt
// comparison, or the missing comparison is itself a timing tell. This cannot
// assert on wall-clock time without being flaky, so it asserts on the thing
// that actually causes the constant cost: Matches was called.
func TestSignInWithPasswordPaysForAComparisonOnUnknownEmail(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	if _, err := h.uc.SignInWithPassword(ctx, "nobody@test.com", "whatever123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal(err)
	}
	if h.hasher.MatchesCalls != 1 {
		t.Fatalf("Matches called %d times for an unknown email, want 1 - an absent account must cost what a real one does",
			h.hasher.MatchesCalls)
	}
}

func TestRegisterWithPasswordRejectsShortPassword(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	if _, err := h.uc.RegisterWithPassword(ctx, "ana@test.com", "Ana", "short12"); !errors.Is(err, identity.ErrWeakPassword) {
		t.Fatalf("7-char password err = %v, want ErrWeakPassword", err)
	}
	if _, err := h.users.GetByEmail(ctx, shared.EmailAddress("ana@test.com")); !errors.Is(err, fakes.ErrNotFound) {
		t.Fatalf("a rejected registration must not create an account, GetByEmail err = %v", err)
	}
}

func TestRegisterWithPasswordRejectsExistingEmailWithoutOverwritingPassword(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	if _, err := h.uc.RegisterWithPassword(ctx, "ana@test.com", "Ana", "first-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.uc.RegisterWithPassword(ctx, "ana@test.com", "Someone Else", "second-password"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("re-registration err = %v, want ErrEmailTaken", err)
	}
	if _, err := h.uc.SignInWithPassword(ctx, "ana@test.com", "first-password"); err != nil {
		t.Fatalf("the original password should still sign in: %v", err)
	}
	if _, err := h.uc.SignInWithPassword(ctx, "ana@test.com", "second-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("the second password must not have been stored, err = %v", err)
	}
}

// An account created through the magic link (or Google) has no password at
// all - password_hash is "". SignInWithPassword must refuse every password
// for it, not just fail to find a match.
func TestSignInWithPasswordRejectsMagicLinkOnlyAccount(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	if _, err := h.uc.SignInWithGoogle(ctx, "ana@test.com", "Ana"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.uc.SignInWithPassword(ctx, "ana@test.com", "anything12"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("magic-link-only account err = %v, want ErrInvalidCredentials", err)
	}
}

// SetPassword is the CLI's entry point for giving a magic-link-only account
// a password (or resetting one). It goes through the same validation and
// storage as registration.
func TestSetPasswordThenSignIn(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	if _, err := h.uc.SignInWithGoogle(ctx, "ana@test.com", "Ana"); err != nil {
		t.Fatal(err)
	}
	user, err := h.users.GetByEmail(ctx, shared.EmailAddress("ana@test.com"))
	if err != nil {
		t.Fatal(err)
	}
	if err := h.uc.SetPassword(ctx, user.ID, "brand-new-password"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if _, err := h.uc.SignInWithPassword(ctx, "ana@test.com", "brand-new-password"); err != nil {
		t.Fatalf("SignInWithPassword after SetPassword: %v", err)
	}
	if err := h.uc.SetPassword(ctx, user.ID, "short"); !errors.Is(err, identity.ErrWeakPassword) {
		t.Fatalf("SetPassword with a short password err = %v, want ErrWeakPassword", err)
	}
}
