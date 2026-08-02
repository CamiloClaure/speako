# S15 - Deploy and operations

**Branch:** `ops/s15-deploy`
**You own:** `deploy/**`, `Caddyfile`, `deploy/speako.service`, `deploy/litestream.yml`,
`.github/workflows/deploy.yml`

## Goal

Running in São Paulo, backed up, and deployable in one command.

## Tasks

1. VPS in São Paulo. Non-root user, ufw allowing 22/80/443 only, unattended-upgrades on.
2. Caddy: automatic TLS, reverse proxy to the binary on 127.0.0.1:8080, security headers (HSTS,
   `X-Content-Type-Options`, a CSP that fits a static SPA), gzip/zstd.
3. systemd unit: `Restart=always`, `EnvironmentFile=/etc/speako/env` (mode 0600, root-owned),
   `NoNewPrivileges`, `ProtectSystem=strict` with a writable path for the database.
4. Litestream replicating the SQLite file to Cloudflare R2 continuously.
5. Frontend to Cloudflare Pages; CORS on the API locked to that exact origin.
6. `slog` JSON to journald, `/healthz` monitored by an external uptime check, alert to email.
7. `deploy/deploy.sh`: build static binary → upload → `systemctl restart` → health check → roll back on
   failure. GitHub Actions calls it on a tag.
8. DNS, SPF, DKIM and DMARC for the sending domain - magic-link login depends on deliverability.

## Definition of done

- [ ] `curl https://<domain>/healthz` returns 200 over TLS with a valid certificate.
- [ ] **Restore drill actually performed**: `litestream restore` into a temp file, row counts for `orders` and
      `bookings` match production. Document the exact commands in `docs/runbook.md`.
- [ ] A deploy takes one command and rolls back automatically on a failed health check.
- [ ] Secrets exist only in `/etc/speako/env` and GitHub Actions secrets - never in the repo.
- [ ] A test email from the production sender passes SPF/DKIM/DMARC checks.
