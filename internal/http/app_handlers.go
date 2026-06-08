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
	ReefCheck    AppReefCheckService
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

type AppReefCheckService interface {
	ListForApp(ctx context.Context, actor policy.User) ([]service.ReefCheckSurvey, error)
	Create(ctx context.Context, actor policy.User, input service.ReefCheckSurveyInput) (service.ReefCheckSurvey, error)
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

func (h *AppHandlers) ReefCheckConfig(w http.ResponseWriter, r *http.Request) {
	_, ok := requireAppHandlerService(w, r, true)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, reefCheckConfigResponse())
}

func (h *AppHandlers) ListReefCheckSurveys(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAppHandlerService(w, r, h != nil && h.ReefCheck != nil)
	if !ok {
		return
	}
	surveys, err := h.ReefCheck.ListForApp(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err, "list reef check surveys failed")
		return
	}
	response := make([]ReefCheckSurveyResponse, 0, len(surveys))
	for _, survey := range surveys {
		response = append(response, reefCheckSurveyResponse(survey))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *AppHandlers) CreateReefCheckSurvey(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAppHandlerService(w, r, h != nil && h.Mutations != nil)
	if !ok {
		return
	}
	input, decoded := decodeReefCheckSurveyInput(w, r)
	if !decoded {
		return
	}
	var response ReefCheckSurveyResponse
	err := h.Mutations.RunAdminMutation(r.Context(), func(services AdminMutationServices) error {
		if services.ReefCheck == nil || services.AuditLogs == nil {
			return errAdminMutationUnavailable
		}
		survey, err := services.ReefCheck.Create(r.Context(), actor, input)
		if err != nil {
			return err
		}
		response = reefCheckSurveyResponse(survey)
		return writeAudit(r, services.AuditLogs, actor, repository.AuditActionCreate, "reef_check_surveys", survey.ID, nil, response)
	})
	if err != nil {
		writeServiceError(w, err, "create reef check survey failed")
		return
	}
	writeJSON(w, http.StatusCreated, response)
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

func reefCheckConfigResponse() ReefCheckConfigResponse {
	segments := service.ReefCheckDefaultSegments()
	segmentResponses := make([]ReefCheckSegmentResponse, 0, len(segments))
	for _, segment := range segments {
		segmentResponses = append(segmentResponses, ReefCheckSegmentResponse{
			Index:  segment.Index,
			Label:  segment.Label,
			StartM: segment.StartM,
			EndM:   segment.EndM,
		})
	}
	codes := service.ReefCheckSubstrateCodeCatalog()
	codeResponses := make([]ReefCheckSubstrateCodeResponse, 0, len(codes))
	for _, code := range codes {
		codeResponses = append(codeResponses, ReefCheckSubstrateCodeResponse{
			Code:               code.Code,
			DisplayName:        code.DisplayName,
			NormalizedCategory: code.NormalizedCategory,
		})
	}
	metrics := service.ReefCheckMetricCatalog()
	metricResponses := make([]ReefCheckMetricResponse, 0, len(metrics))
	for _, metric := range metrics {
		metricResponses = append(metricResponses, ReefCheckMetricResponse{
			Module:      string(metric.Module),
			Key:         metric.Key,
			ChineseName: metric.ChineseName,
			EnglishName: metric.EnglishName,
			SizeClass:   metric.SizeClass,
			SortOrder:   metric.SortOrder,
		})
	}
	return ReefCheckConfigResponse{
		Segments:        segmentResponses,
		SubstrateCodes:  codeResponses,
		Metrics:         metricResponses,
		FishLengthModes: []string{string(service.ReefCheckFishLengthModeSeparate), string(service.ReefCheckFishLengthModeCombined)},
	}
}

func decodeReefCheckSurveyInput(w http.ResponseWriter, r *http.Request) (service.ReefCheckSurveyInput, bool) {
	var req SaveReefCheckSurveyRequest
	if !decodeAdminJSON(w, r, &req) {
		return service.ReefCheckSurveyInput{}, false
	}
	surveyDate, err := parseObservationDate(req.SurveyDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid surveyDate")
		return service.ReefCheckSurveyInput{}, false
	}
	siteID := uuid.Nil
	if strings.TrimSpace(req.SiteID) != "" {
		siteID, err = uuid.Parse(strings.TrimSpace(req.SiteID))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid siteId")
			return service.ReefCheckSurveyInput{}, false
		}
	}
	recorders, ok := decodeReefCheckRecorders(w, req.Recorders)
	if !ok {
		return service.ReefCheckSurveyInput{}, false
	}
	return service.ReefCheckSurveyInput{
		SurveyDate: surveyDate,
		SiteID:     siteID,
		Site: service.ReefCheckSiteInput{
			County:          req.Site.County,
			LocationName:    req.Site.LocationName,
			SiteName:        req.Site.SiteName,
			SiteEnglishName: req.Site.SiteEnglishName,
			Latitude:        req.Site.Latitude,
			Longitude:       req.Site.Longitude,
		},
		DepthM:              req.DepthM,
		CountryIsland:       req.CountryIsland,
		TeamLeader:          req.TeamLeader,
		SurveyTime:          req.SurveyTime,
		Visibility:          req.Visibility,
		Temperature:         req.Temperature,
		GeneralComments:     req.GeneralComments,
		SubstrateComments:   req.SubstrateComments,
		RKCReason:           req.RKCReason,
		RKCBleachingPercent: req.RKCBleachingPercent,
		FishLengthMode:      service.ReefCheckFishLengthMode(req.FishLengthMode),
		Recorders:           recorders,
		Segments:            decodeReefCheckSegments(req.Segments),
		SubstratePoints:     decodeSubstratePoints(req.SubstratePoints),
		SubstrateBleaching:  decodeSubstrateBleaching(req.SubstrateBleaching),
		MetricCounts:        decodeReefCheckMetricCounts(req.MetricCounts),
	}, true
}

