package httpx

import (
	"context"
	"net/http"
	"strings"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AppHandlers struct {
	Catalog      AppCatalogService
	Observations AppObservationService
	Mutations    AdminMutationRunner
}

type AppCatalogService interface {
	ListLocations(ctx context.Context, actor policy.User) ([]service.Location, error)
	ListSpecies(ctx context.Context, actor policy.User) ([]service.Species, error)
}

type AppObservationService interface {
	ListForApp(ctx context.Context, actor policy.User) ([]service.Observation, error)
	Create(ctx context.Context, actor policy.User, input service.ObservationInput) (service.Observation, error)
	Update(ctx context.Context, actor policy.User, id uuid.UUID, input service.ObservationInput) (service.Observation, error)
	Delete(ctx context.Context, actor policy.User, id uuid.UUID) error
}

func (h *AppHandlers) Session(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAppHandlerService(w, r, true)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, SessionResponse{
		Authenticated: true,
		User:          currentUserResponse(actor),
		CSRFToken:     currentCSRFToken(r),
	})
}

func (h *AppHandlers) ListLocations(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAppHandlerService(w, r, h != nil && h.Catalog != nil)
	if !ok {
		return
	}
	locations, err := h.Catalog.ListLocations(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err, "list locations failed")
		return
	}
	response := make([]CatalogResponse, 0, len(locations))
	for _, location := range locations {
		response = append(response, locationResponse(location))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AppHandlers) ListSpecies(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAppHandlerService(w, r, h != nil && h.Catalog != nil)
	if !ok {
		return
	}
	speciesList, err := h.Catalog.ListSpecies(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err, "list species failed")
		return
	}
	response := make([]CatalogResponse, 0, len(speciesList))
	for _, species := range speciesList {
		response = append(response, speciesResponse(species))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AppHandlers) ListObservations(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAppHandlerService(w, r, h != nil && h.Observations != nil)
	if !ok {
		return
	}
	observations, err := h.Observations.ListForApp(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err, "list observations failed")
		return
	}
	response := make([]AppObservationResponse, 0, len(observations))
	for _, observation := range observations {
		response = append(response, appObservationResponse(observation))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AppHandlers) CreateObservation(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAppHandlerService(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeAppObservationInput(w, r, actor)
	if !decoded {
		return
	}
	var response AppObservationResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Observations == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		observation, err := services.Observations.Create(r.Context(), actor, input)
		if err != nil {
			return err
		}
		response = appObservationResponse(observation)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionCreate, "observations", observation.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "create observation failed")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *AppHandlers) UpdateObservation(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAppUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeAppObservationInput(w, r, actor)
	if !decoded {
		return
	}
	var response AppObservationResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Observations == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		observation, err := services.Observations.Update(r.Context(), actor, id, input)
		if err != nil {
			return err
		}
		response = appObservationResponse(observation)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionUpdate, "observations", observation.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "update observation failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AppHandlers) DeleteObservation(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAppUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Observations == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		if err := services.Observations.Delete(r.Context(), actor, id); err != nil {
			return err
		}
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionDelete, "observations", id, nil, nil)
	})
	if err != nil {
		writeServiceError(w, err, "delete observation failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireAppHandlerService(w http.ResponseWriter, r *http.Request, configured bool) (policy.User, bool) {
	actor, ok := currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return policy.User{}, false
	}
	if !configured {
		writeError(w, http.StatusServiceUnavailable, "app api is not configured")
		return policy.User{}, false
	}
	return actor, true
}

func requireAppUUID(w http.ResponseWriter, r *http.Request, configured bool) (policy.User, uuid.UUID, bool) {
	actor, ok := requireAppHandlerService(w, r, configured)
	if !ok {
		return policy.User{}, uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "id")))
	if err != nil || id == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return policy.User{}, uuid.Nil, false
	}
	return actor, id, true
}

func decodeAppObservationInput(w http.ResponseWriter, r *http.Request, actor policy.User) (service.ObservationInput, bool) {
	var req SaveObservationRequest
	if !decodeAdminJSON(w, r, &req) {
		return service.ObservationInput{}, false
	}
	observedOn, err := parseObservationDate(req.ObservedOn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid observedOn")
		return service.ObservationInput{}, false
	}
	locationID, err := uuid.Parse(strings.TrimSpace(req.LocationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid locationId")
		return service.ObservationInput{}, false
	}
	speciesID, err := uuid.Parse(strings.TrimSpace(req.SpeciesID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid speciesId")
		return service.ObservationInput{}, false
	}
	return service.ObservationInput{
		ObservedOn: observedOn,
		LocationID: locationID,
		SpeciesID:  speciesID,
		ObserverID: actor.ID,
		Count:      req.Count,
		Notes:      req.Notes,
	}, true
}

func appObservationResponse(observation service.Observation) AppObservationResponse {
	return AppObservationResponse{
		ID:         observation.ID.String(),
		ObservedOn: observation.ObservedOn.Format("2006-01-02"),
		LocationID: observation.LocationID.String(),
		SpeciesID:  observation.SpeciesID.String(),
		Count:      observation.Count,
		Notes:      observation.Notes,
		CreatedAt:  formatTime(observation.CreatedAt),
		UpdatedAt:  formatTime(observation.UpdatedAt),
	}
}
