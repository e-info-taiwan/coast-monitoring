package httpx

import (
	"archive/zip"
	"bytes"
	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type reportStub struct {
	dataHandlerStub
	events []service.ReefDataEvent
	loaded []int
	fail   bool
}

func (s *reportStub) ListEvents(context.Context) ([]service.ReefDataEvent, error) {
	return s.events, nil
}
func (s *reportStub) Event(_ context.Context, id int) (service.ReefDataDetail, error) {
	s.loaded = append(s.loaded, id)
	if s.fail {
		return service.ReefDataDetail{}, service.ErrNotFound
	}
	return service.ReefDataDetail{Event: service.ReefDataEvent{ID: id, EventID: "saved-event"}}, nil
}
func TestReefReportAdminDownloadAndFilters(t *testing.T) {
	stub := &reportStub{events: []service.ReefDataEvent{
		{ID: 1, SiteID: 2, SiteName: "合界", SurveyDate: "2026-04-11", Methods: []string{"line", "belt_fish"}},
		{ID: 2, SiteID: 2, SiteName: "合界", SurveyDate: "2025-04-11", Methods: []string{"line"}},
		{ID: 3, SiteID: 3, SiteName: "其他", SurveyDate: "2026-04-11", Methods: []string{"line"}},
	}}
	h := &AdminHandlers{ReefData: stub}
	req := dataHandlerRequest("GET", "/api/admin/reef-check-data/reports.xlsx?year=2026&site=2&method=belt_fish&search=合界", "", policy.RoleAdmin)
	w := httptest.NewRecorder()
	h.ExportReefReports(w, req)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "spreadsheetml") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(stub.loaded) != 1 || stub.loaded[0] != 1 {
		t.Fatalf("filters ignored: %v", stub.loaded)
	}
	if _, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len())); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"year=invalid", "site=-2", "method=invalid", "year=2024"} {
		w := httptest.NewRecorder()
		h.ExportReefReports(w, dataHandlerRequest("GET", "/reports.xlsx?"+q, "", policy.RoleAdmin))
		if w.Code != 400 && w.Code != 404 {
			t.Fatal(q, w.Code)
		}
	}
	w = httptest.NewRecorder()
	h.ExportReefReports(w, dataHandlerRequest("GET", "/reports.xlsx", "", policy.RoleVolunteer))
	if w.Code != http.StatusForbidden {
		t.Fatal("non-admin download", w.Code)
	}
}
func TestReefReportSingleEventAndFailure(t *testing.T) {
	stub := &reportStub{}
	h := &AdminHandlers{ReefData: stub}
	request := func(id string) *http.Request {
		req := dataHandlerRequest("GET", "/report.xlsx", "", policy.RoleAdmin)
		route := chi.NewRouteContext()
		route.URLParams.Add("id", id)
		return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	}
	for _, test := range []struct {
		id   string
		fail bool
		want int
	}{{"12", false, 200}, {"bad", false, 400}, {"12", true, 404}} {
		stub.fail = test.fail
		w := httptest.NewRecorder()
		h.ExportReefReport(w, request(test.id))
		if w.Code != test.want {
			t.Fatal(test, w.Code)
		}
		if test.want != 200 && strings.Contains(w.Header().Get("Content-Type"), "spreadsheetml") {
			t.Fatal("partial workbook on error")
		}
	}
}
func TestReefReportDoesNotSilentlyTruncate(t *testing.T) {
	stub := &reportStub{}
	for i := 0; i < 101; i++ {
		stub.events = append(stub.events, service.ReefDataEvent{ID: i + 1})
	}
	h := &AdminHandlers{ReefData: stub}
	w := httptest.NewRecorder()
	h.ExportReefReports(w, dataHandlerRequest("GET", "/reports.xlsx", "", policy.RoleAdmin))
	if w.Code != 400 || len(stub.loaded) != 0 {
		t.Fatal(w.Code, len(stub.loaded))
	}
}
