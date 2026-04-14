#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

echo "Starting via docker compose (HTTPS only)..."
docker compose up -d --build

echo "Done. HTTPS: https://localhost:8443/wedding/invitation/"
