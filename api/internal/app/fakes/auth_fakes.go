package fakes

import (
	"context"
	"sync"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// SessionRepo is an in-memory SessionRepo. Set Err to force every method to
// fail.
type SessionRepo struct {
	mu       sync.Mutex
	byHash   map[shared.ID]identity.Session
	Err      error
	Deleted  []shared.ID
	Expunged int
}

func (r *SessionRepo) init() {
	if r.byHash == nil {
		r.byHash = map[shared.ID]identity.Session{}
	}
}

func (r *SessionRepo) Create(ctx context.Context, s identity.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.byHash[s.Token] = s
	return nil
}

func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash shared.ID) (identity.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return identity.Session{}, r.Err
	}
	s, ok := r.byHash[tokenHash]
	if !ok {
		return identity.Session{}, ErrNotFound
	}
	return s, nil
}

func (r *SessionRepo) Delete(ctx context.Context, tokenHash shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	delete(r.byHash, tokenHash)
	r.Deleted = append(r.Deleted, tokenHash)
	return nil
}

func (r *SessionRepo) DeleteExpired(ctx context.Context, before time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	for hash, s := range r.byHash {
		if s.ExpiresAt.Before(before) {
			delete(r.byHash, hash)
			r.Expunged++
		}
	}
	return nil
}

// LoginTokenRepo is an in-memory LoginTokenRepo. Consume is single-use: the
// second call for the same hash reports ErrNotFound, which is what stops a
// forwarded magic link from logging someone in twice.
type LoginTokenRepo struct {
	mu     sync.Mutex
	byHash map[string]ports.LoginToken
	Err    error
}

func (r *LoginTokenRepo) init() {
	if r.byHash == nil {
		r.byHash = map[string]ports.LoginToken{}
	}
}

func (r *LoginTokenRepo) Create(ctx context.Context, t ports.LoginToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.byHash[t.TokenHash] = t
	return nil
}

func (r *LoginTokenRepo) Consume(ctx context.Context, tokenHash string, now time.Time) (ports.LoginToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return ports.LoginToken{}, r.Err
	}
	r.init()
	t, ok := r.byHash[tokenHash]
	if !ok || !t.UsedAt.IsZero() || !t.ExpiresAt.After(now) {
		return ports.LoginToken{}, ErrNotFound
	}
	t.UsedAt = now
	r.byHash[tokenHash] = t
	return t, nil
}

func (r *LoginTokenRepo) DeleteExpired(ctx context.Context, before time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	for hash, t := range r.byHash {
		if t.ExpiresAt.Before(before) {
			delete(r.byHash, hash)
		}
	}
	return nil
}

// PasswordHasher is a fast fake of ports.PasswordHasher: Hash just tags the
// plaintext and Matches checks the tag. It is not a real hash - it exists so
// Auth use case tests do not pay for real bcrypt on every run. Set Err to
// force Hash to fail. MatchesCalls counts calls to Matches, so a test can
// prove a comparison happened without measuring wall-clock time.
type PasswordHasher struct {
	mu           sync.Mutex
	Err          error
	MatchesCalls int
}

const fakeHashTag = "fake-hash:"

func (h *PasswordHasher) Hash(plain string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Err != nil {
		return "", h.Err
	}
	return fakeHashTag + plain, nil
}

func (h *PasswordHasher) Matches(hash, plain string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.MatchesCalls++
	return hash == fakeHashTag+plain
}

// PaymentEventRepo is an in-memory PaymentEventRepo. Claim is a real claim:
// only the first caller for an eventID gets true, which is what a test needs
// to prove a redelivered webhook grants nothing twice.
type PaymentEventRepo struct {
	mu       sync.Mutex
	claimed  map[string]shared.ID
	Err      error
	Attempts int
}

func (r *PaymentEventRepo) Claim(ctx context.Context, eventID string, orderID shared.ID, now time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return false, r.Err
	}
	if r.claimed == nil {
		r.claimed = map[string]shared.ID{}
	}
	r.Attempts++
	if _, seen := r.claimed[eventID]; seen {
		return false, nil
	}
	r.claimed[eventID] = orderID
	return true, nil
}
