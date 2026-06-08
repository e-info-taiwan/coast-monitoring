package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootServesAdminHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `src="/app.js"`) {
		t.Fatalf("body does not look like admin HTML")
	}
}

func TestPublicStaticRouteServesPublicHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/public/species.html", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `src="/public/species.js"`) {
		t.Fatalf("body does not look like public species HTML")
	}
}

func TestUnknownAPIRouteDoesNotServeAdminHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); strings.Contains(body, `src="/app.js"`) {
		t.Fatalf("unknown API route served admin HTML")
	}
}

func TestAdminStaticAssetsDoNotExposeDefaultCredentials(t *testing.T) {
	adminDir := staticDir("web/admin")
	for _, name := range []string{"index.html", "app.js"} {
		path := filepath.Join(adminDir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		text := string(body)
		for _, forbidden := range []string{"hcchien@gmail.com", "Test1234!"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s exposes default credential hint %q", path, forbidden)
			}
		}
	}
}

func TestObservationDateChoicesDoNotOfferTomorrow(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(staticDir("web/admin"), "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, forbidden := range []string{`key: "tomorrow"`, "明天"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("admin app exposes future observation date choice %q", forbidden)
		}
	}
	for _, want := range []string{`key: "yesterday"`, `key: "today"`, "昨天", "今天"} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin app missing date choice %q", want)
		}
	}
}

func TestReefCheckSubstrateUIUsesFieldSheetLayout(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(staticDir("web/admin"), "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"renderReefCheckSubstrateSheetGrid",
		"rc-field-sheet-grid",
		"rc-field-sheet-row",
		"Site name",
		"Country/Island",
		"TS/TL",
		"Time",
		"Visibility",
		"Temperature",
		"Data recorded by",
		"What percentage of recorded RKC is a result of bleaching?",
		"rkcBleachingPercent",
		"substrateComments",
		"0 - 19.5 m",
		"25 - 44.5 m",
		"50 - 69.5 m",
		"75 - 94.5 m",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin app missing Reef Check field sheet UI marker %q", want)
		}
	}
}

func TestReefCheckUIUsesSwitchableFieldSheets(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(staticDir("web/admin"), "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"reefCheckActiveSheet",
		"renderReefCheckSheetTabs",
		"bindReefCheckSheetTabs",
		"data-rc-sheet-tab",
		"data-rc-sheet-panel",
		"renderReefCheckFishFieldSheet",
		"renderReefCheckInvertsFieldSheet",
		"FIELD SHEET FISH",
		"FIELD SHEET INVERTS",
		"Butterflyfish",
		"Grouper",
		"Rare animals sighted (#/type/size)",
		"Giant clam",
		"Impacts: Coral Damage/Disease/Bleaching/Trash",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin app missing Reef Check sheet switching marker %q", want)
		}
	}
}
