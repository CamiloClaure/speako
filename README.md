# Speako

English e-learning for Brazilian learners, run by one teacher. Sells digital materials and 1:1 lesson packs;
students spend credits to book slots. Video calls happen outside the platform.

- **Backend** Go 1.26, `net/http`, SQLite (`modernc.org/sqlite`, no cgo), no framework, no ORM
- **Frontend** React 19 + Vite + Tailwind v4, public pages prerendered to static HTML
- **Payments** Mercado Pago Checkout Pro (Pix, card up to 12x, boleto)
- **Infra** one VPS in São Paulo + Caddy + systemd, Litestream → Cloudflare R2, SPA on Cloudflare Pages

Read `AGENTS.md` before writing code, and `docs/architecture.md` before deciding where code goes.

## Setup

Go lives in `~/.local/go`. Add it to your shell once:

```bash
echo 'export PATH=$HOME/.local/go/bin:$PATH' >> ~/.zshrc && source ~/.zshrc
```

```bash
cp .env.example .env      # fill in the secrets
make seed                 # reset the dev database with teacher, availability, products
make run                  # API on :8080
make web-dev              # SPA on :5173 (proxies /api)
```

## Commands

```bash
make test     # unit + contract tests
make arch     # the dependency rule - must pass before any task is "done"
make lint     # go vet + gofmt
make e2e      # Playwright against a live stack
make build    # static binary + web bundle
```

## Working on this repo

Work is cut into sessions in `docs/sessions/`. Each one declares the files it owns, its tasks, and a
definition of done. Run one at a time:

```bash
opencode run "$(cat docs/sessions/S1-domain.md)"
```

Order: `S0 → S1 → S2 → S3`, then the backend track (`S4…S8`) and frontend track (`B1…B5`) in parallel on
separate branches, then `S14 → S15 → S16`.

`docs/api.yaml` is frozen after S3. Changing it breaks the other track silently, so it gets its own session.
