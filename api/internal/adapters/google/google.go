// Package google implements Google sign-in: id_token verification against the
// published JWKS.
package google

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var _ ports.GoogleVerifier = (*Verifier)(nil)

// jwksURL is Google's published JSON Web Key Set for Sign-In tokens.
const jwksURL = "https://www.googleapis.com/oauth2/v3/certs"

// keyCacheTTL bounds how long a cached key is trusted before a fresh fetch is
// forced, so a key Google rotates out eventually stops being accepted even if
// its kid happens to still be requested.
const keyCacheTTL = time.Hour

// googleIssuers are the two values Google is known to put in a token's iss
// claim; either is accepted.
var googleIssuers = map[string]bool{
	"accounts.google.com":         true,
	"https://accounts.google.com": true,
}

// jwk is the subset of a JSON Web Key this verifier understands - RSA keys
// only, which is all Google publishes for Sign-In tokens.
type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

// Verifier verifies Google Sign-In id_tokens against Google's published
// JWKS. Keys are cached in memory and refreshed at most once an hour, so a
// steady stream of sign-ins costs one outbound request per hour, not one per
// request.
type Verifier struct {
	clientID string
	client   *http.Client

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// NewVerifier returns a Verifier that only accepts tokens whose aud claim
// equals clientID.
func NewVerifier(clientID string) *Verifier {
	return &Verifier{
		clientID: clientID,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// Verify checks idToken's structure, signature and claims and returns the
// identity it attests to. Every check below runs in this order, and the
// first three never touch the network - a malformed or algorithm-confused
// token is rejected before Verify reaches for Google's JWKS, so a machine
// with no outbound network still rejects a forged token instantly instead of
// hanging on a fetch that can never succeed.
func (v *Verifier) Verify(ctx context.Context, idToken string) (ports.GoogleIdentity, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	// alg is compared against a hardcoded constant, never used to select how
	// verification runs below - trusting the token's own claim of its
	// algorithm is the classic algorithm-confusion hole (e.g. "none", or
	// resigning as HMAC using the RSA public key as the secret).
	if header.Alg != "RS256" {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	// Google always sets kid. A forged token that omits it must be rejected
	// here, offline - without this check it looks like a cache miss and sends
	// this call chasing a JWKS fetch it can never satisfy.
	if header.Kid == "" {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}

	key, err := v.keyFor(ctx, header.Kid)
	if err != nil {
		return ports.GoogleIdentity{}, err
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig); err != nil {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}

	var claims struct {
		Aud           string `json:"aud"`
		Iss           string `json:"iss"`
		Exp           int64  `json:"exp"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := decodeSegment(parts[1], &claims); err != nil {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	if claims.Aud != v.clientID {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	if !googleIssuers[claims.Iss] {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	if !time.Unix(claims.Exp, 0).After(time.Now()) {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	if !claims.EmailVerified {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}
	email, err := shared.ParseEmail(claims.Email)
	if err != nil {
		return ports.GoogleIdentity{}, ports.ErrInvalidGoogleToken
	}

	return ports.GoogleIdentity{Email: email, Name: claims.Name}, nil
}

// keyFor returns the public key for kid, refreshing the cache on a miss or
// once it is older than keyCacheTTL. If kid is still absent after a refresh,
// the token is invalid, not the network's fault.
func (v *Verifier) keyFor(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	key, ok := v.keys[kid]
	stale := time.Since(v.fetchedAt) > keyCacheTTL
	v.mu.Unlock()
	if ok && !stale {
		return key, nil
	}
	if err := v.refreshKeys(ctx); err != nil {
		// A fetch failure is not "invalid token" - it is a JWKS outage, and the
		// two must be distinguishable to whoever handles this error.
		return nil, fmt.Errorf("google: fetch jwks: %w", err)
	}
	v.mu.Lock()
	key, ok = v.keys[kid]
	v.mu.Unlock()
	if !ok {
		return nil, ports.ErrInvalidGoogleToken
	}
	return key, nil
}

// refreshKeys fetches Google's current JWKS and replaces the cache wholesale.
func (v *Verifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var parsed jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(parsed.Keys))
	for _, k := range parsed.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKey(k.N, k.E)
		if err != nil {
			continue // skip a malformed key rather than failing the whole refresh
		}
		keys[k.Kid] = pub
	}

	v.mu.Lock()
	v.keys, v.fetchedAt = keys, time.Now()
	v.mu.Unlock()
	return nil
}

// rsaPublicKey decodes a JWK's base64url-encoded modulus and exponent into an
// *rsa.PublicKey.
func rsaPublicKey(nEnc, eEnc string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nEnc)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eEnc)
	if err != nil {
		return nil, err
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}

// decodeSegment base64url-decodes a JWT segment and unmarshals it as JSON.
func decodeSegment(seg string, v any) error {
	data, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
