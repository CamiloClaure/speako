#!/bin/sh
# Arranca Speako + túnel público (cloudflared quick tunnel, URL aleatoria
# *.trycloudflare.com). Requiere cloudflared instalado. Ctrl+C corta los tres
# procesos: túnel, API y Vite.
#
# A diferencia de mechasys, aquí hay dos procesos: la API en :8080 y el SPA en
# :5173. El túnel apunta sólo al SPA porque Vite ya proxya /api a la API, así
# que las cookies siguen siendo same-origin desde afuera.
set -e
cd "$(dirname "$0")"

command -v cloudflared >/dev/null || {
  echo "cloudflared no instalado: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/" >&2
  exit 1
}

export PATH="$HOME/.local/go/bin:$PATH"
command -v go >/dev/null || {
  echo "go no está en el PATH (vive en ~/.local/go)" >&2
  exit 1
}

WEB_PORT=5173
API_PORT=8080
DB="$PWD/api/speako.dev.db"

# El túnel apunta a :$WEB_PORT y Vite proxya a :$API_PORT; si cualquiera está
# ocupado, el proceso equivocado termina expuesto a Internet.
for PORT in "$WEB_PORT" "$API_PORT"; do
  if ss -ltn "sport = :$PORT" | grep -q LISTEN; then
    echo "==> puerto $PORT ocupado; libéralo antes de arrancar el túnel" >&2
    exit 1
  fi
done

# Idempotente y no destructivo. Para una base limpia con profesora, horarios y
# productos: `make seed` (eso sí borra la base).
(cd api && SPEAKO_DB_PATH="$DB" go run ./cmd/speako-cli migrate)

LOG=$(mktemp)
cloudflared tunnel --url "http://localhost:$WEB_PORT" >"$LOG" 2>&1 &
TUNNEL_PID=$!
API_PID=""
trap 'kill "$TUNNEL_PID" ${API_PID:+"$API_PID"} 2>/dev/null' EXIT

URL=""
for _ in $(seq 1 30); do
  URL=$(grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' "$LOG" | head -1)
  [ -n "$URL" ] && break
  sleep 1
done

if [ -z "$URL" ]; then
  echo "==> No se obtuvo URL del túnel; log en $LOG" >&2
  exit 1
fi

# La API tiene que arrancar sabiendo la URL pública: SPEAKO_PUBLIC_URL es la
# base de los links mágicos. Con el default (localhost:5173) quien revisa desde
# afuera recibe un link que sólo abre en esta máquina.
(cd api && SPEAKO_DB_PATH="$DB" SPEAKO_PUBLIC_URL="$URL" go run ./cmd/api) &
API_PID=$!

echo ""
echo "==> URL pública: $URL"
echo "==> Link de acceso para la profesora:"
echo "    (cd api && SPEAKO_DB_PATH=$DB SPEAKO_PUBLIC_URL=$URL go run ./cmd/speako-cli login-link professora@speako.com.br)"
echo ""

cd web && npm run dev -- --port "$WEB_PORT"
