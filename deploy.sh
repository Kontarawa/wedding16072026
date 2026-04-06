#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker not found" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Docker compose not found" >&2
  exit 1
fi

docker compose up -d --build
docker compose ps
