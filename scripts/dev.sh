#!/usr/bin/env bash
# Local-dev bootstrap: ensure .env exists, bring up Postgres/Redis, then start
# api + web. `pnpm dev` is the intended entry point.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

services_only=0
if [[ "${1:-}" == "--services-only" ]]; then
  services_only=1
fi

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "created .env from .env.example (edit SHORTLINK_OIDC_* if you have a local IdP)"
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker is required to start Postgres/Redis (see docker/compose.dev.yml)" >&2
  exit 1
fi

echo "starting backing services (Postgres :7846, Redis :7847)..."
docker compose --env-file .env -f docker/compose.dev.yml up -d --wait

if [[ "$services_only" -eq 1 ]]; then
  echo "backing services are up"
  exit 0
fi

exec pnpm -F "./apps/**" --parallel --stream run dev
