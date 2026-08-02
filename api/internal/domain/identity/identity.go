package identity

import (
	"strings"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	ErrInvalidUser     = shared.NewDomainError("user.invalid", "invalid user")
	ErrInvalidRole     = shared.NewDomainError("user.role", "invalid role")
	ErrInvalidTimezone = shared.NewDomainError("user.timezone", "invalid timezone")
	ErrEmptyName       = shared.NewDomainError("user.name", "name must not be empty")
	ErrInvalidToken    = shared.NewDomainError("session.token", "invalid session token")
)

// Role is the user's access level. Values are English in code, pt-BR in UI.
type Role string

const (
	RoleStudent Role = "student"
	RoleTeacher Role = "teacher"
	RoleAdmin   Role = "admin"
)

func (r Role) IsStudent() bool { return r == RoleStudent }
func (r Role) IsTeacher() bool { return r == RoleTeacher }
func (r Role) IsAdmin() bool   { return r == RoleAdmin }

// Valid reports whether r is one of the known roles.
func (r Role) Valid() bool {
	return r == RoleStudent || r == RoleTeacher || r == RoleAdmin
}

// User is a person on the platform. Timezone is an IANA name such as
// "America/Sao_Paulo", validated at construction; it is converted to a
// *time.Location only at the edges.
type User struct {
	ID       shared.ID
	Email    shared.EmailAddress
	Name     string
	Role     Role
	Timezone string
}

// NewUser validates every field and returns a normalized user.
func NewUser(id shared.ID, email shared.EmailAddress, name string, role Role, timezone string) (User, error) {
	if id.IsZero() {
		return User{}, ErrInvalidUser
	}
	if email.String() == "" {
		return User{}, ErrInvalidUser
	}
	if strings.TrimSpace(name) == "" {
		return User{}, ErrEmptyName
	}
	if !role.Valid() {
		return User{}, ErrInvalidRole
	}
	if timezone == "" {
		return User{}, ErrInvalidTimezone
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return User{}, ErrInvalidTimezone
	}
	return User{
		ID:       id,
		Email:    email,
		Name:     name,
		Role:     role,
		Timezone: timezone,
	}, nil
}

// Location returns the user's timezone as a loaded location. It cannot fail
// because NewUser already validated it.
func (u User) Location() *time.Location {
	loc, _ := time.LoadLocation(u.Timezone)
	return loc
}

// Session is a value that carries who a token belongs to and when it dies.
type Session struct {
	Token     shared.ID
	UserID    shared.ID
	ExpiresAt time.Time
}

// IsExpired reports whether the session is dead at now. A session expires at
// exactly ExpiresAt, so the boundary second counts as expired.
func (s Session) IsExpired(now time.Time) bool {
	return !s.ExpiresAt.After(now)
}

// IsValid reports whether the session is live at now and carries a user.
func (s Session) IsValid(now time.Time) bool {
	return !s.UserID.IsZero() && !s.Token.IsZero() && !s.IsExpired(now)
}
