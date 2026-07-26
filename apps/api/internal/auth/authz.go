package auth

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

// RequireRole returns the caller's identity when it holds at least one of the
// wanted roles; 401 when anonymous, 403 otherwise. Every dashboard handler
// calls this first — the dashboard has no non-admin surface.
func RequireRole(ctx context.Context, wanted []string) (*Identity, error) {
	id, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	if !id.HasAnyRole(wanted) {
		return nil, huma.Error403Forbidden("this dashboard is admin-only")
	}
	return id, nil
}
