#
# shortlink-web: the apps/web Nuxt 4 frontend (Nitro node-server preset).
#
#   docker build -f docker/web.Dockerfile -t shortlink-web .
#
# Build context MUST be the repo root: the pnpm workspace install needs the
# lockfile + every workspace manifest. @kungal/ui-* comes from npm.
#
# Nitro route rules (the /api/** and /s/** proxy) are BAKED at build time from
# SHORTLINK_API_PROXY_TARGET. The default targets the compose service name
# `api` (docker/compose.dokploy.yml); override the build arg only when the
# API is reachable under a different name.
ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-trixie-slim AS base
RUN corepack enable
WORKDIR /repo

# ---- deps: copy every workspace manifest, install only the web subgraph ----
FROM base AS deps
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/web/package.json apps/web/package.json
COPY apps/api/package.json apps/api/package.json
# --ignore-scripts: `postinstall: nuxt prepare` can't run here (app source
# isn't copied yet); the later `nuxt build` runs prepare itself.
RUN pnpm install --frozen-lockfile --ignore-scripts --filter "@shortlink/web..."

# ---- build ----
FROM deps AS build
ARG API_PROXY_TARGET=http://api:7845
ENV SHORTLINK_API_PROXY_TARGET=${API_PROXY_TARGET}
COPY apps/web apps/web
RUN pnpm --filter "@shortlink/web" run build

# ---- run: just Node + the self-contained .output (no pnpm, no sources) ----
FROM node:${NODE_VERSION}-trixie-slim AS run
ENV NODE_ENV=production HOST=0.0.0.0 NITRO_PORT=3000
WORKDIR /app
COPY --from=build /repo/apps/web/.output ./.output
USER node
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
