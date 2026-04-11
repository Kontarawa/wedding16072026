#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

IMAGE_NAME="${IMAGE_NAME:-wedding-invitation:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-wedding-invitation}"
PORT="${PORT:-8080}"

mkdir -p data
if [[ ! -f data/guests.json ]]; then
  echo '{}' > data/guests.json
fi

echo "Building ${IMAGE_NAME}..."
docker build -t "${IMAGE_NAME}" .

if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}\$"; then
  echo "Removing existing container ${CONTAINER_NAME}..."
  docker rm -f "${CONTAINER_NAME}" >/dev/null
fi

echo "Starting ${CONTAINER_NAME} on port ${PORT}..."
docker run -d \
  --name "${CONTAINER_NAME}" \
  --restart unless-stopped \
  -p "${PORT}:8080" \
  -e LISTEN_ADDR=":8080" \
  -e DATA_DIR=/app/data \
  -e GUEST_DB=/app/data/guests.json \
  -e GOOGLE_SHEETS_WEBAPP_URL="${GOOGLE_SHEETS_WEBAPP_URL:-}" \
  -e ADMIN_TOKEN="${ADMIN_TOKEN:-}" \
  -e TLS_CERT_FILE="${TLS_CERT_FILE:-}" \
  -e TLS_KEY_FILE="${TLS_KEY_FILE:-}" \
  -v "$ROOT/data:/app/data" \
  "${IMAGE_NAME}"

echo "Done. Open http://127.0.0.1:${PORT}/wedding/invitation/"
echo "Production HTTPS: обычно ставят Caddy или nginx перед контейнером; либо передайте TLS_CERT_FILE и TLS_KEY_FILE (пути к PEM внутри образа) и пробросьте том с сертификатами."
