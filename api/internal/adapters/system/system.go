package system

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/google/uuid"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	_ ports.Clock    = Clock{}
	_ ports.IDGen    = IDGen{}
	_ ports.TokenGen = TokenGen{}
)

// Clock is the real wall clock, always in UTC so nothing downstream has to
// guess which zone a stored timestamp was in.
type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

// IDGen issues UUIDv7 identifiers: random enough to be unguessable in a URL,
// ordered enough that an index on them stays compact.
type IDGen struct{}

func (IDGen) NewID() shared.ID {
	id, err := uuid.NewV7()
	if err != nil {
		// NewV7 only fails if the system entropy source does, in which case
		// issuing a guessable identifier is worse than stopping.
		panic("system: cannot generate id: " + err.Error())
	}
	return shared.ID(id.String())
}

// TokenGen issues session and magic-link secrets. The raw value is shown once
// and never stored; only Hash(raw) is persisted, so a database leak cannot be
// replayed as a login.
type TokenGen struct{}

func (TokenGen) NewToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, TokenGen{}.Hash(raw), nil
}

// Hash is the one hashing implementation in the system. Callers must never
// re-derive it themselves: two implementations drift, and the day they do,
// every login stops matching with no error to explain why.
func (TokenGen) Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
