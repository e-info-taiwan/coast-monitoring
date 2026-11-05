package httpx

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"coast-monitoring/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type prdPublicReefFake struct {
	rows      []service.PublicReefSeries
	filters   []service.PublicReefFilter
	err       error
	deadlines []time.Time
}

func (f *prdPublicReefFake) PublicSites(context.Context, service.PublicReefFilter) ([]json.RawMessage, error) {
	return []json.RawMessage{}, nil
}
func (f *prdPublicReefFake) SetPublication(context.Context, int, string, string) error { return nil }
func (f *prdPublicReefFake) PublicSeries(ctx context.Context, _ int, filter service.PublicReefFilter) ([]service.PublicReefSeries, error) {
	f.filters = append(f.filters, filter)
	deadline, _ := ctx.Deadline()
	f.deadlines = append(f.deadlines, deadline)
	return f.rows, f.err
}

func TestPRD20261104C09SeriesCSVPreservesEventsAndMissingValues(t *testing.T) {
	zero := 0.0
	morning, afternoon := "09:00", "14:00"
	fake := &prdPublicReefFake{rows: []service.PublicReefSeries{
		{SiteID: 1, EventID: "A-0900", Date: "2026-11-04", Time: &morning, Depth: 5, Chart: "taxa", Value: &zero, N: 4, Status: "ok", Unit: "individuals_per_100m2"},
		{SiteID: 1, EventID: "A-1400", Date: "2026-11-04", Time: &afternoon, Depth: 5, Chart: "taxa", N: 0, MissingReason: "not_recorded", Unit: "individuals_per_100m2"},
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

func TestPublicStatisticsTimeoutsAreNotEmptySuccess(t *testing.T) {
	for _, failure := range []error{context.DeadlineExceeded, &pgconn.PgError{Code: "57014", Message: "statement timeout"}} {
		for _, path := range []string{"/sites/102/series", "/sites/102/export.csv", "/compare?site_ids=102,151", "/compare.csv?site_ids=102,151"} {
			fake := &prdPublicReefFake{err: failure}
			h := &PublicHandlers{Reef: fake}
			router := chi.NewRouter()
			router.Get("/sites/{id}/series", h.Series)
			router.Get("/sites/{id}/export.csv", h.Series)
			router.Get("/compare", h.Compare)
			router.Get("/compare.csv", h.Compare)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
			if response.Code != 504 || !strings.Contains(response.Body.String(), "圖表資料查詢逾時") {
				t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
			}
			if len(fake.deadlines) != 1 || time.Until(fake.deadlines[0]) > 20*time.Second || time.Until(fake.deadlines[0]) < 19*time.Second {
				t.Fatalf("missing shared request deadline: %v", fake.deadlines)
			}
		}
	}
}

func TestPublicStatisticsCompareAndCSVContract(t *testing.T) {
	zero := 0.0
	fake := &prdPublicReefFake{rows: []service.PublicReefSeries{{SiteID: 102, EventID: "event", Chart: "substrate", Key: "HC", Name: "硬珊瑚", Value: &zero, Mean: &zero, N: 1, Unit: "percent", UnitLabel: "%", UpdatedAt: "2026-11-05T01:00:00Z", Status: "ok"}}}
	h := &PublicHandlers{Reef: fake}
	router := chi.NewRouter()
	router.Get("/compare", h.Compare)
	router.Get("/compare.csv", h.Compare)
	for _, path := range []string{"/compare?site_ids=102,151", "/compare.csv?site_ids=102,151"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d", path, response.Code)
		}
		if strings.Contains(path, ".csv") {
			rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(response.Body.String(), "\ufeff"))).ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 3 || len(rows[0]) != 25 {
				t.Fatalf("CSV dimensions: %v", rows)
			}
			index := map[string]int{}
			for i, k := range rows[0] {
				index[k] = i
			}
			for _, k := range []string{"event_time", "size_class", "fish_size_mode", "taxon_id", "sd", "se", "density_per_100m2"} {
				if rows[1][index[k]] != "" {
					t.Fatalf("CSV null %s", k)
				}
			}
			if rows[1][index["value"]] != "0" || rows[1][index["data_updated_at"]] != fake.rows[0].UpdatedAt {
				t.Fatal("CSV values lost")
			}
		} else {
			var data struct {
				Sites []struct {
					Availability bool             `json:"availability"`
					Series       []map[string]any `json:"series"`
				} `json:"sites"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
				t.Fatal(err)
			}
			if len(data.Sites) != 2 || !data.Sites[0].Availability || len(data.Sites[0].Series) != 1 {
				t.Fatal("comparison shape")
			}
			for _, k := range []string{"event_time", "size_class", "fish_size_mode", "taxon_id"} {
				if v, ok := data.Sites[0].Series[0][k]; !ok || v != nil {
					t.Fatalf("JSON missing null %s: %v", k, v)
				}
			}
		}
	}
	if len(fake.deadlines) != 4 || !fake.deadlines[0].Equal(fake.deadlines[1]) || !fake.deadlines[2].Equal(fake.deadlines[3]) {
		t.Fatal("comparison must share one deadline across sites")
	}
}
