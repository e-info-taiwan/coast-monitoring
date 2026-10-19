package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootServesReefCheckHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, `src="/public/reef-check/app.js"`) {
		t.Fatalf("body does not look like Reef Check HTML")
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

func TestPublicStaticRouteServesReefCheckVolunteerForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/public/reef-check/", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	body := rec.Body.String()
	for _, want := range []string{`<title>珊瑚礁體檢｜志工記錄站</title>`, `src="/public/reef-check/app.js"`, `href="/public/reef-check/theme.css"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("reef check volunteer form missing marker %q", want)
		}
	}
}

func TestUnknownAPIRouteDoesNotServeAdminHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); strings.Contains(body, `src="/site/app.js?v=20261003"`) {
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

func TestReefDataUIHasExpectedFeatures(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(staticDir("web/admin"), "reef-data.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"createReefData",
		"rd-create-event",
		"rd-delete-event",
		"rd-add-participant-btn",
		"rd-remove-participant",
		"/admin/reef-check-data/events",
		"/admin/reef-check-data/participants",
		"rd-user-badge",
		"methods",
		"roles",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("reef-data.js missing modern Reef Check marker %q", want)
		}
	}
}

func TestVolunteerFormUsesV18EntryRules(t *testing.T) {
	dir := staticDir("web/public/reef-check")
	appBody, err := os.ReadFile(filepath.Join(dir, "app.js"))
	if err != nil {
		t.Fatal(err)
	}
	catalogBody, err := os.ReadFile(filepath.Join(dir, "catalog.js"))
	if err != nil {
		t.Fatal(err)
	}
	app := string(appBody)
	catalog := string(catalogBody)
	for _, want := range []string{
		"if(t==='0')return 'OT'",
		"HC-${t.toLowerCase()}",
		"fish_length_mode:k==='fish'&&state.fishLengthMode?state.fishLengthMode:undefined",
		"r.group==='trash'||r.group==='coral_damage'",
		"data-action=\"toggle-bleach-point\"",
		"is_bleached:isBleachableCode",
		"new URLSearchParams(window.location.search).get('debug')==='1'",
		"DEBUG · 未儲存",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("volunteer app missing v1.8 rule %q", want)
		}
	}
	for _, want := range []string{"['HC-a'", "['HC-b'", "['HC-c'", "'count','coral_damage'", "'count','trash'"} {
		if !strings.Contains(catalog, want) {
			t.Fatalf("volunteer catalog missing v1.8 rule %q", want)
		}
	}
	if strings.Contains(app, "/^(?:[1-9]|10)$/") {
		t.Fatal("volunteer app still accepts legacy numeric 10 as new OT input")
	}
}
