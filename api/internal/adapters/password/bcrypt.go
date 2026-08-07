// Package password implements ports.PasswordHasher with bcrypt.
package password

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/camiloclaure/speako/api/internal/app/ports"
)

var _ ports.PasswordHasher = Bcrypt{}

// Bcrypt hashes and verifies passwords with bcrypt at the library's default
// cost.
type Bcrypt struct{}

// Hash returns the bcrypt hash of plain.
//
// This bcrypt refuses input over 72 bytes outright (ErrPasswordTooLong) rather
// than truncating it, so identity.ValidatePassword rejects a long passphrase
// before it ever reaches here. Callers that skip that check get an error, not
// a silently weakened hash.
func (Bcrypt) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Matches reports whether plain is the password behind hash.
func (Bcrypt) Matches(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
