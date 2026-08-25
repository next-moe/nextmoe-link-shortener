#!/usr/bin/env bash
# Register this site's public local-dev OAuth client in kun_galgame_infra.
# Secret contract (infra docs/dev-environment.md 裁定 3): plaintext
# `dev-secret-<client_id>` hashed as sha256:<hex>.
set -euo pipefail

client_id='shortlink-dev'
plaintext="dev-secret-${client_id}"
hash="$(printf %s "$plaintext" | sha256sum | cut -d' ' -f1)"

export PGHOST="${PGHOST:-127.0.0.1}"
export PGPORT="${PGPORT:-5432}"
export PGUSER="${PGUSER:-postgres}"
export PGDATABASE="${PGDATABASE:-kun_galgame_infra}"
export PGPASSWORD="${PGPASSWORD:-191007}"

psql -v ON_ERROR_STOP=1 <<SQL
INSERT INTO oauth_clients
  (id, name, secret, redirect_uris, grants, is_public, auto_consent,
   refresh_token_ttl_seconds, allowed_scopes,
   image_enabled, listed,
   dev_enabled, dev_tier, dev_nsfw_allowed, dev_rate_per_min, dev_quota_daily,
   dev_review_status)
VALUES
  ('${client_id}', 'KunGal Link Shortener (dev)', 'sha256:${hash}',
   '["http://127.0.0.1:7844/auth/callback"]',
   '["authorization_code","refresh_token"]',
   false, true, 7776000, '["openid","profile","email"]',
   false, false,
   false, '', false, 0, 0,
   'approved')
ON CONFLICT (id) DO UPDATE SET
  secret = EXCLUDED.secret,
  redirect_uris = EXCLUDED.redirect_uris,
  grants = EXCLUDED.grants,
  is_public = EXCLUDED.is_public,
  auto_consent = EXCLUDED.auto_consent,
  allowed_scopes = EXCLUDED.allowed_scopes,
  dev_review_status = EXCLUDED.dev_review_status;
SQL

echo "registered oauth client ${client_id} (secret ${plaintext})"
