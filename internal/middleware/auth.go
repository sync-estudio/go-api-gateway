package middleware

import (
	"net/http"
	"strings"

	"sync.gateway/internal/auth"
	"sync.gateway/internal/service"
)

func NewAuthMiddleware(validator *auth.Validator, registry *service.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Preflight requests should pass through without auth checks.
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Check if this route requires authentication
			svc := registry.FindServiceForPath(r.URL.Path)
			if svc == nil || !svc.Auth.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				auth.WriteError(w, http.StatusUnauthorized, auth.ErrMissingAuthHeader)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				auth.WriteError(w, http.StatusUnauthorized, auth.ErrInvalidAuthFormat)
				return
			}
			token := parts[1]

			identity, err := validator.Validate(token)
			if err != nil {
				auth.WriteError(w, http.StatusUnauthorized, err.Error())
				return
			}

			if identity.UserID != "" {
				r.Header.Set("X-User-Id", identity.UserID)
			}
			if identity.Email != "" {
				r.Header.Set("X-User-Email", identity.Email)
			}
			if len(identity.Roles) > 0 {
				r.Header.Set("X-User-Roles", strings.Join(identity.Roles, ","))
			}

			ctx := auth.ToContext(r.Context(), identity)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
