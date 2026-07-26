#
# shortlink-api: pure-Go binary (no cgo), distroless runtime.
#
#   docker build -f docker/api.Dockerfile -t shortlink-api .
#
# Build context MUST be the repo root.
ARG GO_VERSION=1.26

# ---- build ----
FROM golang:${GO_VERSION}-trixie AS build
WORKDIR /src
# Manifests first → module download layer is cached until go.mod/sum change.
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/app ./cmd/server

# ---- run ----
# distroless/static: ~2MB base, no shell, nonroot. Bundles ca-certificates
# (outbound HTTPS: OIDC discovery/JWKS/token endpoint) + tzdata.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
