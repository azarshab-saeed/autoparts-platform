package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/example/autoparts-core/internal/platform/api"
)

type contextKey string

const claimsKey contextKey = "auth_claims"

type Verifier interface {
	VerifyToken(context.Context, string) (Claims, error)
}

func Middleware(verifier Verifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			api.WriteError(w, api.Unauthorized("missing_token", "authentication required"))
			return
		}
		claims, err := verifier.VerifyToken(r.Context(), strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")))
		if err != nil {
			api.WriteError(w, api.Unauthorized("invalid_token", "invalid or expired access token"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}

func ClaimsFrom(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := ClaimsFrom(r.Context())
			if !ok {
				api.WriteError(w, api.Unauthorized("missing_context", "authentication required"))
				return
			}
			if allowed[c.Role] {
				next.ServeHTTP(w, r)
				return
			}
			for _, role := range c.Roles {
				if allowed[role] {
					next.ServeHTTP(w, r)
					return
				}
			}
			api.WriteError(w, api.Forbidden("insufficient_role", "role is not allowed for this operation"))
		})
	}
}
