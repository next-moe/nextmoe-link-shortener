package httpapi

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/next-moe/nextmoe-link-shortener/apps/api/internal/engine"
)

// RegisterRedirect mounts GET /s/{alias} as a plain Fiber route — it is a
// browser-facing 302, not a JSON operation, so it lives outside the Huma
// layer (and outside the OpenAPI spec on purpose: the redirect is the
// product, not the API).
//
// trustProxyHeader makes CF-Connecting-IP authoritative for the client IP.
// True ONLY behind Cloudflare/Traefik in prod — the header is spoofable when
// the service is reached directly.
func RegisterRedirect(app *fiber.App, eng *engine.Engine, trustProxyHeader bool) {
	app.Get("/s/:alias", func(c fiber.Ctx) error {
		alias := c.Params("alias")
		dest, err := eng.ResolveRedirect(alias, string(c.RequestCtx().URI().QueryString()), engine.VisitMeta{
			IP:        clientIP(c, trustProxyHeader),
			UserAgent: c.Get("User-Agent"),
			Referer:   c.Get("Referer"),
		})
		if err != nil {
			switch {
			case errors.Is(err, engine.ErrNotFound):
				return c.Status(fiber.StatusNotFound).SendString("short link not found")
			case errors.Is(err, engine.ErrLinkGone):
				return c.Status(fiber.StatusGone).SendString("short link disabled, expired, or over its visit limit")
			default:
				slog.Error("redirect", "alias", alias, "error", err)
				return c.Status(fiber.StatusInternalServerError).SendString("internal error")
			}
		}
		return c.Redirect().Status(fiber.StatusFound).To(dest)
	})
}

// clientIP resolves the caller's IP. Behind the edge (prod) CF-Connecting-IP
// is authoritative; everywhere else the transport address is the only value
// that cannot be spoofed.
func clientIP(c fiber.Ctx, trustProxyHeader bool) string {
	if trustProxyHeader {
		if ip := c.Get("CF-Connecting-IP"); ip != "" {
			return ip
		}
	}
	return c.IP()
}
