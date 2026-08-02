# S14 - Integration and end-to-end

**Branch:** `main` (both tracks merge here first)
**You own:** `web/e2e/**`, `docs/verify.md`, glue fixes across tracks

## Goal

Prove the two halves actually fit, then lock the money paths behind tests.

## Tasks

1. Merge `be/*` and `fe/*`. Point the SPA at the real API, drop MSW from the dev path (keep it for unit tests).
2. Fix contract drift against `docs/api.yaml`. If the spec was wrong, fix the spec **and** both sides.
3. Playwright specs, from a clean database seeded by the CLI:
   - buy a material (signed webhook fixture) → it appears in `/painel/materiais` → download returns 200
   - buy a pack → credits appear → book a slot → credit debited by exactly one
   - cancel ≥ 24h → credit returned
   - cancel < 24h → credit consumed
   - book a slot already taken → `slot_taken` surfaced in the UI
   - unauthenticated access to `/painel` and `/admin` redirects
4. Write `docs/verify.md`: the curl walkthrough plus how to replay a webhook locally.

## Definition of done

- [ ] `make e2e` is green from an empty database, twice in a row (no test depends on leftover state).
- [ ] No `console.error` during the E2E run.
- [ ] `make test && make arch && make lint` all green on `main`.
