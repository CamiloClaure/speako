package fakes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// The fakes must satisfy the ports they stand in for.
var (
	_ ports.SessionRepo    = (*SessionRepo)(nil)
	_ ports.LoginTokenRepo = (*LoginTokenRepo)(nil)
)

func testEmail(t *testing.T) shared.EmailAddress {
	t.Helper()
	e, err := shared.ParseEmail("aluna@exemplo.com.br")
	if err != nil {
		t.Fatalf("ParseEmail: %v", err)
	}
	return e
}

func TestLoginTokenConsumeIsSingleUse(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	repo := &LoginTokenRepo{}

	if err := repo.Create(ctx, ports.LoginToken{
		TokenHash: "hash-1",
		Email:     testEmail(t),
		ExpiresAt: now.Add(15 * time.Minute),
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Consume(ctx, "hash-1", now)
	if err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	if !got.UsedAt.Equal(now) {
		t.Errorf("UsedAt = %v, want %v", got.UsedAt, now)
	}

	// A forwarded or re-clicked link must not log anyone in a second time.
	if _, err := repo.Consume(ctx, "hash-1", now.Add(time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Consume err = %v, want ErrNotFound", err)
	}
}

func TestLoginTokenConsumeRejectsExpired(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	expires := now.Add(15 * time.Minute)
	repo := &LoginTokenRepo{}

	if err := repo.Create(ctx, ports.LoginToken{TokenHash: "h", Email: testEmail(t), ExpiresAt: expires}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Expiry is exclusive: at exactly ExpiresAt the token is already dead.
	if _, err := repo.Consume(ctx, "h", expires); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Consume at expiry err = %v, want ErrNotFound", err)
	}
	if _, err := repo.Consume(ctx, "h", expires.Add(-time.Second)); err != nil {
		t.Fatalf("Consume one second before expiry: %v", err)
	}
}

func TestSessionRepoDeleteAndExpire(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	repo := &SessionRepo{}

	sessions := []identity.Session{
		{Token: shared.ID("live"), UserID: shared.ID("u1"), ExpiresAt: now.Add(time.Hour)},
		{Token: shared.ID("dead"), UserID: shared.ID("u1"), ExpiresAt: now.Add(-time.Hour)},
	}
	for _, s := range sessions {
		if err := repo.Create(ctx, s); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	if err := repo.DeleteExpired(ctx, now); err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if _, err := repo.GetByTokenHash(ctx, shared.ID("dead")); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session survived: %v", err)
	}
	if _, err := repo.GetByTokenHash(ctx, shared.ID("live")); err != nil {
		t.Errorf("live session was expunged: %v", err)
	}

	if err := repo.Delete(ctx, shared.ID("live")); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByTokenHash(ctx, shared.ID("live")); !errors.Is(err, ErrNotFound) {
		t.Errorf("logout left the session usable: %v", err)
	}
}

func TestFakesForceErrors(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")

	sessions := &SessionRepo{Err: boom}
	dummy := identity.Session{Token: shared.ID("x"), UserID: shared.ID("u1"), ExpiresAt: time.Now()}
	if err := sessions.Create(ctx, dummy); !errors.Is(err, boom) {
		t.Errorf("SessionRepo.Create err = %v, want boom", err)
	}
	tokens := &LoginTokenRepo{Err: boom}
	if _, err := tokens.Consume(ctx, "x", time.Now()); !errors.Is(err, boom) {
		t.Errorf("LoginTokenRepo.Consume err = %v, want boom", err)
	}
}
