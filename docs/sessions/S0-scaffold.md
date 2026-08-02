# S0 - Scaffold and rules  [DONE]

Completed before the opencode runs started. Recorded here so the sequence is reproducible.

Delivered: repo tree, `AGENTS.md`, `Makefile`, `docs/architecture.md`, `.env.example`, CI workflow,
Go module with per-layer `doc.go` stubs, `api/internal/arch/arch_test.go` (verified to fail on both an
internal-layer leak and a forbidden stdlib import), Vite + React 19 + Tailwind v4 scaffold with the
brand tokens, and a working `npm run build`.

Prerequisite for every later session: `export PATH=$HOME/.local/go/bin:$PATH` (Go 1.26.5 lives there).