func decodeReefCheckRecorders(w http.ResponseWriter, req []ReefCheckRecorderRequest) ([]service.ReefCheckRecorderInput, bool) {
	recorders := make([]service.ReefCheckRecorderInput, 0, len(req))
	for _, recorder := range req {
		userID := uuid.Nil
		if strings.TrimSpace(recorder.UserID) != "" {
			parsed, err := uuid.Parse(strings.TrimSpace(recorder.UserID))
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid recorder userId")
				return nil, false
			}
			userID = parsed
		}
		recorders = append(recorders, service.ReefCheckRecorderInput{
			Role:         service.ReefCheckRecorderRole(recorder.Role),
			UserID:       userID,
			RecorderName: recorder.RecorderName,
		})
	}
	return recorders, true
}

func decodeReefCheckSegments(req []ReefCheckSegmentRequest) []service.ReefCheckSegmentInput {
	segments := make([]service.ReefCheckSegmentInput, 0, len(req))
	for _, segment := range req {
		segments = append(segments, service.ReefCheckSegmentInput{
			Index:  segment.Index,
			Label:  segment.Label,
			StartM: segment.StartM,
			EndM:   segment.EndM,
		})
	}
	return segments
}

func decodeSubstratePoints(req []SubstratePointRequest) []service.SubstratePointInput {
	points := make([]service.SubstratePointInput, 0, len(req))
	for _, point := range req {
		points = append(points, service.SubstratePointInput{
			SegmentIndex: point.SegmentIndex,
			PointIndex:   point.PointIndex,
			TransectM:    point.TransectM,
			Code:         point.Code,
		})
	}
	return points
}

func decodeSubstrateBleaching(req []SubstrateBleachingRequest) []service.SubstrateBleachingInput {
	rows := make([]service.SubstrateBleachingInput, 0, len(req))
	for _, row := range req {
		rows = append(rows, service.SubstrateBleachingInput{
			SegmentIndex:    row.SegmentIndex,
			HCBleachedCount: row.HCBleachedCount,
			SCBleachedCount: row.SCBleachedCount,
		})
	}
	return rows
}

func decodeReefCheckMetricCounts(req []ReefCheckMetricCountRequest) []service.ReefCheckMetricCountInput {
	counts := make([]service.ReefCheckMetricCountInput, 0, len(req))
	for _, count := range req {
		counts = append(counts, service.ReefCheckMetricCountInput{
			Module:       service.ReefCheckModule(count.Module),
			MetricKey:    count.MetricKey,
			SegmentIndex: count.SegmentIndex,
			Count:        count.Count,
			Comment:      count.Comment,
		})
	}
	return counts
}

func reefCheckSurveyResponse(survey service.ReefCheckSurvey) ReefCheckSurveyResponse {
	return ReefCheckSurveyResponse{
		ID:                  survey.ID.String(),
		SurveyDate:          survey.SurveyDate.Format("2006-01-02"),
		SiteID:              survey.SiteID.String(),
		DepthM:              survey.DepthM,
		CountryIsland:       survey.CountryIsland,
		TeamLeader:          survey.TeamLeader,
		SurveyTime:          survey.SurveyTime,
		Visibility:          survey.Visibility,
		Temperature:         survey.Temperature,
		GeneralComments:     survey.GeneralComments,
		SubstrateComments:   survey.SubstrateComments,
		RKCReason:           survey.RKCReason,
		RKCBleachingPercent: survey.RKCBleachingPercent,
		FishLengthMode:      string(survey.FishLengthMode),
		CreatedAt:           formatTime(survey.CreatedAt),
		UpdatedAt:           formatTime(survey.UpdatedAt),
	}
}
