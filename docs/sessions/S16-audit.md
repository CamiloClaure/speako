# S16 - Audit and hardening

**Run in Claude Code**, not opencode: `/architecture-audit` then `/security-audit`.
**You own:** `ARCHITECTURE_AUDIT.md`, `SECURITY.md`, `docs/debt.md`, plus the fixes they justify.

## Goal

Find what drifted before real money flows through it.

## Tasks

1. `/architecture-audit` on the repo. Expect the honest bar for an MVP - fix Critical and High, record the rest.
2. `/security-audit`. Non-negotiable checks:
   - webhook signature verification cannot be bypassed
   - no user enumeration on the auth endpoints
   - session cookie flags and rotation on login
   - signed URLs cannot be lengthened or reused after expiry
   - every SQL statement parameterized
   - no PII or token in any log line
   - rate limits on auth, checkout and booking
3. `grep -rn "ponytail:"` → collect into `docs/debt.md` with the upgrade trigger for each, so deliberate
   shortcuts stay deliberate.
4. Load test: `oha -z 30s -c 50` against `/api/v1/products` and `/api/v1/slots`.
5. LGPD pass: consent recorded with timestamp and IP, a documented deletion path, privacy policy matching what
   the code actually collects.

## Definition of done

- [ ] Zero Critical or High findings open.
- [ ] p95 < 50 ms on the two hot endpoints; process RSS < 80 MB under load.
- [ ] `docs/debt.md` lists every `ponytail:` marker with its trigger.
- [ ] A written go/no-go note on switching Mercado Pago out of sandbox.
