# S3 - Contract freeze  (gate for the parallel phase)

**Read first:** `AGENTS.md`, the API contract section of the approved plan, `api/internal/app`
**Branch:** `be/s3-contract`
**You own:** `docs/api.yaml`, `docs/fixtures/*.json`, `api/internal/http/dto/**`, `web/src/api/**`

## Goal

Freeze the wire contract so the backend and frontend tracks can run in parallel without talking to each other.

## Tasks

1. Write `docs/api.yaml` (OpenAPI 3.1) for every endpoint in the plan, including:
   - the error envelope `{"error":{"code":"...","message":"..."}}` with `code` as a **closed enum**:
     `unauthenticated, forbidden, not_found, validation_failed, insufficient_credits, slot_taken,
     too_late_to_book, too_late_to_cancel, already_purchased, payment_failed, rate_limited, internal`
   - money as integer cents with a `price_cents` naming convention (never a float, never a formatted string)
   - all timestamps as RFC3339 UTC strings on the wire
2. Generate `web/src/api/types.ts` from the spec (`openapi-typescript`), committed.
3. Write `api/internal/http/dto` structs mirroring the spec exactly, with `json:` tags. These are the only
   structs in the repo allowed to have them.
4. Hand-write `docs/fixtures/*.json` - one realistic response per endpoint, in pt-BR, with believable prices
   (material R$ 47,00; packs of 4/8/12 lessons) - and wire them into MSW handlers at `web/src/api/mocks`.
5. Write `web/src/api/client.ts`: a thin `fetch` wrapper with `credentials: "include"`, typed responses, and
   error-envelope parsing. No data-fetching library.

## Definition of done

- [ ] `npx @redocly/cli lint docs/api.yaml` is clean.
- [ ] `npm run lint` passes with the generated types in place.
- [ ] `npm run dev` renders a page fed entirely by MSW fixtures, with the Go API not running.
- [ ] A short `docs/decisions/001-contract-freeze.md` records that changing `api.yaml` afterwards requires its
      own session touching both tracks.
