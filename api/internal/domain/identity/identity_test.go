package identity

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

func mustEmail(t *testing.T, raw string) shared.EmailAddress {
	t.Helper()
	e, err := shared.ParseEmail(raw)
	if err != nil {
		t.Fatalf("ParseEmail(%q): %v", raw, err)
	}
	return e
}

func TestNewUser(t *testing.T) {
	email := mustEmail(t, "Ana@Speako.com.br")
	tests := []struct {
		name     string
		id       shared.ID
		email    shared.EmailAddress
		nm       string
		role     Role
		timezone string
		wantErr  error
	}{
		{"valid student", "u1", email, "Ana", RoleStudent, "America/Sao_Paulo", nil},
		{"valid teacher", "u2", mustEmail(t, "t@speako.com"), "Teacher", RoleTeacher, "America/Sao_Paulo", nil},
		{"valid admin", "u3", mustEmail(t, "a@speako.com"), "Admin", RoleAdmin, "America/New_York", nil},
		{"empty id", "", email, "Ana", RoleStudent, "America/Sao_Paulo", ErrInvalidUser},
		{"empty email", "u1", "", "Ana", RoleStudent, "America/Sao_Paulo", ErrInvalidUser},
		{"blank name", "u1", email, "   ", RoleStudent, "America/Sao_Paulo", ErrEmptyName},
		{"unknown role", "u1", email, "Ana", Role("superuser"), "America/Sao_Paulo", ErrInvalidRole},
		{"garbage timezone", "u1", email, "Ana", RoleStudent, "America/Nope", ErrInvalidTimezone},
		{"empty timezone", "u1", email, "Ana", RoleStudent, "", ErrInvalidTimezone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := NewUser(tt.id, tt.email, tt.nm, tt.role, tt.timezone)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewUser error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && u.Email != tt.email {
				t.Fatalf("NewUser did not carry normalized email: got %q, want %q", u.Email, tt.email)
			}
		})
	}
}

func TestRolePredicates(t *testing.T) {
	tests := []struct {
		role                           Role
		student, teacher, admin, valid bool
	}{
		{RoleStudent, true, false, false, true},
		{RoleTeacher, false, true, false, true},
		{RoleAdmin, false, false, true, true},
		{Role(""), false, false, false, false},
		{Role("owner"), false, false, false, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.IsStudent(); got != tt.student {
				t.Errorf("IsStudent() = %v, want %v", got, tt.student)
			}
			if got := tt.role.IsTeacher(); got != tt.teacher {
				t.Errorf("IsTeacher() = %v, want %v", got, tt.teacher)
			}
			if got := tt.role.IsAdmin(); got != tt.admin {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.admin)
			}
			if got := tt.role.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestUserLocationLoads(t *testing.T) {
	u, err := NewUser("u1", mustEmail(t, "a@speako.com"), "Ana", RoleStudent, "America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	if loc := u.Location(); loc.String() != "America/Sao_Paulo" {
		t.Fatalf("Location() = %v, want America/Sao_Paulo", loc)
	}
}

func TestSessionExpiry(t *testing.T) {
	now := time.Date(2026, 1, 5, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{"valid well before", now.Add(24 * time.Hour), now, false},
		{"valid one second before", now.Add(time.Second), now, false},
		{"expired at exact instant", now, now, true},
		{"expired one second after", now, now.Add(time.Second), true},
		{"zero expiry is expired", time.Time{}, now, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Session{Token: "tok", UserID: "u1", ExpiresAt: tt.expiresAt}
			if got := s.IsExpired(tt.now); got != tt.want {
				t.Fatalf("IsExpired() = %v, want %v", got, tt.want)
			}
			if got := s.IsValid(tt.now); got == tt.want && tt.want {
				t.Fatalf("IsValid() should not be true when expired, got %v", got)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		plain   string
		wantErr error
	}{
		{"empty", "", ErrWeakPassword},
		{"seven chars fails", "1234567", ErrWeakPassword},
		{"eight chars passes", "12345678", nil},
		{"long passphrase passes", "correct horse battery staple", nil},
		// bcrypt refuses anything over 72 bytes, so the boundary is a real
		// limit of the hasher, not a taste preference. Off by one here and a
		// person with a password manager gets a 500 instead of a message.
		{"seventy-two bytes passes", strings.Repeat("a", 72), nil},
		{"seventy-three bytes fails", strings.Repeat("a", 73), ErrPasswordTooLong},
		// Bytes, not runes: accented pt-BR text costs more than it looks.
		{"accented text counts bytes", strings.Repeat("á", 37), ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.plain)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidatePassword(%q) error = %v, want %v", tt.plain, err, tt.wantErr)
			}
		})
	}
}

func TestSessionRequiresIdentity(t *testing.T) {
	now := time.Date(2026, 1, 5, 14, 0, 0, 0, time.UTC)
	s := Session{Token: "tok", ExpiresAt: now.Add(time.Hour)}
	if s.IsValid(now) {
		t.Fatal("session without a user must not be valid")
	}
	s2 := Session{UserID: "u1", ExpiresAt: now.Add(time.Hour)}
	if s2.IsValid(now) {
		t.Fatal("session without a token must not be valid")
	}
}
