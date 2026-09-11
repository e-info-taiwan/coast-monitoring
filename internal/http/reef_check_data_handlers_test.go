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

type dataAuditStub struct{ fail bool }

func (s *dataAuditStub) ListAuditLogs(context.Context) ([]repository.AuditLog, error) {
	return nil, nil
}
func (s *dataAuditStub) CreateAuditLog(context.Context, repository.CreateAuditLogRecord) (repository.AuditLog, error) {
	if s.fail {
		return repository.AuditLog{}, errors.New("audit failed")
	}
	return repository.AuditLog{}, nil
}
