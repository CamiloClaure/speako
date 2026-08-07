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

DB="$PWD/api/speako.dev.db"

# Primer puerto libre desde $1, saltando el que ya elegimos ($2, opcional).
# Nunca reusamos un puerto ocupado: el túnel serviría el proceso equivocado y
# el proxy de Vite hablaría con la API de otra persona.
pick_port() {
  p=$1
  while [ "$p" = "$2" ] || ss -ltn "sport = :$p" | grep -q LISTEN; do
    p=$((p + 1))
    [ "$p" -gt $(($1 + 50)) ] && {
      echo "==> no hay puerto libre entre $1 y $p" >&2
      exit 1
    }
  done
  echo "$p"
}

WEB_PORT=$(pick_port 5173)
API_PORT=$(pick_port 8080 "$WEB_PORT")

[ "$WEB_PORT" = 5173 ] || echo "==> 5173 ocupado, SPA en $WEB_PORT"
[ "$API_PORT" = 8080 ] || echo "==> 8080 ocupado, API en $API_PORT"

# Vite lee ambos: sirve en WEB_PORT con strictPort y proxya /api a API_PORT.
export SPEAKO_WEB_PORT="$WEB_PORT" SPEAKO_API_PORT="$API_PORT"

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
(cd api && SPEAKO_DB_PATH="$DB" SPEAKO_PUBLIC_URL="$URL" SPEAKO_ADDR=":$API_PORT" go run ./cmd/api) &
API_PID=$!

echo ""
echo "==> URL pública: $URL"
echo "==> Link de acceso para la profesora:"
echo "    (cd api && SPEAKO_DB_PATH=$DB SPEAKO_PUBLIC_URL=$URL go run ./cmd/speako-cli login-link professora@speako.com.br)"
echo ""

# Sin --port: vite.config.ts ya lee SPEAKO_WEB_PORT, y una sola fuente evita
# que la bandera y el config discrepen.
cd web && npm run dev
