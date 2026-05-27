package httpx

import (
	"context"
	"net/http"
	"strings"

	"coast-monitoring/internal/policy"
)

type contextKey string

const currentUserKey contextKey = "currentUser"
const csrfTokenKey contextKey = "csrfToken"

func withCurrentUser(ctx context.Context, user policy.User) context.Context {
	return context.WithValue(ctx, currentUserKey, user)
}

func withCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfTokenKey, token)
}

func currentUser(r *http.Request) (policy.User, bool) {
	user, ok := r.Context().Value(currentUserKey).(policy.User)
	return user, ok
}

func currentCSRFToken(r *http.Request) string {
	token, _ := r.Context().Value(csrfTokenKey).(string)
	return token
}

func (h *AuthHandlers) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.Auth == nil {
			writeError(w, http.StatusServiceUnavailable, "auth is not configured")
			return
		}
		cfg := h.config()
		sessionToken, ok := cookieValue(r, cfg.SessionCookieName)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		csrfToken := strings.TrimSpace(r.Header.Get(cfg.CSRFHeaderName))
		if csrfToken == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, err := h.Auth.AuthenticateSession(r.Context(), sessionToken, csrfToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := withCurrentUser(r.Context(), user)
		ctx = withCSRFToken(ctx, csrfToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (deps Dependencies) RequireSession(next http.Handler) http.Handler {
	if deps.AuthHandlers == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusServiceUnavailable, "auth is not configured")
		})
	}
	return deps.AuthHandlers.RequireSession(next)
}

func (deps Dependencies) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !policy.CanUseAdminAPI(user) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (deps Dependencies) RequireAppUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !policy.CanUseAppAPI(user) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (deps Dependencies) CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := originSet(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" && allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", deps.allowedCORSHeaders())
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				if origin != "" && !allowed[origin] {
					writeError(w, http.StatusForbidden, "origin is not allowed")
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (deps Dependencies) allowedCORSHeaders() string {
	header := defaultCSRFHeaderName
	if deps.AuthHandlers != nil {
		header = deps.AuthHandlers.config().CSRFHeaderName
	}
	return "Accept, Content-Type, " + header
}

func originSet(origins []string) map[string]bool {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = true
		}
	}
	return allowed
}
