# Project Guidelines

## 铁律 (Iron Rules — non-negotiable)

1. **No background gradients in any UI, ever.** Never use gradient backgrounds (`bg-gradient-*`, `linear-gradient()`, etc.); use solid colors from the KunUI palette.
2. **Prefer KunUI components; do not modify KunUI itself.** Reach for a `<Kun*>` component (`@kungal/ui-*`) first — do not hand-roll a custom component unless there is genuinely no KunUI equivalent. If KunUI appears to have a bug or a missing feature, report it to the user instead of patching around it.
3. **Zero secrets in the repo.** `.env.example` carries placeholder values only; real credentials live in the git-ignored `.env`.

## What this is

The NextMoe ecosystem's shared **short-link service** (`s.kungal.com` class):
a small Go API + Nuxt dashboard. Sibling products (kungal / moyu / letmoe …)
create short links over an S2S API; humans manage links in an admin-only
dashboard behind ecosystem OIDC login.

## Architecture (standard ecosystem site shape, modeled on kun-softmoe)

```
apps/api   Go 1.26 · Fiber v3 · Huma v2 (code-first OpenAPI 3.1) · GORM · Postgres (kun_shortlink) · Redis
apps/web   Nuxt 4 · @kungal/ui-nuxt layer · Tailwind v4 · same-origin /api proxy to the Go API
```

- The Go API is the **OIDC RP and BFF**: PKCE authorization-code flow against
  the NextMoe IdP (endpoints via discovery, never hardcoded), tokens live only
  in Redis server-side sessions (`shortlink_session` httpOnly cookie — the
  browser never sees a token), access tokens verified via JWKS (ES256/RS256,
  fail-closed).
- **Dashboard access is role-gated**: JWT roles claim ∩ `SHORTLINK_ADMIN_ROLES`
  (default `admin`). There is no self-serve signup.
- **S2S surface** (`/s2s/*`): sibling products authenticate with a Bearer API
  key (`slk_` prefix; SHA-256 hash at rest, shown once at creation). Keys are
  minted in the dashboard.
- **Redirect route** `/s/{alias}` is served by the Go API directly (plain
  Fiber, before the Huma layer). Every hit is recorded transactionally:
  visit row + hourly bucket + counters.
- The web app proxies `/api/**` and `/s/**` to the Go API via Nitro route
  rules, so prod and dev share one same-origin shape (zero CORS).

## Commands

- `pnpm dev` — run API (:7845) + web (:7844) together; backing services come
  from `docker compose -f docker/compose.dev.yml up -d` (Postgres :7846,
  Redis :7847).
- `pnpm verify` — lint + test + regenerate OpenAPI spec & TS types + fail on
  drift. Run before committing API-surface changes.
- `pnpm gen` — regenerate `apps/api/openapi/openapi.yaml` (from code) and
  `apps/web/shared/types/api.d.ts` (from the spec). The spec is code-first:
  never hand-edit it.
- DB schema: GORM `AutoMigrate` runs at API startup (image/artifact-service
  precedent) — no separate migrate step.

## Conventions

1. All commit messages and code comments in English.
2. Frontend functions are arrow functions; compose class names with `cn` where practical.
3. `pages/` contain only `definePageMeta` + a single container component; the
   business components live in `components/<page>/` (Nuxt auto-import prefixes
   the directory: `components/dash/Container.vue` → `<DashContainer>`).
4. Constants in `app/constants/`, shared types in `shared/types/`.
5. Use KunUI semantic colors (`text-default-500`, `bg-primary-100`, `text-danger-600`, …), never Tailwind's raw palette; colors adapt to dark mode without `dark:` prefixes.
6. A Nuxt page must have a single real root element (no root comments, no `display: contents`).
7. Keep frontend and backend response shapes in sync — regenerate types (`pnpm gen`) whenever the API surface changes.
