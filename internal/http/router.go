package httpx

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	AuthHandlers        *AuthHandlers
	AdminHandlers       *AdminHandlers
	AppHandlers         *AppHandlers
	AdminAllowedOrigins []string
	AppAllowedOrigins   []string
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Group(func(r chi.Router) {
		r.Use(deps.CORS(appendOrigins(deps.AdminAllowedOrigins, deps.AppAllowedOrigins)))
		r.Options("/api/session", noContent)
		r.Get("/api/session", deps.AuthHandlers.Session)
		r.Options("/api/auth/password", noContent)
		r.Post("/api/auth/password", deps.AuthHandlers.PasswordLogin)
		r.Options("/api/auth/google/start", noContent)
		r.Get("/api/auth/google/start", deps.AuthHandlers.GoogleStart)
		r.Options("/api/auth/google/callback", noContent)
		r.Get("/api/auth/google/callback", deps.AuthHandlers.GoogleCallback)
		r.Options("/api/auth/logout", noContent)
		r.Post("/api/auth/logout", deps.AuthHandlers.Logout)
	})
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(deps.CORS(deps.AdminAllowedOrigins))
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
	r.Route("/api/app", func(r chi.Router) {
		r.Use(deps.CORS(deps.AppAllowedOrigins))
		r.Use(deps.RequireSession)
		r.Use(deps.RequireAppUser)
		r.Get("/session", deps.AppHandlers.Session)
		r.Get("/locations", deps.AppHandlers.ListLocations)
		r.Get("/species", deps.AppHandlers.ListSpecies)
		r.Get("/observations", deps.AppHandlers.ListObservations)
		r.Post("/observations", deps.AppHandlers.CreateObservation)
		r.Patch("/observations/{id}", deps.AppHandlers.UpdateObservation)
		r.Delete("/observations/{id}", deps.AppHandlers.DeleteObservation)
		r.Get("/reef-check/config", deps.AppHandlers.ReefCheckConfig)
		r.Get("/reef-check/surveys", deps.AppHandlers.ListReefCheckSurveys)
		r.Post("/reef-check/surveys", deps.AppHandlers.CreateReefCheckSurvey)
		r.Get("/reef-check/surveys/{id}", deps.AppHandlers.GetReefCheckSurvey)
		r.Put("/reef-check/surveys/{id}", deps.AppHandlers.UpdateReefCheckSurvey)
		r.Delete("/reef-check/surveys/{id}", deps.AppHandlers.DeleteReefCheckSurvey)
		r.Get("/reef-check/surveys/{id}/report", deps.AppHandlers.ReefCheckReport)
	})
	r.Handle("/public/*", http.StripPrefix("/public/", http.FileServer(http.Dir(staticDir("web/public")))))
	adminFiles := http.FileServer(http.Dir(staticDir("web/admin")))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		adminFiles.ServeHTTP(w, r)
	})
	return r
}

func noContent(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func appendOrigins(left, right []string) []string {
	merged := make([]string, 0, len(left)+len(right))
	merged = append(merged, left...)
	merged = append(merged, right...)
	return merged
}

func staticDir(path string) string {
	candidates := []string{
		path,
		filepath.Join("..", path),
		filepath.Join("..", "..", path),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return path
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
