package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"coast-monitoring/internal/policy"
	"coast-monitoring/internal/service"
)

type activityFake struct {
	fakeAppReefDataService
	code    string
	invalid bool
}

func (f *activityFake) VerifyActivity(ctx context.Context, code string) (service.ReefCheckActivity, error) {
	f.code = code
	if f.invalid {
		return service.ReefCheckActivity{}, service.ErrValidation
	}
	return service.ReefCheckActivity{ID: 1, Name: "活動", StartDate: "2026-10-14", EndDate: "2026-10-16", SiteIDs: []int{1}, Active: true, Code: "must-not-leak"}, nil
}
func (f *activityFake) ListActivities(context.Context) ([]service.ReefCheckActivity, error) {
	return nil, nil
}
func (f *activityFake) CreateActivity(context.Context, service.ReefCheckActivityInput, policy.User) (service.ReefCheckActivity, error) {
	return service.ReefCheckActivity{}, nil
}
func (f *activityFake) ChangeActivity(context.Context, int, string, policy.User) (service.ReefCheckActivity, error) {
	return service.ReefCheckActivity{}, nil
}
func TestPublicActivityVerifyNoCodeLeakAndErrors(t *testing.T) {
	fake := &activityFake{}
	router := NewRouter(Dependencies{AuthHandlers: testAuthHandlers(), AppHandlers: &AppHandlers{ReefData: fake}, AdminHandlers: &AdminHandlers{}})
	req := httptest.NewRequest(http.MethodPost, "/api/public/reef-check/activities/verify", bytes.NewBufferString(`{"code":"RC-INPUT"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || fake.code != "RC-INPUT" {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var data map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if _, exists := data["code"]; exists {
		t.Fatal("verification leaked a code")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("verification response can be cached")
	}
	fake.invalid = true
	req = httptest.NewRequest(http.MethodPost, "/api/public/reef-check/activities/verify", bytes.NewBufferString(`{"code":"invalid"}`))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid code accepted: %d", rec.Code)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch} {
		url := "/api/admin/reef-check-data/activities"
		if method == http.MethodPatch {
			url += "/1"
		}
		req = httptest.NewRequest(method, url, bytes.NewBufferString(`{}`))
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
			t.Fatal("activity management allowed anonymous access")
		}
	}
}

func TestPublicEntryCannotBypassActivityWithLegacyFormat(t *testing.T) {
	fake := &fakeAppReefDataService{}
	h := &AppHandlers{ReefData: fake}
	req := httptest.NewRequest(http.MethodPost, "/api/public/reef-check/surveys", bytes.NewBufferString(`{"format":"reefcheck-web-prototype-v2"}`))
	rec := httptest.NewRecorder()
	h.SubmitPublicReefCheckSurvey(rec, req)
	if rec.Code != http.StatusBadRequest || fake.submittedSub.Format != "" {
		t.Fatal("legacy format bypassed activity validation")
	}
}
