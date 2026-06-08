package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxAdminRequestBodyBytes = 1 << 20

type AdminHandlers struct {
	Users        AdminUserService
	Catalog      AdminCatalogService
	Observations AdminObservationService
	ReefCheck    AppReefCheckService
	AuditLogs    AdminAuditLogService
	Mutations    AdminMutationRunner
}

type AdminUserService interface {
	ListUsers(ctx context.Context, actor policy.User) ([]service.User, error)
	CreateUser(ctx context.Context, actor policy.User, input service.CreateUserInput) (service.User, error)
	UpdateUser(ctx context.Context, actor policy.User, id uuid.UUID, input service.UpdateUserInput) (service.User, error)
	DisableUser(ctx context.Context, actor policy.User, id uuid.UUID) error
}

type AdminCatalogService interface {
	ListLocations(ctx context.Context, actor policy.User) ([]service.Location, error)
	CreateLocation(ctx context.Context, actor policy.User, input service.CatalogInput) (service.Location, error)
	UpdateLocation(ctx context.Context, actor policy.User, id uuid.UUID, input service.CatalogInput) (service.Location, error)
	DeleteLocation(ctx context.Context, actor policy.User, id uuid.UUID) error
	ListSpecies(ctx context.Context, actor policy.User) ([]service.Species, error)
	CreateSpecies(ctx context.Context, actor policy.User, input service.CatalogInput) (service.Species, error)
	UpdateSpecies(ctx context.Context, actor policy.User, id uuid.UUID, input service.CatalogInput) (service.Species, error)
	DeleteSpecies(ctx context.Context, actor policy.User, id uuid.UUID) error
}

type AdminObservationService interface {
	ListForAdmin(ctx context.Context, actor policy.User) ([]service.Observation, error)
	Create(ctx context.Context, actor policy.User, input service.ObservationInput) (service.Observation, error)
	Update(ctx context.Context, actor policy.User, id uuid.UUID, input service.ObservationInput) (service.Observation, error)
	Delete(ctx context.Context, actor policy.User, id uuid.UUID) error
}

type AdminAuditLogService interface {
	ListAuditLogs(ctx context.Context) ([]repository.AuditLog, error)
	CreateAuditLog(ctx context.Context, input repository.CreateAuditLogRecord) (repository.AuditLog, error)
}

type AdminMutationRunner interface {
	RunAdminMutation(ctx context.Context, fn func(AdminMutationServices) error) error
}

type AdminMutationServices struct {
	Users        AdminUserService
	Catalog      AdminCatalogService
	Observations AdminObservationService
	ReefCheck    AppReefCheckService
	AuditLogs    AdminAuditLogService
}

var errAdminMutationUnavailable = errors.New("admin mutation runner is not configured")

