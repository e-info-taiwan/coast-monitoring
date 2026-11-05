package service

import (
	"encoding/json"
	"reflect"
	"testing"
)

// These target-spec tests deliberately remain red while PRD 0.9.13 / Schema
// 1.16 features are missing. Run explicitly; see docs/prd-test-cases-2026-11-04.md.
func TestPRD20261104H01IntroPopupSurvivesContentParsing(t *testing.T) {
	raw := `{"kind":"settings","data":{"title":"首頁設定","article_limit":10,"intro_popup":{"kicker":"珊瑚觀測","title":"認識海洋","body":"第一行\n第二行","steps":["選擇樣點","查看資料"],"button_label":"開始探索"}}}`
	var input PublicContentInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatal(err)
	}
	if err := input.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var before, after map[string]any
	if err := json.Unmarshal([]byte(raw), &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &after); err != nil {
		t.Fatal(err)
	}
	want := before["data"].(map[string]any)["intro_popup"]
	got := after["data"].(map[string]any)["intro_popup"]
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("H01: intro_popup lost or changed during parsing: got %#v, want %#v", got, want)
	}
}

func TestPRD20261104B04ArticleCategories(t *testing.T) {
	for _, category := range []string{"news", "observation_report", "unsupported"} {
		t.Run(category, func(t *testing.T) {
			var input PublicContentInput
			raw := `{"kind":"article","data":{"title":"觀測紀錄","external_url":"https://example.org/report","date":"2026-11-04","category":"` + category + `"}}`
			if err := json.Unmarshal([]byte(raw), &input); err != nil {
				t.Fatal(err)
			}
			err := input.Validate()
			if category == "unsupported" {
				if err == nil {
					t.Fatal("B04: unknown article category accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(input.Data)
			if err != nil {
				t.Fatal(err)
			}
			var data map[string]any
			if err := json.Unmarshal(encoded, &data); err != nil {
				t.Fatal(err)
			}
			if data["category"] != category {
				t.Fatalf("B04: category lost: got %v, want %s", data["category"], category)
			}
		})
	}
}

func TestPRD20261104B06UnknownReportDate(t *testing.T) {
	var input PublicContentInput
	if err := json.Unmarshal([]byte(`{"kind":"article","data":{"title":"日期未知的報告","external_url":"https://example.org/report","category":"observation_report"}}`), &input); err != nil {
		t.Fatal(err)
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("B06: unknown original publication date must remain unknown: %v", err)
	}
	if input.Data.Date != "" {
		t.Fatal("B06: validation fabricated a publication date")
	}
}

func TestPRD20261104H02SettingsAreGlobal(t *testing.T) {
	limit, site := 10, 1
	input := PublicContentInput{Kind: "settings", SiteID: &site, Data: ContentData{Title: "首頁設定", ArticleLimit: &limit}}
	if err := input.Validate(); err == nil {
		t.Fatal("H02 / Schema 9.9: site-specific global settings accepted")
	}
}

func TestPRD20261104C09ExplicitTaxonomyInSeries(t *testing.T) {
	// Exercise the actual public response type, not a parallel test-only DTO.
	var row PublicReefSeries
	if err := json.Unmarshal([]byte(`{"site_id":1,"chart":"taxa","key":"14","taxon_id":14,"taxon_group":"fish","size_class":"30-40cm","fish_size_mode":"split"}`), &row); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{"taxon_id": float64(14), "taxon_group": "fish", "size_class": "30-40cm", "fish_size_mode": "split"} {
		if got[key] != want {
			t.Errorf("C09: %s = %v, want %v", key, got[key], want)
		}
	}
}
