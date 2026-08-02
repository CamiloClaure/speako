package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// ErrSessionExpired reports a session whose lifetime has ended; the caller
// must ask the student to sign in again.
var ErrSessionExpired = errors.New("session expired")

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
	clock        ports.Clock
	idgen        ports.IDGen
	tokengen     ports.TokenGen
	loginURLBase string
	sessionTTL   time.Duration
}

// NewAuth wires the authentication use cases to their ports.
func NewAuth(
	users ports.UserRepo,
	sessions ports.SessionRepo,
	tokens ports.LoginTokenRepo,
	mailer ports.Mailer,
	clock ports.Clock,
	idgen ports.IDGen,
	tokengen ports.TokenGen,
	loginURLBase string,
	sessionTTL time.Duration,
) *Auth {
	return &Auth{
		users:        users,
		sessions:     sessions,
		tokens:       tokens,
		mailer:       mailer,
		clock:        clock,
		idgen:        idgen,
		tokengen:     tokengen,
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

// SignInWithGoogle finds or creates the account behind a Google identity and
// mints a fresh session for it.
func (uc *Auth) SignInWithGoogle(ctx context.Context, rawEmail, name string) (identity.Session, error) {
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		return identity.Session{}, err
	}
	user, err := uc.users.GetByEmail(ctx, email)
	if errors.Is(err, ports.ErrNotFound) {
		user, err = identity.NewUser(uc.idgen.NewID(), email, name, identity.RoleStudent, studentTimezone)
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
