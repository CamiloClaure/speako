package google

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/camiloclaure/speako/api/internal/app/ports"
)

func b64url(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// The three checks that come before any network call must reject a forged
// token entirely offline - see Verify's doc comment. A machine with no
// outbound network still has to fail these instantly, not hang on a JWKS
// fetch it can never complete.
func TestVerifyRejectsMalformedTokensWithoutNetwork(t *testing.T) {
	v := NewVerifier("some-client-id")
	ctx := context.Background()

	cases := map[string]string{
		"not three dot-separated parts": "not-a-jwt",
		"alg is not RS256":              b64url(`{"alg":"HS256","kid":"k1"}`) + ".e30.sig",
		"kid is empty":                  b64url(`{"alg":"RS256"}`) + ".e30.sig",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(ctx, token); !errors.Is(err, ports.ErrInvalidGoogleToken) {
				t.Fatalf("err = %v, want ErrInvalidGoogleToken", err)
			}
		})
	}
}
