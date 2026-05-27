package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	AuthHandlers  *AuthHandlers
	AdminHandlers *AdminHandlers
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
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(deps.RequireSession)
		r.Use(deps.RequireAdmin)
		r.Get("/users", deps.AdminHandlers.ListUsers)
		r.Post("/users", deps.AdminHandlers.CreateUser)
		r.Patch("/users/{id}", deps.AdminHandlers.UpdateUser)
		r.Delete("/users/{id}", deps.AdminHandlers.DisableUser)
		r.Get("/locations", deps.AdminHandlers.ListLocations)
		r.Post("/locations", deps.AdminHandlers.CreateLocation)
		r.Patch("/locations/{id}", deps.AdminHandlers.UpdateLocation)
		r.Delete("/locations/{id}", deps.AdminHandlers.DeleteLocation)
		r.Get("/species", deps.AdminHandlers.ListSpecies)
		r.Post("/species", deps.AdminHandlers.CreateSpecies)
		r.Patch("/species/{id}", deps.AdminHandlers.UpdateSpecies)
		r.Delete("/species/{id}", deps.AdminHandlers.DeleteSpecies)
		r.Get("/observations", deps.AdminHandlers.ListObservations)
		r.Patch("/observations/{id}", deps.AdminHandlers.UpdateObservation)
		r.Delete("/observations/{id}", deps.AdminHandlers.DeleteObservation)
		r.Get("/audit-logs", deps.AdminHandlers.ListAuditLogs)
	})
	return r
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
