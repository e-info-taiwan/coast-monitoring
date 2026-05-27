package httpx

import (
	"context"
	"net/http"
	"strings"

	"coast-monitoring/internal/policy"
)

type contextKey string

const currentUserKey contextKey = "currentUser"

func withCurrentUser(ctx context.Context, user policy.User) context.Context {
	return context.WithValue(ctx, currentUserKey, user)
}

func currentUser(r *http.Request) (policy.User, bool) {
	user, ok := r.Context().Value(currentUserKey).(policy.User)
	return user, ok
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
		next.ServeHTTP(w, r.WithContext(withCurrentUser(r.Context(), user)))
	})
}
