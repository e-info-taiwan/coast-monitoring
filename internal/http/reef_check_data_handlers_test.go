package httpx

import (
	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/repository"
	"coast-monitoring/internal/service"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type dataHandlerStub struct {
	current service.ReefDataTransect
	updates int
}

func (s *dataHandlerStub) ListEvents(context.Context) ([]service.ReefDataEvent, error) {
	return []service.ReefDataEvent{{ID: 1, EventID: "site_2025_01_01_na_5m", Methods: []string{"line"}}}, nil
}
func (s *dataHandlerStub) Codes(context.Context) ([]service.ReefDataCode, error) {
	return []service.ReefDataCode{{Code: "NA", Active: true}}, nil
}
func (s *dataHandlerStub) Event(context.Context, int) (service.ReefDataDetail, error) {
	return service.ReefDataDetail{Transects: []service.ReefDataTransect{s.current}}, nil
}
func (s *dataHandlerStub) Transect(context.Context, int, bool) (service.ReefDataTransect, error) {
	return s.current, nil
}
func (s *dataHandlerStub) Update(_ context.Context, _ int, u service.ReefDataUpdate) (service.ReefDataTransect, error) {
	s.updates++
	return s.current, nil
}
func (s *dataHandlerStub) CreateEvent(context.Context, service.ReefDataCreateInput) (service.ReefDataDetail, error) {
	return service.ReefDataDetail{Event: service.ReefDataEvent{ID: 10, EventID: "site_test"}}, nil
}
func (s *dataHandlerStub) DeleteEvent(context.Context, int) (service.ReefDataDetail, error) {
	return service.ReefDataDetail{Event: service.ReefDataEvent{ID: 10}}, nil
}
func (s *dataHandlerStub) Sites(context.Context) ([]service.ReefDataSite, error) {
	return []service.ReefDataSite{{ID: 1, NameZH: "Site 1"}}, nil
}
func (s *dataHandlerStub) Users(context.Context) ([]service.ReefDataUser, error) {
	return []service.ReefDataUser{{ID: uuid.New(), Email: "diver@example.com", Name: "Diver"}}, nil
}
func (s *dataHandlerStub) Divers(context.Context) ([]service.ReefDataDiver, error) {
	return []service.ReefDataDiver{{ID: 1, NameZH: "Diver 1"}}, nil
}
func (s *dataHandlerStub) AddParticipant(context.Context, int, service.ReefDataParticipantInput) error {
	return nil
}
func (s *dataHandlerStub) RemoveParticipant(context.Context, int) error {
	return nil
}
func dataHandlerRequest(method, path, body string, role policy.Role) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	return req.WithContext(withCurrentUser(req.Context(), policy.User{ID: uuid.New(), Email: "test@example.test", Role: role, Status: policy.StatusActive}))
}
func TestReefDataHandlerRequiresAdminEvenWhenCalledDirectly(t *testing.T) {
	h := &AdminHandlers{ReefData: &dataHandlerStub{}}
	for _, role := range []policy.Role{policy.RoleVolunteer, policy.RoleAdmin} {
		w := httptest.NewRecorder()
		h.ListReefDataEvents(w, dataHandlerRequest("GET", "/", "", role))
		expected := http.StatusOK
		if role == policy.RoleVolunteer {
			expected = http.StatusForbidden
		}
		if w.Code != expected {
			t.Fatalf("role %s status %d body %s", role, w.Code, w.Body)
		}
	}
	w := httptest.NewRecorder()
	h.ListReefDataEvents(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestReefDataRoutesRequireSession(t *testing.T) {
	for _, path := range []string{"/api/admin/reef-check-data/events", "/api/admin/reef-check-data/codes", "/api/admin/reef-check-data/events/1", "/api/admin/reef-check-data/transects/1"} {
		method := "GET"
		if strings.Contains(path, "transects") {
			method = "PATCH"
		}
		w := httptest.NewRecorder()
		NewRouter(Dependencies{AuthHandlers: testAuthHandlers(), AdminHandlers: &AdminHandlers{ReefData: &dataHandlerStub{}}}).ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != 401 {
			t.Fatalf("%s status %d", path, w.Code)
		}
	}
}
func TestReefDataStrictUpdateBodyAndConflict(t *testing.T) {
	stub := &dataHandlerStub{current: service.ReefDataTransect{ID: 1, Method: "line"}}
	runner := &dataMutationStub{services: AdminMutationServices{ReefData: stub, AuditLogs: &dataAuditStub{}}}
	h := &AdminHandlers{Mutations: runner}
	for _, body := range []string{`{"version":"old","metadata":{"comments":"partial update"}}`, `{"version":"old","unexpected":1}`, `{"version":"old"} {}`, `{"version":"old","changes":[]}`} {
		req := dataHandlerRequest("PATCH", "/1", body, policy.RoleAdmin)
		route := chi.NewRouteContext()
		route.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		h.UpdateReefDataTransect(w, req)
		if w.Code != 400 && w.Code != 409 {
			t.Fatalf("status %d body %s", w.Code, w.Body)
		}
	}
	if stub.updates != 0 {
		t.Fatal("invalid or stale request wrote data")
	}
}

type dataMutationStub struct{ services AdminMutationServices }

func (s *dataMutationStub) RunAdminMutation(ctx context.Context, fn func(AdminMutationServices) error) error {
	return fn(s.services)
}

// The audit failure must be returned to the transaction runner, never acknowledged as success.
func TestReefDataAuditFailureFailsMutation(t *testing.T) {
	stub := &dataHandlerStub{current: service.ReefDataTransect{ID: 1, Method: "line"}}
	runner := &dataMutationStub{services: AdminMutationServices{ReefData: stub, AuditLogs: &dataAuditStub{fail: true}}}
	h := &AdminHandlers{Mutations: runner}
	req := dataHandlerRequest("PATCH", "/1", `{"version":"`+stub.current.Revision()+`","metadata":{"start_time":null,"water_temp_c":null,"visibility_min_m":null,"visibility_max_m":null,"comments":null,"rkc_bleaching_note":null}}`, policy.RoleAdmin)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	h.UpdateReefDataTransect(w, req)
	if w.Code != 500 {
		t.Fatalf("audit failure status %d", w.Code)
	}
}

type dataAuditStub struct {
	fail    bool
	created []repository.CreateAuditLogRecord
}

func (s *dataAuditStub) ListAuditLogs(context.Context) ([]repository.AuditLog, error) {
	return nil, nil
}
func (s *dataAuditStub) CreateAuditLog(_ context.Context, r repository.CreateAuditLogRecord) (repository.AuditLog, error) {
	if s.fail {
		return repository.AuditLog{}, errors.New("audit failed")
	}
	s.created = append(s.created, r)
	return repository.AuditLog{}, nil
}

func TestCreateReefDataEvent(t *testing.T) {
	stub := &dataHandlerStub{}
	audit := &dataAuditStub{}
	runner := &dataMutationStub{services: AdminMutationServices{ReefData: stub, AuditLogs: audit}}
	h := &AdminHandlers{ReefData: stub, Mutations: runner}

	body := `{"site_id":1,"survey_date":"2026-06-01","depth_m":5,"methods":["line","belt_fish"]}`
	req := dataHandlerRequest("POST", "/api/admin/reef-check-data/events", body, policy.RoleAdmin)
	w := httptest.NewRecorder()
	h.CreateReefDataEvent(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body %s", w.Code, http.StatusCreated, w.Body)
	}
	if len(audit.created) != 1 {
		t.Fatalf("audit count = %d, want 1", len(audit.created))
	}
	if audit.created[0].Action != repository.AuditActionCreate || audit.created[0].TargetTable != "event" {
		t.Fatalf("unexpected audit record: %+v", audit.created[0])
	}
}

func TestDeleteReefDataEvent(t *testing.T) {
	stub := &dataHandlerStub{}
	audit := &dataAuditStub{}
	runner := &dataMutationStub{services: AdminMutationServices{ReefData: stub, AuditLogs: audit}}
	h := &AdminHandlers{ReefData: stub, Mutations: runner}

	req := dataHandlerRequest("DELETE", "/api/admin/reef-check-data/events/10", "", policy.RoleAdmin)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	h.DeleteReefDataEvent(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body %s", w.Code, http.StatusNoContent, w.Body)
	}
	if len(audit.created) != 1 {
		t.Fatalf("audit count = %d, want 1", len(audit.created))
	}
	if audit.created[0].Action != repository.AuditActionDelete || audit.created[0].TargetTable != "event" {
		t.Fatalf("unexpected audit record: %+v", audit.created[0])
	}
}

func TestAddAndRemoveReefDataParticipant(t *testing.T) {
	stub := &dataHandlerStub{}
	audit := &dataAuditStub{}
	runner := &dataMutationStub{services: AdminMutationServices{ReefData: stub, AuditLogs: audit}}
	h := &AdminHandlers{ReefData: stub, Mutations: runner}

	// Add participant
	addBody := `{"role":"member","name_zh":"Test Diver"}`
	addReq := dataHandlerRequest("POST", "/api/admin/reef-check-data/transects/1/participants", addBody, policy.RoleAdmin)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "1")
	addReq = addReq.WithContext(context.WithValue(addReq.Context(), chi.RouteCtxKey, route))
	addW := httptest.NewRecorder()
	h.AddReefDataParticipant(addW, addReq)

	if addW.Code != http.StatusOK {
		t.Fatalf("add participant status = %d, want %d, body %s", addW.Code, http.StatusOK, addW.Body)
	}
	if len(audit.created) != 1 || audit.created[0].Action != repository.AuditActionUpdate || audit.created[0].TargetTable != "transect_participant" {
		t.Fatalf("unexpected audit record for add: %+v", audit.created)
	}

	// Remove participant
	delReq := dataHandlerRequest("DELETE", "/api/admin/reef-check-data/participants/100", "", policy.RoleAdmin)
	delRoute := chi.NewRouteContext()
	delRoute.URLParams.Add("id", "100")
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), chi.RouteCtxKey, delRoute))
	delW := httptest.NewRecorder()
	h.RemoveReefDataParticipant(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Fatalf("remove participant status = %d, want %d, body %s", delW.Code, http.StatusNoContent, delW.Body)
	}
	if len(audit.created) != 2 || audit.created[1].Action != repository.AuditActionDelete || audit.created[1].TargetTable != "transect_participant" {
		t.Fatalf("unexpected audit record for remove: %+v", audit.created)
	}
}

func TestReefDataSitesUsersDiversEndpoints(t *testing.T) {
	stub := &dataHandlerStub{}
	h := &AdminHandlers{ReefData: stub}

	// Sites
	wSites := httptest.NewRecorder()
	h.ReefDataSites(wSites, dataHandlerRequest("GET", "/sites", "", policy.RoleAdmin))
	if wSites.Code != http.StatusOK {
		t.Fatalf("sites status = %d", wSites.Code)
	}

	// Users
	wUsers := httptest.NewRecorder()
	h.ReefDataUsers(wUsers, dataHandlerRequest("GET", "/users", "", policy.RoleAdmin))
	if wUsers.Code != http.StatusOK {
		t.Fatalf("users status = %d", wUsers.Code)
	}

	// Divers
	wDivers := httptest.NewRecorder()
	h.ReefDataDivers(wDivers, dataHandlerRequest("GET", "/divers", "", policy.RoleAdmin))
	if wDivers.Code != http.StatusOK {
		t.Fatalf("divers status = %d", wDivers.Code)
	}
}
