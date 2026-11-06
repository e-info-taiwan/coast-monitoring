package httpx

import (
	"bytes"
	"coast-monitoring/internal/service"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type environmentFake struct {
	fakeAppReefDataService
	input service.ReefEntryEnvironmentInput
	err   error
}

func (f *environmentFake) EntryEnvironment(_ context.Context, in service.ReefEntryEnvironmentInput) (service.ReefEntryEnvironment, error) {
	f.input = in
	temp := 27.0
	station := "TEST"
	name := "測試測站"
	at := time.Date(2026, 11, 6, 7, 30, 0, 0, time.UTC)
	return service.ReefEntryEnvironment{SiteID: in.SiteID, StationID: &station, StationName: &name, SeaTempC: &temp, ObservedAt: &at, Source: "cwa_reference"}, f.err
}
func TestEntryEnvironmentReadOnlyContract(t *testing.T) {
	fake := &environmentFake{}
	router := NewRouter(Dependencies{AuthHandlers: testAuthHandlers(), AppHandlers: &AppHandlers{ReefData: fake}, AdminHandlers: &AdminHandlers{}})
	input := service.ReefEntryEnvironmentInput{ActivityCode: "RC-TEST-ONLY", SiteID: 1, SurveyDate: "2026-11-06", EventTime: "15:40"}
	send := func(in service.ReefEntryEnvironmentInput) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(in)
		req := httptest.NewRequest(http.MethodPost, "/api/public/reef-check/entry-environment", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	rec := send(input)
	if rec.Code != 200 || fake.input != input || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(input.ActivityCode)) {
		t.Fatal("activity code leaked")
	}
	var data service.ReefEntryEnvironment
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil || data.SeaTempC == nil || *data.SeaTempC != 27 {
		t.Fatalf("preview: %+v %v", data, err)
	}
	for _, invalid := range []string{"24:00", "12:60", "上午09:00", "9:30", "1540"} {
		in := input
		in.EventTime = invalid
		if r := send(in); r.Code != 400 {
			t.Fatalf("invalid time %s: %d", invalid, r.Code)
		}
	}
	in := input
	in.ActivityCode = ""
	if r := send(in); r.Code != 400 {
		t.Fatal("missing activity accepted")
	}
	fake.err = service.ErrValidation
	if r := send(input); r.Code != 400 {
		t.Fatal("scope error ignored")
	}
}