func (h *AdminHandlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdminHandlerService(w, r, h != nil && h.Users != nil)
	if !ok {
		return
	}
	users, err := h.Users.ListUsers(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err, "list users failed")
		return
	}
	response := make([]AdminUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, adminUserResponse(user))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AdminHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdminHandlerService(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	var req SaveUserRequest
	if !decodeAdminJSON(w, r, &req) {
		return
	}
	input := service.CreateUserInput{
		Email:    req.Email,
		Name:     req.Name,
		Role:     policy.Role(req.Role),
		Status:   policy.Status(req.Status),
		Password: req.Password,
	}
	var response AdminUserResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Users == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		user, err := services.Users.CreateUser(r.Context(), actor, input)
		if err != nil {
			return err
		}
		response = adminUserResponse(user)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionCreate, "users", user.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "create user failed")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *AdminHandlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	var req struct {
		Email    *string `json:"email"`
		Name     *string `json:"name"`
		Role     *string `json:"role"`
		Status   *string `json:"status"`
		Password *string `json:"password"`
	}
	if !decodeAdminJSON(w, r, &req) {
		return
	}
	input := service.UpdateUserInput{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	}
	if req.Role != nil {
		role := policy.Role(*req.Role)
		input.Role = &role
	}
	if req.Status != nil {
		status := policy.Status(*req.Status)
		input.Status = &status
	}
	var response AdminUserResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Users == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		user, err := services.Users.UpdateUser(r.Context(), actor, id, input)
		if err != nil {
			return err
		}
		response = adminUserResponse(user)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionUpdate, "users", user.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "update user failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AdminHandlers) DisableUser(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Users == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		if err := services.Users.DisableUser(r.Context(), actor, id); err != nil {
			return err
		}
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionDelete, "users", id, nil, nil)
	})
	if err != nil {
		writeServiceError(w, err, "disable user failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandlers) ListLocations(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdminHandlerService(w, r, h != nil && h.Catalog != nil)
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

func (h *AdminHandlers) CreateLocation(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdminHandlerService(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeCatalogInput(w, r)
	if !decoded {
		return
	}
	var response CatalogResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Catalog == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		location, err := services.Catalog.CreateLocation(r.Context(), actor, input)
		if err != nil {
			return err
		}
		response = locationResponse(location)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionCreate, "locations", location.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "create location failed")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *AdminHandlers) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeCatalogInput(w, r)
	if !decoded {
		return
	}
	var response CatalogResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Catalog == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		location, err := services.Catalog.UpdateLocation(r.Context(), actor, id, input)
		if err != nil {
			return err
		}
		response = locationResponse(location)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionUpdate, "locations", location.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "update location failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AdminHandlers) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Catalog == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		if err := services.Catalog.DeleteLocation(r.Context(), actor, id); err != nil {
			return err
		}
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionDelete, "locations", id, nil, nil)
	})
	if err != nil {
		writeServiceError(w, err, "delete location failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandlers) ListSpecies(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdminHandlerService(w, r, h != nil && h.Catalog != nil)
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

func (h *AdminHandlers) CreateSpecies(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdminHandlerService(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeCatalogInput(w, r)
	if !decoded {
		return
	}
	var response CatalogResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Catalog == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		species, err := services.Catalog.CreateSpecies(r.Context(), actor, input)
		if err != nil {
			return err
		}
		response = speciesResponse(species)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionCreate, "species", species.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "create species failed")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *AdminHandlers) UpdateSpecies(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeCatalogInput(w, r)
	if !decoded {
		return
	}
	var response CatalogResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Catalog == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		species, err := services.Catalog.UpdateSpecies(r.Context(), actor, id, input)
		if err != nil {
			return err
		}
		response = speciesResponse(species)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionUpdate, "species", species.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "update species failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AdminHandlers) DeleteSpecies(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Catalog == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		if err := services.Catalog.DeleteSpecies(r.Context(), actor, id); err != nil {
			return err
		}
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionDelete, "species", id, nil, nil)
	})
	if err != nil {
		writeServiceError(w, err, "delete species failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandlers) ListObservations(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdminHandlerService(w, r, h != nil && h.Observations != nil)
	if !ok {
		return
	}
	observations, err := h.Observations.ListForAdmin(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err, "list observations failed")
		return
	}
	response := make([]ObservationResponse, 0, len(observations))
	for _, observation := range observations {
		response = append(response, observationResponse(observation))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AdminHandlers) UpdateObservation(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeObservationInput(w, r)
	if !decoded {
		return
	}
	var response ObservationResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.Observations == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		observation, err := services.Observations.Update(r.Context(), actor, id, input)
		if err != nil {
			return err
		}
		response = observationResponse(observation)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionUpdate, "observations", observation.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "update observation failed")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AdminHandlers) DeleteObservation(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := requireAdminUUID(w, r, h != nil && h.Mutations != nil)
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

func (h *AdminHandlers) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	_, ok := requireAdminHandlerService(w, r, h != nil && h.AuditLogs != nil)
	if !ok {
		return
	}
	logs, err := h.AuditLogs.ListAuditLogs(r.Context())
	if err != nil {
		writeServiceError(w, err, "list audit logs failed")
		return
	}
	response := make([]AuditLogResponse, 0, len(logs))
	for _, log := range logs {
		response = append(response, auditLogResponse(log))
	}
	writeJSON(w, http.StatusOK, response)
}

func requireAdminHandlerService(w http.ResponseWriter, r *http.Request, configured bool) (policy.User, bool) {
	actor, ok := currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return policy.User{}, false
	}
	if !configured {
		writeError(w, http.StatusServiceUnavailable, "admin api is not configured")
		return policy.User{}, false
	}
	return actor, true
}

func requireAdminUUID(w http.ResponseWriter, r *http.Request, configured bool) (policy.User, uuid.UUID, bool) {
	actor, ok := requireAdminHandlerService(w, r, configured)
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

func decodeAdminJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid json request")
		return false
	}
	return true
}

func decodeCatalogInput(w http.ResponseWriter, r *http.Request) (service.CatalogInput, bool) {
	var req SaveCatalogRequest
	if !decodeAdminJSON(w, r, &req) {
		return service.CatalogInput{}, false
	}
	return service.CatalogInput{
		ChineseName: req.ChineseName,
		EnglishName: req.EnglishName,
	}, true
}

