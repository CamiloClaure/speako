package http_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/camiloclaure/speako/api/internal/http/dto"
)

// The 6th login attempt inside the 15-minute window, from the same IP
// against the same email, must be rejected with 429 - regardless of whether
// any of the first 5 would themselves have succeeded.
func TestLoginRateLimitBlocksSixthAttempt(t *testing.T) {
	rig := newTestRig(t)
	const attackerIP = "203.0.113.10:1234"
	body := `{"email":"attacker@test.com","password":"wrong-password"}`

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		req.RemoteAddr = attackerIP
		rr := httptest.NewRecorder()
		rig.mux.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was rate-limited too early; body=%s", i+1, rr.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.RemoteAddr = attackerIP
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt status = %d, want %d; body=%s", rr.Code, http.StatusTooManyRequests, rr.Body.String())
	}
	var envelope dto.ErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}
	if envelope.Error.Code != "rate_limited" {
		t.Errorf("code = %q, want %q", envelope.Error.Code, "rate_limited")
	}

	// A different email from the same IP has its own budget: an attacker
	// exhausting one target's bucket must not collaterally block every other
	// account behind the same address.
	otherReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"someone-else@test.com","password":"x"}`))
	otherReq.RemoteAddr = attackerIP
	otherRR := httptest.NewRecorder()
	rig.mux.ServeHTTP(otherRR, otherReq)
	if otherRR.Code == http.StatusTooManyRequests {
		t.Fatalf("a different email from the same IP was blocked too; body=%s", otherRR.Body.String())
	}
}

// Password spraying is the attack the per-email bucket is blind to: one
// caller trying a single common password against many different accounts
// never spends more than one attempt on any of them. Only a per-IP ceiling
// catches it.
func TestLoginRateLimitStopsPasswordSpray(t *testing.T) {
	rig := newTestRig(t)
	const sprayerIP = "198.51.100.7:5555"

	blocked := 0
	for i := 0; i < 40; i++ {
		body := fmt.Sprintf(`{"email":"alvo-%d@test.com","password":"senha123"}`, i)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		req.RemoteAddr = sprayerIP
		rr := httptest.NewRecorder()
		rig.mux.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked == 0 {
		t.Fatal("40 attempts against 40 different accounts from one IP were all allowed - " +
			"a sprayer walks straight through the per-email bucket")
	}

	// The ceiling must not be so tight that it punishes a shared address: a
	// household or office behind one NAT has to survive a handful of tries.
	unblockedIP := "198.51.100.8:5555"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"vizinho@test.com","password":"x"}`))
	req.RemoteAddr = unblockedIP
	rr := httptest.NewRecorder()
	rig.mux.ServeHTTP(rr, req)
	if rr.Code == http.StatusTooManyRequests {
		t.Fatal("an unrelated IP was blocked by another address's spray")
	}
}
