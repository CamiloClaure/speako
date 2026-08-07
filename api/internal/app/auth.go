package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// ErrSessionExpired reports a session whose lifetime has ended; the caller
// must ask the student to sign in again.
var ErrSessionExpired = errors.New("session expired")

// ErrGoogleNotConfigured is what SignInWithGoogle returns when no
// GoogleVerifier was wired in - GOOGLE_CLIENT_ID unset. Distinguishing "this
// feature is off" from "that token was forged" keeps an operator's
// configuration gap from being reported as an attack.
var ErrGoogleNotConfigured = errors.New("google sign-in is not configured")

var (
	// ErrInvalidCredentials is what SignInWithPassword returns for every way
	// a password sign-in can fail: unknown email, an account with no
	// password set, or a wrong password. Returning anything else for any one
	// of those three would tell a caller which case it hit - see
	// SignInWithPassword's doc comment.
	ErrInvalidCredentials = shared.NewDomainError("auth.invalid_credentials", "invalid credentials")
	// ErrEmailTaken is returned by RegisterWithPassword when the email
	// already has an account.
	ErrEmailTaken = shared.NewDomainError("auth.email_taken", "email already registered")
)

// magicLinkTTL is how long a magic link stays redeemable.
const magicLinkTTL = 15 * time.Minute

// studentTimezone is the default timezone for new accounts; the platform is
// Brazilian-run and a learner can change it later.
const studentTimezone = "America/Sao_Paulo"

// Auth owns sign-in and sign-out. Sessions and magic links store only the
// hash of a secret, so a leaked database cannot be replayed as a login.
type Auth struct {
	users        ports.UserRepo
	sessions     ports.SessionRepo
	tokens       ports.LoginTokenRepo
	mailer       ports.Mailer
	google       ports.GoogleVerifier
	tx           ports.TxManager
	clock        ports.Clock
	idgen        ports.IDGen
	tokengen     ports.TokenGen
	hasher       ports.PasswordHasher
	loginURLBase string
	sessionTTL   time.Duration
}

// dummyPasswordHash is a bcrypt hash of an arbitrary, never-issued password.
// SignInWithPassword compares against it whenever there is no real hash to
// compare against (unknown email, or an account with no password set), so
// that case costs the same wall-clock time as a wrong-password attempt on a
// real account. Without it, timing alone would tell an attacker which emails
// have accounts. It is computed lazily from whichever Auth calls it first:
// every PasswordHasher implementation hashes the same fixed input to the
// same output, so it does not matter which instance wins the race, and it
// only has to happen once per process.
var (
	dummyPasswordHash     string
	dummyPasswordHashOnce sync.Once
)

const dummyPassword = "speako-dummy-password-for-timing-safety"

// getDummyHash returns dummyPasswordHash, computing it on the first call.
func (uc *Auth) getDummyHash() string {
	dummyPasswordHashOnce.Do(func() {
		// Hash only fails if the system entropy source fails; falling back to
		// "" still makes Matches do a real (failing) comparison rather than
		// skip it, so the timing guarantee holds either way.
		dummyPasswordHash, _ = uc.hasher.Hash(dummyPassword)
	})
	return dummyPasswordHash
}

// NewAuth wires the authentication use cases to their ports. google may be
// nil - a caller that has not configured GOOGLE_CLIENT_ID passes nil, and
// SignInWithGoogle reports ErrGoogleNotConfigured rather than crashing. Pass
// a concrete *google.Verifier only from inside the branch that checked the
// client ID is non-empty: assigning a typed nil pointer to this parameter
// produces a non-nil interface, and the nil check below would never fire.
func NewAuth(
	users ports.UserRepo,
	sessions ports.SessionRepo,
	tokens ports.LoginTokenRepo,
	mailer ports.Mailer,
	google ports.GoogleVerifier,
	tx ports.TxManager,
	clock ports.Clock,
	idgen ports.IDGen,
	tokengen ports.TokenGen,
	hasher ports.PasswordHasher,
	loginURLBase string,
	sessionTTL time.Duration,
) *Auth {
	return &Auth{
		users:        users,
		sessions:     sessions,
		tokens:       tokens,
		mailer:       mailer,
		google:       google,
		tx:           tx,
		clock:        clock,
		idgen:        idgen,
		tokengen:     tokengen,
		hasher:       hasher,
		loginURLBase: loginURLBase,
		sessionTTL:   sessionTTL,
	}
}

