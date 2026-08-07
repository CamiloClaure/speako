# 001 - Contract freeze

`docs/api.yaml` is the frozen wire contract between the Go API and the React SPA. This session
(`feat/auth-roles`, password sign-in + `/auth/google`) is what actually produced it - S3 never met its
definition of done, so the API and the frontend track have been drifting against a contract that only existed
as an idea. Not anymore.

## Changing this contract

Editing `docs/api.yaml` after this freeze requires its own session that touches both tracks (the Go DTOs in
`api/internal/http/dto` and the TypeScript client in `web/src/api/client.ts`) in the same change. A one-sided
edit is how the drift in this session happened in the first place.

## `already_purchased` was removed from the error-code enum

S3's plan listed 12 codes including `already_purchased`. It is deleted here: nothing in
`api/internal/http/errors.go` emits it and no screen consumes it. The closed enum is now the 14 codes actually
in use - see `ErrorCode` in `docs/api.yaml`.

## Deliberately out of scope

Two things S3's plan called for are not done here, on purpose, not by oversight:

- **Generated `web/src/api/types.ts`.** `client.ts` hand-types every response and nothing in the SPA imports
  from a generated types module. Generating one now would be code with zero consumers. Pay this when a second
  client of the contract appears (e.g. a mobile app, an admin CLI) or when a response shape gets non-trivial
  enough that hand-typing starts drifting from `docs/api.yaml` on its own.
- **`docs/fixtures/*.json` + MSW handlers.** Nothing in `web/src/api/mocks` exists yet and nothing imports
  fixtures. Pay this when a frontend test needs to render a screen without the Go API running.
