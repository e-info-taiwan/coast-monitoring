package service

import (
	"encoding/json"
	"testing"
)

func TestPRD20261104H02ArticleLimitBoundaries(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"1", true}, {"50", true}, {"0", false}, {"51", false}, {"1.5", false}, {`"10"`, false}, {"null", false}} {
		t.Run(tc.value, func(t *testing.T) {
			var input PublicContentInput
			err := json.Unmarshal([]byte(`{"kind":"settings","data":{"title":"首頁設定","article_limit":`+tc.value+`}}`), &input)
			if err == nil {
				err = input.Validate()
			}
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestPRD20261104B10AnnotationProvenance(t *testing.T) {
	site := 1
	for _, tc := range []struct {
		name   string
		mutate func(*PublicContentInput)
		valid  bool
	}{
		{"valid", func(*PublicContentInput) {}, true},
		{"missing site", func(c *PublicContentInput) { c.SiteID = nil }, false},
		{"missing date", func(c *PublicContentInput) { c.Data.Date = "" }, false},
		{"invalid date", func(c *PublicContentInput) { c.Data.Date = "2026-02-30" }, false},
		{"missing source", func(c *PublicContentInput) { c.Data.SourceURL = "" }, false},
		{"unsafe source", func(c *PublicContentInput) { c.Data.SourceURL = "javascript:alert(1)" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := PublicContentInput{Kind: "annotation", SiteID: &site, Data: ContentData{Title: "事件註記", Date: "2026-11-04", SourceURL: "https://example.org/source"}}
			tc.mutate(&c)
			if err := c.Validate(); (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