func decodeObservationInput(w http.ResponseWriter, r *http.Request) (service.ObservationInput, bool) {
	var req AdminSaveObservationRequest
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
	observerID, err := uuid.Parse(strings.TrimSpace(req.ObserverID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid observerId")
		return service.ObservationInput{}, false
	}
	return service.ObservationInput{
		ObservedOn: observedOn,
		LocationID: locationID,
		SpeciesID:  speciesID,
		ObserverID: observerID,
		Count:      req.Count,
		Notes:      req.Notes,
	}, true
}

func parseObservationDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty date")
	}
	if date, err := time.Parse("2006-01-02", value); err == nil {
		return date, nil
	}
	return time.Parse(time.RFC3339, value)
}

func adminUserResponse(user service.User) AdminUserResponse {
	return AdminUserResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		Name:        user.Name,
		Role:        string(user.Role),
		Status:      string(user.Status),
		HasGoogle:   user.GoogleSub != nil,
		HasPassword: user.HasPassword,
		CreatedAt:   formatTime(user.CreatedAt),
		UpdatedAt:   formatTime(user.UpdatedAt),
	}
}

func locationResponse(location service.Location) CatalogResponse {
	return CatalogResponse{
		ID:          location.ID.String(),
		ChineseName: location.ChineseName,
		EnglishName: location.EnglishName,
		CreatedAt:   formatTime(location.CreatedAt),
		UpdatedAt:   formatTime(location.UpdatedAt),
	}
}

func speciesResponse(species service.Species) CatalogResponse {
	return CatalogResponse{
		ID:          species.ID.String(),
		ChineseName: species.ChineseName,
		EnglishName: species.EnglishName,
		CreatedAt:   formatTime(species.CreatedAt),
		UpdatedAt:   formatTime(species.UpdatedAt),
	}
}

func observationResponse(observation service.Observation) ObservationResponse {
	return ObservationResponse{
		ID:         observation.ID.String(),
		ObservedOn: observation.ObservedOn.Format("2006-01-02"),
		LocationID: observation.LocationID.String(),
		SpeciesID:  observation.SpeciesID.String(),
		ObserverID: observation.ObserverID.String(),
		Count:      observation.Count,
		Notes:      observation.Notes,
		CreatedAt:  formatTime(observation.CreatedAt),
		UpdatedAt:  formatTime(observation.UpdatedAt),
	}
}

func auditLogResponse(log repository.AuditLog) AuditLogResponse {
	response := AuditLogResponse{
		ID:          log.ID.String(),
		Action:      string(log.Action),
		TargetTable: log.TargetTable,
		TargetID:    log.TargetID.String(),
		ActorEmail:  log.ActorEmail,
		Method:      log.Method,
		Path:        log.Path,
		IP:          log.IP,
		UserAgent:   log.UserAgent,
		LoggedAt:    formatTime(log.LoggedAt),
	}
	if log.ActorUserID != nil {
		response.ActorUserID = log.ActorUserID.String()
	}
	response.BeforeData = safeAuditJSON(log.BeforeData)
	response.AfterData = safeAuditJSON(log.AfterData)
	return response
}

func safeAuditJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return stripSecretFields(value)
}

func stripSecretFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, nested := range typed {
			switch key {
			case "password_hash", "passwordHash", "google_sub", "googleSub":
				continue
			default:
				cleaned[key] = stripSecretFields(nested)
			}
		}
		return cleaned
	case []any:
		cleaned := make([]any, 0, len(typed))
		for _, nested := range typed {
			cleaned = append(cleaned, stripSecretFields(nested))
		}
		return cleaned
	default:
		return value
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func writeAudit(r *http.Request, auditLogs AdminAuditLogService, actor policy.User, action repository.AuditAction, targetTable string, targetID uuid.UUID, before any, after any) error {
	actorID := actor.ID
	_, err := auditLogs.CreateAuditLog(r.Context(), repository.CreateAuditLogRecord{
		Action:      action,
		TargetTable: targetTable,
		TargetID:    targetID,
		ActorUserID: &actorID,
		ActorEmail:  actor.Email,
		BeforeData:  before,
		AfterData:   after,
		Method:      r.Method,
		Path:        r.URL.Path,
		IP:          remoteIP(r),
		UserAgent:   r.UserAgent(),
	})
	return err
}

func writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, errAdminMutationUnavailable):
		writeError(w, http.StatusServiceUnavailable, "admin mutation runner is not configured")
	case errors.Is(err, service.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, service.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, service.ErrConflict):
		writeError(w, http.StatusConflict, "conflict")
	case errors.Is(err, service.ErrInvalidReference):
		writeError(w, http.StatusBadRequest, "invalid reference")
	default:
		writeError(w, http.StatusInternalServerError, fallback)
	}
}
