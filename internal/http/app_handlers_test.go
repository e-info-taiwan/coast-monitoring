package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"

	"github.com/google/uuid"
)

func TestAppRoutesRequireSession(t *testing.T) {
	router := NewRouter(Dependencies{
		AuthHandlers: testAuthHandlers(),
		AppHandlers:  &AppHandlers{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/app/observations", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAppRoutesDoNotExposeUsersEndpoint(t *testing.T) {
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: activeVolunteer()}
	router := NewRouter(Dependencies{
		AuthHandlers: handlers,
		AppHandlers:  &AppHandlers{},
	})

	req := appRequest(http.MethodGet, "/api/app/users", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestVolunteerOnlySeesOwnObservations(t *testing.T) {
	volunteer := activeVolunteer()
	otherID := uuid.New()
	observations := &fakeAppObservationService{observations: []service.Observation{
		testObservation(volunteer.ID, 2),
		testObservation(otherID, 4),
	}}
	router := appRouterForUser(volunteer, observations, nil, nil)

	req := appRequest(http.MethodGet, "/api/app/observations", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("body length = %d, want 1", len(body))
	}
	if got := int(body[0]["count"].(float64)); got != 2 {
		t.Fatalf("count = %d, want volunteer observation", got)
	}
	for _, forbidden := range []string{"observerId", "observer", "user", "userId", "email", "role"} {
		if _, ok := body[0][forbidden]; ok {
			t.Fatalf("app observation response leaked %q: %#v", forbidden, body[0])
		}
	}
}

func TestVolunteerCannotUpdateOtherUsersObservation(t *testing.T) {
	volunteer := activeVolunteer()
	otherObservation := testObservation(uuid.New(), 1)
	observations := &fakeAppObservationService{observations: []service.Observation{otherObservation}}
	auditLogs := &fakeAdminAuditLogService{}
	router := appRouterForUser(volunteer, observations, auditLogs, nil)

	req := appRequest(http.MethodPatch, "/api/app/observations/"+otherObservation.ID.String(), bytes.NewBufferString(appObservationJSON(uuid.New(), uuid.New(), uuid.New())))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if len(auditLogs.created) != 0 {
		t.Fatalf("audit log count = %d, want 0 for forbidden update", len(auditLogs.created))
	}
}

func TestAdminCanUseAppObservationRoute(t *testing.T) {
	admin := activeAdminHTTP()
	observations := &fakeAppObservationService{}
	auditLogs := &fakeAdminAuditLogService{}
	runner := &fakeAdminMutationRunner{services: AdminMutationServices{
		Observations: observations,
		AuditLogs:    auditLogs,
	}}
	router := appRouterForUser(admin, observations, auditLogs, runner)
	locationID := uuid.New()
	speciesID := uuid.New()
	otherObserverID := uuid.New()

	req := appRequest(http.MethodPost, "/api/app/observations", bytes.NewBufferString(appObservationJSON(locationID, speciesID, otherObserverID)))
	req.RemoteAddr = "192.0.2.55:12345"
	req.Header.Set("User-Agent", "app-test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if runner.calls != 1 {
		t.Fatalf("mutation runner calls = %d, want 1", runner.calls)
	}
	if observations.created.ObserverID != admin.ID {
		t.Fatalf("created observer = %s, want current admin %s", observations.created.ObserverID, admin.ID)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	if _, ok := body["observerId"]; ok {
		t.Fatalf("app create response leaked observerId: %#v", body)
	}
	if len(auditLogs.created) != 1 {
		t.Fatalf("audit log count = %d, want 1", len(auditLogs.created))
	}
	audit := auditLogs.created[0]
	if audit.Action != repository.AuditActionCreate || audit.TargetTable != "observations" || audit.ActorUserID == nil || *audit.ActorUserID != admin.ID {
		t.Fatalf("audit log = %+v, want create observations by admin", audit)
	}
}

func TestReefCheckConfigExposesPDFCatalogs(t *testing.T) {
	router := appRouterForUser(activeVolunteer(), &fakeAppObservationService{}, &fakeAdminAuditLogService{}, nil)

	req := appRequest(http.MethodGet, "/api/app/reef-check/config", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("Decode body error = %v", err)
	}
	substrateCodes := body["substrateCodes"].([]any)
	if len(substrateCodes) != 21 {
		t.Fatalf("substrate code count = %d, want 21", len(substrateCodes))
	}
	metrics := body["metrics"].([]any)
	keys := map[string]bool{}
	for _, item := range metrics {
		metric := item.(map[string]any)
		keys[string(metric["key"].(string))] = true
	}
	for _, want := range []string{"butterflyfish", "grouper_30_40", "lobster", "trash", "turtles"} {
		if !keys[want] {
			t.Fatalf("config missing metric key %q", want)
		}
	}
}

func TestAppCanCreateReefCheckSurvey(t *testing.T) {
	admin := activeAdminHTTP()
	reefCheck := &fakeAppReefCheckService{}
	auditLogs := &fakeAdminAuditLogService{}
	runner := &fakeAdminMutationRunner{services: AdminMutationServices{
		ReefCheck: reefCheck,
		AuditLogs: auditLogs,
	}}
	router := appRouterForUserWithReefCheck(admin, reefCheck, auditLogs, runner)

	req := appRequest(http.MethodPost, "/api/app/reef-check/surveys", bytes.NewBufferString(reefCheckSurveyJSON()))
	req.RemoteAddr = "192.0.2.56:12345"
	req.Header.Set("User-Agent", "reef-check-test")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if runner.calls != 1 {
		t.Fatalf("mutation runner calls = %d, want 1", runner.calls)
	}
	if reefCheck.created.Site.County != "屏東縣" || reefCheck.created.Site.SiteName != "合界" {
		t.Fatalf("created site = %+v, want PDF sample site fields", reefCheck.created.Site)
	}
	if len(reefCheck.created.SubstratePoints) != 160 {
		t.Fatalf("substrate points = %d, want 160", len(reefCheck.created.SubstratePoints))
	}
	if got := reefCheck.created.SubstratePoints[0].TransectM; got != 0 {
		t.Fatalf("first substrate transectM = %.1f, want 0.0", got)
	}
	if got := reefCheck.created.SubstratePoints[159].TransectM; got != 94.5 {
		t.Fatalf("last substrate transectM = %.1f, want 94.5", got)
	}
	if len(reefCheck.created.MetricCounts) != 20 {
		t.Fatalf("metric counts = %d, want 20", len(reefCheck.created.MetricCounts))
	}
	if reefCheck.created.CountryIsland != "台灣" {
		t.Fatalf("country island = %q, want 台灣", reefCheck.created.CountryIsland)
	}
	if reefCheck.created.TeamLeader != "小鄭教練" {
		t.Fatalf("team leader = %q, want 小鄭教練", reefCheck.created.TeamLeader)
	}
	if reefCheck.created.SurveyTime != "08:40" {
		t.Fatalf("survey time = %q, want 08:40", reefCheck.created.SurveyTime)
	}
	if reefCheck.created.Visibility != "15米" {
		t.Fatalf("visibility = %q, want 15米", reefCheck.created.Visibility)
	}
	if reefCheck.created.Temperature != "28度" {
		t.Fatalf("temperature = %q, want 28度", reefCheck.created.Temperature)
	}
	if reefCheck.created.SubstrateComments != "現場表格備註" {
		t.Fatalf("substrate comments = %q, want 現場表格備註", reefCheck.created.SubstrateComments)
	}
	if reefCheck.created.RKCBleachingPercent == nil || *reefCheck.created.RKCBleachingPercent != 12.5 {
		t.Fatalf("rkc bleaching percent = %v, want 12.5", reefCheck.created.RKCBleachingPercent)
	}
	if len(auditLogs.created) != 1 {
		t.Fatalf("audit log count = %d, want 1", len(auditLogs.created))
	}
	audit := auditLogs.created[0]
	if audit.Action != repository.AuditActionCreate || audit.TargetTable != "reef_check_surveys" || audit.ActorUserID == nil || *audit.ActorUserID != admin.ID {
		t.Fatalf("audit log = %+v, want create reef_check_surveys by admin", audit)
	}
}

func appRouterForUser(user policy.User, observations *fakeAppObservationService, auditLogs *fakeAdminAuditLogService, runner AdminMutationRunner) http.Handler {
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: user}
	if runner == nil {
		runner = &fakeAdminMutationRunner{services: AdminMutationServices{
			Observations: observations,
			AuditLogs:    auditLogs,
		}}
	}
	return NewRouter(Dependencies{
		AuthHandlers: handlers,
		AppHandlers: &AppHandlers{
			Catalog:      &fakeAdminCatalogService{},
			Observations: observations,
			Mutations:    runner,
		},
	})
}

func appRouterForUserWithReefCheck(user policy.User, reefCheck *fakeAppReefCheckService, auditLogs *fakeAdminAuditLogService, runner AdminMutationRunner) http.Handler {
	handlers := testAuthHandlers()
	handlers.Auth = &fakeHTTPAuthService{sessionUser: user}
	if runner == nil {
		runner = &fakeAdminMutationRunner{services: AdminMutationServices{
			ReefCheck: reefCheck,
			AuditLogs: auditLogs,
		}}
	}
	return NewRouter(Dependencies{
		AuthHandlers: handlers,
		AppHandlers: &AppHandlers{
			Catalog:      &fakeAdminCatalogService{},
			Observations: &fakeAppObservationService{},
			ReefCheck:    reefCheck,
			Mutations:    runner,
		},
	})
}

func appRequest(method, target string, body *bytes.Buffer) *http.Request {
	req := adminRequest(method, target, body)
	return req
}

func appObservationJSON(locationID, speciesID, observerID uuid.UUID) string {
	return `{"observedOn":"2026-05-27","locationId":"` + locationID.String() + `","speciesId":"` + speciesID.String() + `","observerId":"` + observerID.String() + `","count":3,"notes":" app notes "}`
}

func testObservation(observerID uuid.UUID, count int) service.Observation {
	return service.Observation{
		ID:         uuid.New(),
		ObservedOn: time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC),
		LocationID: uuid.New(),
		SpeciesID:  uuid.New(),
		ObserverID: observerID,
		Count:      count,
		Notes:      "notes",
		CreatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 5, 27, 4, 5, 6, 0, time.UTC),
	}
}

func reefCheckSurveyJSON() string {
	payload := map[string]any{
		"surveyDate":          "2026-04-15",
		"depthM":              5,
		"fishLengthMode":      "separate",
		"generalComments":     "PDF verification payload",
		"countryIsland":       "台灣",
		"teamLeader":          "小鄭教練",
		"surveyTime":          "08:40",
		"visibility":          "15米",
		"temperature":         "28度",
		"substrateComments":   "現場表格備註",
		"rkcBleachingPercent": 12.5,
		"site": map[string]any{
			"county":          "屏東縣",
			"locationName":    "墾丁",
			"siteName":        "合界",
			"siteEnglishName": "Houbihu",
			"latitude":        21.950321,
			"longitude":       120.744123,
		},
		"recorders": []map[string]any{
			{"role": "benthos", "recorderName": "Benthos Recorder"},
			{"role": "fish", "recorderName": "Fish Recorder"},
			{"role": "invertebrate", "recorderName": "Invert Recorder"},
		},
		"segments": []map[string]any{
			{"index": 1, "label": "0-20m", "startM": 0, "endM": 20},
			{"index": 2, "label": "25-45m", "startM": 25, "endM": 45},
			{"index": 3, "label": "50-70m", "startM": 50, "endM": 70},
			{"index": 4, "label": "75-95m", "startM": 75, "endM": 95},
		},
		"substratePoints":    reefCheckSubstratePointPayload(),
		"substrateBleaching": reefCheckBleachingPayload(),
		"metricCounts":       reefCheckMetricPayload(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func reefCheckSubstratePointPayload() []map[string]any {
	points := make([]map[string]any, 0, 160)
	for segment := 1; segment <= 4; segment++ {
		for point := 1; point <= 40; point++ {
			code := "8"
			if point <= 10 {
				code = "1"
			} else if point <= 20 {
				code = "2"
			} else if point <= 25 {
				code = "4"
			}
			points = append(points, map[string]any{
				"segmentIndex": segment,
				"pointIndex":   point,
				"transectM":    reefCheckTestTransectMeter(segment, point),
				"code":         code,
			})
		}
	}
	return points
}

func reefCheckTestTransectMeter(segment, point int) float64 {
	starts := map[int]float64{1: 0, 2: 25, 3: 50, 4: 75}
	return starts[segment] + float64(point-1)*0.5
}

func reefCheckBleachingPayload() []map[string]any {
	return []map[string]any{
		{"segmentIndex": 1, "hcBleachedCount": 0, "scBleachedCount": 0},
		{"segmentIndex": 2, "hcBleachedCount": 1, "scBleachedCount": 0},
		{"segmentIndex": 3, "hcBleachedCount": 0, "scBleachedCount": 1},
		{"segmentIndex": 4, "hcBleachedCount": 0, "scBleachedCount": 0},
	}
}

func reefCheckMetricPayload() []map[string]any {
	metrics := []struct {
		module string
		key    string
	}{
		{module: "fish", key: "butterflyfish"},
		{module: "fish", key: "grouper_30_40"},
		{module: "invertebrate", key: "lobster"},
		{module: "impact", key: "trash"},
		{module: "rare_organism", key: "turtles"},
	}
	counts := make([]map[string]any, 0, len(metrics)*4)
	for _, metric := range metrics {
		for segment := 1; segment <= 4; segment++ {
			counts = append(counts, map[string]any{
				"module":       metric.module,
				"metricKey":    metric.key,
				"segmentIndex": segment,
				"count":        segment - 1,
			})
		}
	}
	return counts
}

type fakeAppObservationService struct {
	observations []service.Observation
	created      service.ObservationInput
	updated      service.ObservationInput
	deletedID    uuid.UUID
}

func (s *fakeAppObservationService) ListForApp(ctx context.Context, actor policy.User) ([]service.Observation, error) {
	if actor.Role == policy.RoleVolunteer {
		var out []service.Observation
		for _, observation := range s.observations {
			if observation.ObserverID == actor.ID {
				out = append(out, observation)
			}
		}
		return out, nil
	}
	return append([]service.Observation(nil), s.observations...), nil
}

func (s *fakeAppObservationService) ListForAdmin(ctx context.Context, actor policy.User) ([]service.Observation, error) {
	return append([]service.Observation(nil), s.observations...), nil
}

func (s *fakeAppObservationService) Create(ctx context.Context, actor policy.User, input service.ObservationInput) (service.Observation, error) {
	s.created = input
	return service.Observation{
		ID:         uuid.New(),
		ObservedOn: input.ObservedOn,
		LocationID: input.LocationID,
		SpeciesID:  input.SpeciesID,
		ObserverID: input.ObserverID,
		Count:      input.Count,
		Notes:      input.Notes,
		CreatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
	}, nil
}

func (s *fakeAppObservationService) Update(ctx context.Context, actor policy.User, id uuid.UUID, input service.ObservationInput) (service.Observation, error) {
	for _, observation := range s.observations {
		if observation.ID == id && actor.Role == policy.RoleVolunteer && observation.ObserverID != actor.ID {
			return service.Observation{}, service.ErrForbidden
		}
	}
	s.updated = input
	return service.Observation{
		ID:         id,
		ObservedOn: input.ObservedOn,
		LocationID: input.LocationID,
		SpeciesID:  input.SpeciesID,
		ObserverID: input.ObserverID,
		Count:      input.Count,
		Notes:      input.Notes,
		CreatedAt:  time.Date(2026, 5, 27, 1, 2, 3, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 5, 27, 4, 5, 6, 0, time.UTC),
	}, nil
}

func (s *fakeAppObservationService) Delete(ctx context.Context, actor policy.User, id uuid.UUID) error {
	s.deletedID = id
	return nil
}

type fakeAppReefCheckService struct {
	created service.ReefCheckSurveyInput
}

func (s *fakeAppReefCheckService) ListForApp(ctx context.Context, actor policy.User) ([]service.ReefCheckSurvey, error) {
	return nil, nil
}

func (s *fakeAppReefCheckService) Create(ctx context.Context, actor policy.User, input service.ReefCheckSurveyInput) (service.ReefCheckSurvey, error) {
	s.created = input
	return service.ReefCheckSurvey{
		ID:        uuid.New(),
		CreatedBy: actor.ID,
		UpdatedBy: actor.ID,
	}, nil
}
