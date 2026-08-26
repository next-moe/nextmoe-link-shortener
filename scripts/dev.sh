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

# Reclaim the app ports before starting. A previous run can leave a listener
# behind — SIGKILL skips the graceful shutdown, and any process killed while
# holding the socket keeps it — and the next start then dies with
# "address already in use". Only 7844/7845 are touched; Postgres and Redis
# live in Docker and are managed by compose.
reclaim_port() {
  local port="$1" label="$2" pids pid
  pids="$(ss -tlnpH "sport = :$port" 2>/dev/null | grep -oP 'pid=\K[0-9]+' | sort -u || true)"
  [[ -n "$pids" ]] || return 0

  for pid in $pids; do
    echo "port $port ($label) is held by pid $pid — $(ps -o args= -p "$pid" 2>/dev/null | cut -c1-80)"
    kill -TERM "$pid" 2>/dev/null || true
  done

  # Give the graceful path a moment, then insist.
  for _ in $(seq 1 20); do
    ss -tlnH "sport = :$port" 2>/dev/null | grep -q . || return 0
    sleep 0.25
  done
  for pid in $pids; do
    kill -KILL "$pid" 2>/dev/null || true
  done
  sleep 0.5
}

if command -v ss >/dev/null 2>&1; then
  reclaim_port 7845 api
  reclaim_port 7844 web
fi

echo "starting backing services (Postgres :7846, Redis :7847)..."
docker compose --env-file .env -f docker/compose.dev.yml up -d --wait

if [[ "$services_only" -eq 1 ]]; then
  echo "backing services are up"
  exit 0
fi

exec pnpm -F "./apps/**" --parallel --stream run dev