// RequestMagicLink emails a single-use login link. It deliberately does not
// care whether the address already has an account: redeeming the link creates
// one, which is how sign-up works on a passwordless platform.
//
// Anti-enumeration comes from the response being identical either way, not
// from refusing to send - only the real inbox owner ever sees the token.
// Rate limiting, not silence, is what keeps this from becoming a mail cannon.
func (uc *Auth) RequestMagicLink(ctx context.Context, rawEmail string) error {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return err
	}
	raw, hash, err := uc.tokengen.NewToken()
	if err != nil {
		return err
	}
	if err := uc.tokens.Create(ctx, ports.LoginToken{
		TokenHash: hash,
		Email:     email,
		ExpiresAt: uc.clock.Now().Add(magicLinkTTL),
	}); err != nil {
		return err
	}
	return uc.mailer.MagicLink(ctx, email, uc.loginURLBase+raw)
}

// CompleteMagicLink redeems a magic-link token. A token is single use, so a
// forwarded or replayed link signs nobody in twice. A first-time address gets
// an account created as a student. The returned session carries the raw
// token so the caller can set the session cookie.
func (uc *Auth) CompleteMagicLink(ctx context.Context, rawToken string) (identity.Session, error) {
	token, err := uc.tokens.Consume(ctx, uc.tokengen.Hash(rawToken), uc.clock.Now())
	if err != nil {
		return identity.Session{}, err
	}
	user, err := uc.users.GetByEmail(ctx, token.Email)
	if errors.Is(err, ports.ErrNotFound) {
		// The token is legitimate but the account is not there yet; create it.
		user, err = identity.NewUser(uc.idgen.NewID(), token.Email, defaultNameFromEmail(token.Email), identity.RoleStudent, studentTimezone)
		if err != nil {
			return identity.Session{}, err
		}
		if err := uc.users.Create(ctx, user); err != nil {
			return identity.Session{}, err
		}
	} else if err != nil {
		return identity.Session{}, err
	}
	return uc.mintSession(ctx, user)
}

// SignInWithGoogle verifies idToken against Google, then finds or creates the
// account behind the verified identity and mints a fresh session for it.
//
// The token is the only input - not a caller-supplied email - on purpose:
// "this email was verified" is a convention a plain string cannot carry, so
// any caller that took a raw email instead could be tricked into signing
// someone in as an address they never proved they own.
func (uc *Auth) SignInWithGoogle(ctx context.Context, idToken string) (identity.Session, error) {
	if uc.google == nil {
		return identity.Session{}, ErrGoogleNotConfigured
	}
	verified, err := uc.google.Verify(ctx, idToken)
	if err != nil {
		// Passed through unchanged: a JWKS outage and a forged token must not
		// look alike, or an outage would be reported as an attack.
		return identity.Session{}, err
	}
	user, err := uc.users.GetByEmail(ctx, verified.Email)
	if errors.Is(err, ports.ErrNotFound) {
		user, err = identity.NewUser(uc.idgen.NewID(), verified.Email, verified.Name, identity.RoleStudent, studentTimezone)
		if err != nil {
			return identity.Session{}, err
		}
		if err := uc.users.Create(ctx, user); err != nil {
			return identity.Session{}, err
		}
	} else if err != nil {
		return identity.Session{}, err
	}
	return uc.mintSession(ctx, user)
}

// RegisterWithPassword creates a student account with a password and signs
// it in. The email must not already have an account - unlike the magic link
// and Google flows, which treat "first time we see this address" as sign-up,
// a password account is created explicitly, once, on purpose.
//
// Create and SetPasswordHash run inside one transaction: without it, a
// SetPasswordHash failure after a successful Create leaves an account that
// exists but has no usable password, and a retry only ever sees ErrEmailTaken
// - there would be no way back in. mintSession stays outside the transaction:
// a session row is not part of the account's integrity, and holding a write
// transaction open across it buys nothing.
func (uc *Auth) RegisterWithPassword(ctx context.Context, rawEmail, name, password string) (identity.Session, error) {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return identity.Session{}, err
	}
	if err := identity.ValidatePassword(password); err != nil {
		return identity.Session{}, err
	}
	if _, err := uc.users.GetByEmail(ctx, email); err == nil {
		return identity.Session{}, ErrEmailTaken
	} else if !errors.Is(err, ports.ErrNotFound) {
		return identity.Session{}, err
	}
	user, err := identity.NewUser(uc.idgen.NewID(), email, name, identity.RoleStudent, studentTimezone)
	if err != nil {
		return identity.Session{}, err
	}
	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return identity.Session{}, err
	}
	if err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		if err := uc.users.Create(ctx, user); err != nil {
			return err
		}
		return uc.users.SetPasswordHash(ctx, user.ID, hash)
	}); err != nil {
		return identity.Session{}, err
	}
	return uc.mintSession(ctx, user)
}

