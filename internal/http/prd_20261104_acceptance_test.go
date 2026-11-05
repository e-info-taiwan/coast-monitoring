package httpx

import (
	"encoding/csv"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPRD20261104C09CSVTaxonomyColumns(t *testing.T) {
	rec := httptest.NewRecorder()
	writeReefCSV(rec, nil)
	header, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\ufeff"))).Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"taxon_id", "taxon_group", "size_class", "fish_size_mode"} {
		found := false
		for _, column := range header {
			if column == key {
				found = true
			}
		}
		if !found {
			t.Errorf("C09: CSV missing explicit %s column", key)
		}
	}
}

func TestPRD20261104C10RejectSameNumericSite(t *testing.T) {
	fake := &prdPublicReefFake{}
	h := &PublicHandlers{Reef: fake}
	rec := httptest.NewRecorder()
	h.Compare(rec, httptest.NewRequest("GET", "/api/public/reef-check/compare?site_ids=1,01", nil))
	if rec.Code != 400 {
		t.Fatalf("C10: two spellings of site 1 accepted: status %d", rec.Code)
	}
	if len(fake.filters) != 0 {
		t.Fatal("invalid comparison queried data")
	}
}
