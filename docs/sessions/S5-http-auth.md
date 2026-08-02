# S5 - HTTP adapter and authentication

**Branch:** `be/s5-http`
**You own:** `api/internal/http/**` (except `dto/`, frozen in S3), `api/internal/adapters/{system,resend,google}/**`,
`api/cmd/api/main.go`

## Goal

Expose the use cases over HTTP and let a student sign in without a password.

## Tasks

1. Router on `net/http` `ServeMux` with method patterns (`POST /api/v1/bookings`). No third-party router.
2. Middleware chain: request-id → panic recovery → structured access log (no PII) → CORS locked to
   `SPEAKO_PUBLIC_URL` with credentials → rate limit → session resolution.
3. `internal/http/errors.go`: the **single** place mapping domain sentinels to status + error code from the
   frozen enum. `ErrInsufficientCredits` → 409, `ErrTooLateToBook` → 422, `slot_taken` → 409, unknown → 500
   with the detail logged and never returned.
4. Magic link: `POST /auth/magic-link` stores only `sha256(token)` with a 15-minute expiry, single use;
   `verify` rotates the session on success. Response is identical whether or not the email exists.
5. Google sign-in: verify the `id_token` signature against Google's JWKS (cache the keys), check `aud` and
   `exp`, then match or create the user.
6. Sessions: opaque random token, stored hashed, cookie `HttpOnly; Secure; SameSite=Lax; Path=/`, TTL from
   config. Logout deletes the row, not just the cookie.
7. Wire the real composition root in `cmd/api/main.go`: config → db → repos → adapters → use cases → router,
   with graceful shutdown already in place.
8. `system` adapter: `Clock`, `IDGen` (UUIDv7), `TokenGen` (`crypto/rand`, 32 bytes, base64url).

## Definition of done

- [ ] `docs/verify.md` contains a curl walkthrough that runs green end to end against a seeded database.
- [ ] Rate limiting is per-IP **and** per-email on auth endpoints; exceeding it returns 429 `rate_limited`.
- [ ] Handler tests use `httptest` and the fakes - no database needed.
- [ ] `make arch` passes: `internal/http` imports **no** adapter package.
- [ ] No token, email or payload appears in any log line.