// SignInWithPassword checks an email and password and mints a session.
//
// It returns the exact same ErrInvalidCredentials for three different
// situations: the email has no account, the account has no password set (it
// is magic-link-only), and the account has a password but it does not match.
// Any other response shape - a different error, a different status code, a
// body that differs by even one byte - would let a caller distinguish which
// case occurred, which means it could enumerate which emails have accounts,
// or that a specific account is magic-link-only.
//
// It also resists a timing attack: when there is no real hash to compare
// against, it still runs one Matches call against a fixed dummy hash
// (getDummyHash), so that branch costs the same wall-clock time as comparing
// against a real, wrong password.
func (uc *Auth) SignInWithPassword(ctx context.Context, rawEmail, password string) (identity.Session, error) {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return identity.Session{}, err
	}
	user, err := uc.users.GetByEmail(ctx, email)
	if errors.Is(err, ports.ErrNotFound) {
		uc.hasher.Matches(uc.getDummyHash(), password)
		return identity.Session{}, ErrInvalidCredentials
	}
	if err != nil {
		// A real storage failure, not "no such account" - it must surface,
		// never collapse into ErrInvalidCredentials.
		return identity.Session{}, err
	}
	hash, err := uc.users.GetPasswordHash(ctx, user.ID)
	if err != nil {
		return identity.Session{}, err
	}
	if hash == "" {
		// Magic-link-only account: still pay for a comparison so this case
		// costs the same as a wrong password on a real hash.
		hash = uc.getDummyHash()
	}
	if !uc.hasher.Matches(hash, password) {
		return identity.Session{}, ErrInvalidCredentials
	}
	return uc.mintSession(ctx, user)
}

// SetPassword validates and stores a new password hash for userID, without
// touching the account's other fields. Used by the CLI to set (or reset) a
// student's password.
func (uc *Auth) SetPassword(ctx context.Context, userID shared.ID, password string) error {
	if err := identity.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return err
	}
	return uc.users.SetPasswordHash(ctx, userID, hash)
}

// SignOut deletes the session behind the cookie value.
func (uc *Auth) SignOut(ctx context.Context, rawSessionToken string) error {
	return uc.sessions.Delete(ctx, shared.ID(uc.tokengen.Hash(rawSessionToken)))
}

// GetMe returns the user behind a live session, rejecting expired ones.
func (uc *Auth) GetMe(ctx context.Context, rawSessionToken string) (identity.User, error) {
	session, err := uc.sessions.GetByTokenHash(ctx, shared.ID(uc.tokengen.Hash(rawSessionToken)))
	if err != nil {
		return identity.User{}, err
	}
	if !session.IsValid(uc.clock.Now()) {
		return identity.User{}, ErrSessionExpired
	}
	return uc.users.GetByID(ctx, session.UserID)
}

// mintSession stores a session under its token hash and hands back the raw
// token for the cookie. The raw value is never persisted.
func (uc *Auth) mintSession(ctx context.Context, user identity.User) (identity.Session, error) {
	raw, hash, err := uc.tokengen.NewToken()
	if err != nil {
		return identity.Session{}, err
	}
	session := identity.Session{
		Token:     shared.ID(hash),
		UserID:    user.ID,
		ExpiresAt: uc.clock.Now().Add(uc.sessionTTL),
	}
	if err := uc.sessions.Create(ctx, session); err != nil {
		return identity.Session{}, err
	}
	session.Token = shared.ID(raw)
	return session, nil
}

// defaultNameFromEmail guesses a first name from the email's local part, for
// accounts created through a magic link, which carries no name.
func defaultNameFromEmail(email shared.EmailAddress) string {
	local, _, _ := strings.Cut(string(email), "@")
	if local == "" {
		return "Student"
	}
	return strings.ToUpper(local[:1]) + local[1:]
}
