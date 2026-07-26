// Package config loads the shortlink API runtime configuration from
// SHORTLINK_-prefixed environment variables. Secrets never live in the repo —
// the shape is documented by the repo-root .env.example.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	Host  string // bind host
	Port  string // bind port
	Mode  string // "dev" | "prod" — selects the slog handler and auth strictness
	DBDSN string // Postgres DSN (required; there is no database-less mode)
	// PublicBaseURL is the origin short links are advertised under
	// (dashboard + S2S responses build "<base>/s/<alias>" from it).
	PublicBaseURL string
	// AdminRoles are the ecosystem roles allowed into the dashboard. The JWT
	// roles claim (global ∪ site roles) is matched against this list.
	AdminRoles []string
	OIDC       OIDCConfig // NextMoe IdP RP settings
	RedisAddr  string     // Redis address for BFF sessions
}

// OIDCConfig is the standard-RP configuration. Endpoints are NOT here — they
// come from the issuer's discovery document at startup. Only the issuer root
// and the client credentials are configured.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured reports whether every field needed to run the OIDC RP is present.
func (o OIDCConfig) Configured() bool {
	return o.Issuer != "" && o.ClientID != "" && o.ClientSecret != "" && o.RedirectURI != ""
}

// Load reads and validates the environment. A missing database DSN is always
// fatal. In prod the OIDC RP config and Redis address are also required — a
// prod process must not start half-authenticated. In dev they are optional:
// without them the server runs with only the Dev header resolver.
func Load() (Config, error) {
	loadDotEnv()

	cfg := Config{
		Host:          getenv("SHORTLINK_HOST", "0.0.0.0"),
		Port:          getenv("SHORTLINK_PORT", "7845"),
		Mode:          getenv("SHORTLINK_MODE", "dev"),
		DBDSN:         os.Getenv("SHORTLINK_DB_DSN"),
		PublicBaseURL: strings.TrimRight(getenv("SHORTLINK_PUBLIC_BASE_URL", "http://127.0.0.1:7844"), "/"),
		AdminRoles:    splitCSV(getenv("SHORTLINK_ADMIN_ROLES", "admin")),
		OIDC: OIDCConfig{
			Issuer:       os.Getenv("SHORTLINK_OIDC_ISSUER"),
			ClientID:     os.Getenv("SHORTLINK_OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("SHORTLINK_OIDC_CLIENT_SECRET"),
			RedirectURI:  os.Getenv("SHORTLINK_OIDC_REDIRECT_URI"),
		},
		RedisAddr: os.Getenv("SHORTLINK_REDIS_ADDR"),
	}
	if cfg.DBDSN == "" {
		return Config{}, fmt.Errorf("SHORTLINK_DB_DSN is required")
	}
	if len(cfg.AdminRoles) == 0 {
		return Config{}, fmt.Errorf("SHORTLINK_ADMIN_ROLES must not be empty")
	}
	if cfg.Mode == "prod" {
		if !cfg.OIDC.Configured() {
			return Config{}, fmt.Errorf("prod mode requires SHORTLINK_OIDC_{ISSUER,CLIENT_ID,CLIENT_SECRET,REDIRECT_URI}")
		}
		if cfg.RedisAddr == "" {
			return Config{}, fmt.Errorf("prod mode requires SHORTLINK_REDIS_ADDR")
		}
	}
	return cfg, nil
}

// loadDotEnv best-effort loads the nearest .env walking up from the working
// directory, so `go run ./cmd/...` from anywhere in the tree picks up the
// repo-root .env. The walk stops at the repository boundary (the first
// directory containing .git). In containers and CI the environment is injected
// directly and no .env exists — that absence is expected, not an error.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		p := filepath.Join(dir, ".env")
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			return
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return // repository root reached; do not walk past it
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return // reached the filesystem root
		}
		dir = parent
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitCSV splits a comma-separated list, trimming blanks and dropping empties.
func splitCSV(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
