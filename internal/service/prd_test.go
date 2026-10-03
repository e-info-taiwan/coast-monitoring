package service

import (
	"encoding/json"
	"net/url"
	"testing"
)

func validSubmission() ReefCheckSurveySubmission {
	s := ReefCheckSurveySubmission{Event: ReefCheckEventInput{SiteID: 1, SurveyDate: "2026-10-02", EventTime: "09:30", DepthM: 5}, Transects: []ReefCheckTransectInput{{TransectKey: "line", Method: "line", Recorders: []string{"Recorder"}, ClassifyHardCoral: true}}}
	for seg := 1; seg <= 4; seg++ {
		for i := 0; i < 40; i++ {
			s.SubstratePoints = append(s.SubstratePoints, ReefCheckSubstratePointInput{TransectKey: "line", Segment: seg, PositionM: float64((seg-1)*25) + float64(i)/2, SubstrateCode: "a", SubstrateLayer: "surface"})
		}
	}
	return s
}
func TestPRDSubmissionContract(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ReefCheckSurveySubmission)
		valid  bool
	}{
		{"zero means OT", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[0].SubstrateCode = "0" }, true},
		{"historical 10 requires import convention", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[0].SubstrateCode = "10" }, false},
		{"blank cannot become NA", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[0].SubstrateCode = "" }, false},
		{"duplicate position", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[1] = s.SubstratePoints[0] }, false},
		{"missing position", func(s *ReefCheckSurveySubmission) { s.SubstratePoints = s.SubstratePoints[:159] }, false},
		{"NA needs reason", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[0].SubstrateCode = "NA" }, false},
		{"NA reason", func(s *ReefCheckSurveySubmission) {
			s.SubstratePoints[0].SubstrateCode = "NA"
			s.MissingReason = "not on original board"
		}, true},
		{"HC shape counts toward bleaching denominator", func(s *ReefCheckSurveySubmission) {
			n := 40
			s.SubstrateBleaching = []ReefCheckBleachingInput{{TransectKey: "line", Segment: 1, SubstrateCode: "HC", BleachedPoints: &n}}
		}, true},
		{"bleaching greater than substrate", func(s *ReefCheckSurveySubmission) {
			n := 1
			s.SubstrateBleaching = []ReefCheckBleachingInput{{TransectKey: "line", Segment: 1, SubstrateCode: "SC", BleachedPoints: &n}}
		}, false},
		{"fish mode required", func(s *ReefCheckSurveySubmission) {
			s.Transects = append(s.Transects, ReefCheckTransectInput{TransectKey: "fish", Method: "belt_fish", Recorders: []string{"Recorder"}})
		}, false},
		{"explicit combined mode", func(s *ReefCheckSurveySubmission) {
			s.Transects = append(s.Transects, ReefCheckTransectInput{TransectKey: "fish", Method: "belt_fish", FishSizeMode: "combined", Recorders: []string{"Recorder"}})
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := validSubmission()
			tc.mutate(&s)
			err := s.Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if err == nil && s.Status != "draft" {
				t.Fatal("not draft")
			}
		})
	}
}
func TestPublicFiltersRejectNonFiniteOrAmbiguousInputs(t *testing.T) {
	for _, q := range []string{"depth_m=NaN", "depth_m=Inf", "from_year=2027&to_year=2026", "charts=private", "taxon_ids=-1", "method=unknown"} {
		v, _ := url.ParseQuery(q)
		if _, e := ParsePublicReefFilter(v); e == nil {
			t.Fatal(q)
		}
	}
}
func TestContentGeometryAndMediaContract(t *testing.T) {
	c := PublicContentInput{Kind: "development", Data: ContentData{Title: "Fixture", Geometry: json.RawMessage(`{"type":"Polygon","coordinates":[[[120,23],[121,23],[121,24],[120,23]]]}`)}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Data.Media = []ContentMedia{{URL: "https://example.com/image.png", Alt: "site", Published: true, Valid: true}}
	if err := c.Validate(); err == nil {
		t.Fatal("unlicensed image published")
	}
	c.Data.Media[0].Licensed = true
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Data.Geometry = json.RawMessage(`{"type":"Polygon","coordinates":[[[120,23],[121,23],[121,24],[120,24]]]}`)
	if err := c.Validate(); err == nil {
		t.Fatal("unclosed polygon")
	}
	c.Data.Geometry = json.RawMessage(`{"type":"Point","coordinates":[181,23]}`)
	if err := c.Validate(); err == nil {
		t.Fatal("invalid coordinate")
	}
}
