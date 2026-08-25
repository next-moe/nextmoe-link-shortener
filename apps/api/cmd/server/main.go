// Command server runs the shortlink HTTP API (Go 1.26 · Fiber v3 · Huma):
// the /s/{alias} redirect, the admin dashboard BFF, and the S2S surface.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/auth"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/config"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/db"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/engine"
	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Fail loud: a misconfigured process must not start half-up.
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	initLogger(cfg.Mode)

	// Connect Postgres up front and fail loud — no degraded, database-less
	// mode. AutoMigrate runs inside Open.
	database, err := db.Open(cfg.DBDSN)
	if err != nil {
		slog.Error("cannot connect to postgres", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to postgres (schema migrated)")
	defer func() { _ = database.Close() }()

	eng := engine.New(database.Gorm)

	rdb := dialRedis(context.Background(), cfg)
	if rdb != nil {
		defer func() { _ = rdb.Close() }()
	}

	resolver, authBackend, err := buildAuth(context.Background(), cfg, rdb)
	if err != nil {
		slog.Error("auth initialization failed", "error", err)
		os.Exit(1)
	}

	app := fiber.New(fiber.Config{
		AppName:      "shortlink-api",
		ServerHeader: "shortlink-api",
	})
	// The redirect route registers BEFORE the Huma catch-all so /s/{alias}
	// stays a plain Fiber 302 outside the JSON surface.
	httpapi.RegisterRedirect(app, eng, cfg.Mode == "prod")
	httpapi.NewAPI(app, httpapi.Deps{
		Engine:        eng,
		Resolver:      resolver,
		Auth:          authBackend,
		AdminRoles:    cfg.AdminRoles,
		PublicBaseURL: cfg.PublicBaseURL,
	})

	if err := run(app, cfg); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

// dialRedis connects the session store's Redis. Fatal when unreachable in
// prod (config guarantees the address there — a prod process must not start
// without sessions); nil with a loud warning in dev, where auth degrades to
// the Dev header backdoor.
func dialRedis(ctx context.Context, cfg config.Config) *redis.Client {
	if cfg.RedisAddr == "" {
		return nil // only reachable in dev (config.Load enforces prod)
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		if cfg.Mode == "prod" {
			slog.Error("cannot connect to redis", "addr", cfg.RedisAddr, "error", err)
			os.Exit(1)
		}
		slog.Warn("dev mode: redis unreachable — sessions are disabled", "addr", cfg.RedisAddr, "error", err)
		return nil
	}
	slog.Info("connected to redis")
	return rdb
}

// buildAuth wires the identity resolver and, when OIDC is configured, the
// session BFF backend.
//
//   - prod: config guarantees OIDC + Redis, and any discovery failure is
//     fatal — a prod process must not start half-authenticated. The dev
//     header backdoor cannot exist there.
//   - dev with OIDC configured and reachable: SessionResolver chained ahead
//     of DevResolver (a real session wins; the dev header stays available).
//   - dev with OIDC configured but the IdP/Redis UNREACHABLE: degrade to the
//     DevResolver with a loud warning instead of refusing to boot; the /auth
//     endpoints return 503.
//   - dev without OIDC: only the DevResolver (a warning is logged).
func buildAuth(ctx context.Context, cfg config.Config, rdb *redis.Client) (auth.Resolver, *httpapi.AuthBackend, error) {
	if !cfg.OIDC.Configured() || rdb == nil {
		// Only reachable in dev (prod is enforced by config.Load + dialRedis).
		slog.Warn("dev mode: OIDC/Redis not available — only the 'Authorization: Dev <user_id> [roles]' backdoor is active; this must never run in prod")
		return auth.DevResolver{}, nil, nil
	}

	provider, err := auth.Discover(ctx, nil, cfg.OIDC.Issuer)
	if err != nil {
		if cfg.Mode == "prod" {
			return nil, nil, fmt.Errorf("oidc discovery: %w", err)
		}
		slog.Warn("dev mode: oidc discovery failed — degrading to the 'Authorization: Dev <user_id> [roles]' backdoor; /auth endpoints will return 503", "error", err)
		return auth.DevResolver{}, nil, nil
	}
	slog.Info("oidc discovery ok", "issuer", provider.Issuer)

	verifier := auth.NewVerifier(auth.NewJWKSResolver(provider.JWKSURI), cfg.OIDC.Issuer, cfg.OIDC.ClientID)
	store := auth.NewSessionStore(rdb, verifier)
	oauth := auth.NewOAuthClient(provider.TokenEndpoint, cfg.OIDC.ClientID, cfg.OIDC.ClientSecret, cfg.OIDC.RedirectURI)
	sessionResolver := auth.NewSessionResolver(store, oauth)

	var resolver auth.Resolver = sessionResolver
	if cfg.Mode == "dev" {
		slog.Warn("dev mode: 'Authorization: Dev <user_id> [roles]' backdoor is chained behind the session resolver; this must never run in prod")
		resolver = auth.ChainResolver{Resolvers: []auth.Resolver{sessionResolver, auth.DevResolver{}}}
	}

	backend := &httpapi.AuthBackend{
		OAuth:        oauth,
		Store:        store,
		Provider:     provider,
		ClientID:     cfg.OIDC.ClientID,
		RedirectURI:  cfg.OIDC.RedirectURI,
		Scopes:       "openid profile",
		CookieSecure: cfg.Mode == "prod",
	}
	return resolver, backend, nil
}

// run serves until an interrupt/termination signal, then shuts down
// gracefully. A bind failure returns immediately rather than leaving a
// process that is alive but not serving.
func run(app *fiber.App, cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "addr", addr, "mode", cfg.Mode)
		if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.Info("server stopped")
	return nil
}

// initLogger configures the global slog logger from the runtime mode:
// structured JSON in prod, human-readable text in dev.
func initLogger(mode string) {
	var h slog.Handler
	if mode == "prod" {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	slog.SetDefault(slog.New(h))
}
