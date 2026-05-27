package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	AuthHandlers *AuthHandlers
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/api/session", deps.AuthHandlers.Session)
	r.Post("/api/auth/password", deps.AuthHandlers.PasswordLogin)
	r.Get("/api/auth/google/start", deps.AuthHandlers.GoogleStart)
	r.Get("/api/auth/google/callback", deps.AuthHandlers.GoogleCallback)
	r.Post("/api/auth/logout", deps.AuthHandlers.Logout)
	return r
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
