package httpx

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"coast-monitoring/internal/service"
	"github.com/go-chi/chi/v5"
)

type prdPublicReefFake struct {
	rows    []service.PublicReefSeries
	filters []service.PublicReefFilter
}

func (f *prdPublicReefFake) PublicSites(context.Context, service.PublicReefFilter) ([]json.RawMessage, error) {
	return []json.RawMessage{}, nil
}
func (f *prdPublicReefFake) SetPublication(context.Context, int, string, string) error { return nil }
func (f *prdPublicReefFake) PublicSeries(_ context.Context, _ int, filter service.PublicReefFilter) ([]service.PublicReefSeries, error) {
	f.filters = append(f.filters, filter)
	return f.rows, nil
}

func TestPRD20261104C09SeriesCSVPreservesEventsAndMissingValues(t *testing.T) {
	zero := 0.0
	fake := &prdPublicReefFake{rows: []service.PublicReefSeries{
		{SiteID: 1, EventID: "A-0900", Date: "2026-11-04", Time: "09:00", Depth: 5, Chart: "taxa", Value: &zero, N: 4, Status: "ok", Unit: "individuals_per_100m2"},
		{SiteID: 1, EventID: "A-1400", Date: "2026-11-04", Time: "14:00", Depth: 5, Chart: "taxa", N: 0, MissingReason: "not_recorded", Unit: "individuals_per_100m2"},
	}}
	h := &PublicHandlers{Reef: fake}
	router := chi.NewRouter()
	router.Get("/sites/{id}/series", h.Series)
	router.Get("/sites/{id}/export.csv", h.Series)
	query := "?from_year=2020&to_year=2026&depth_m=5&charts=taxa&taxon_ids=14,15"
	j := httptest.NewRecorder()
	router.ServeHTTP(j, httptest.NewRequest("GET", "/sites/1/series"+query, nil))
	c := httptest.NewRecorder()
	router.ServeHTTP(c, httptest.NewRequest("GET", "/sites/1/export.csv"+query, nil))
	if j.Code != 200 || c.Code != 200 {
		t.Fatalf("JSON %d / CSV %d", j.Code, c.Code)
	}
	if len(fake.filters) != 2 || !reflect.DeepEqual(fake.filters[0], fake.filters[1]) {
		t.Fatalf("JSON and CSV use different filters: %+v", fake.filters)
	}
	if f := fake.filters[0]; f.FromYear != 2020 || f.ToYear != 2026 || f.Depth == nil || *f.Depth != 5 || !reflect.DeepEqual(f.TaxonIDs, []int{14, 15}) {
		t.Fatalf("filters lost: %+v", f)
	}
	var response struct {
		Series []service.PublicReefSeries `json:"series"`
	}
	if err := json.Unmarshal(j.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(response.Series, fake.rows) {
		t.Fatalf("JSON changed rows: %+v", response.Series)
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(c.Body.String(), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("CSV rows=%d, want header and two events", len(rows))
	}
	index := map[string]int{}
	for i, key := range rows[0] {
		index[key] = i
	}
	for _, key := range []string{"event_id", "event_time", "value", "n", "missing_reason", "unit"} {
		if _, ok := index[key]; !ok {
			t.Fatalf("missing CSV field %s", key)
		}
	}
	if rows[1][index["event_id"]] != "A-0900" || rows[2][index["event_id"]] != "A-1400" || rows[2][index["event_time"]] != "14:00" {
		t.Fatal("same-day events merged or time lost")
	}
	if rows[1][index["value"]] != "0" || rows[2][index["value"]] != "" || rows[2][index["missing_reason"]] != "not_recorded" {
		t.Fatal("zero and missing values confused")
	}
}
