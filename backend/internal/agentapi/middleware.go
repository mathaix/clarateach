package agentapi

import (
	"net/http"
	"strings"
)

// authMiddleware validates the Bearer token for authenticated endpoints.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If no token is configured, skip authentication (for local dev)
		if s.agentToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeError(w, http.StatusUnauthorized, "missing_auth", "Authorization header required")
			return
		}

		// Expect "Bearer <token>" format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			s.writeError(w, http.StatusUnauthorized, "invalid_auth", "Invalid authorization format, expected 'Bearer <token>'")
			return
		}

		token := parts[1]
		if token != s.agentToken {
			s.logger.Warnf("Invalid token attempt from %s", r.RemoteAddr)
			s.writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}
