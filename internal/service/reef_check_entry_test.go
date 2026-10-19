package service

import (
	"testing"
	"time"
)

func tabletFixture() ReefCheckSurveySubmission {
	s := ReefCheckSurveySubmission{Format: "reefcheck-web-v4", ActivityCode: "RC-TEST", BleachingMode: "point_marks", Event: ReefCheckEventInput{SiteID: 1, SurveyDate: "2026-10-14", EventTime: "09:30", DepthM: 5}, Transects: []ReefCheckTransectInput{{TransectKey: "line", Method: "line", Recorders: []string{"志工"}, CoralMorphology: "general"}}}
	for seg := 1; seg <= 4; seg++ {
		for i := 0; i < 40; i++ {
			s.SubstratePoints = append(s.SubstratePoints, ReefCheckSubstratePointInput{TransectKey: "line", Segment: seg, PositionM: float64((seg-1)*25) + float64(i)/2, SubstrateCode: "HC", SubstrateLayer: "surface", IsBleached: dataPtr(false)})
		}
	}
	return s
}
func TestTabletEntryRequiredModesAndPoints(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ReefCheckSurveySubmission)
		valid  bool
	}{
		{"complete", func(s *ReefCheckSurveySubmission) {}, true},
		{"missing activity", func(s *ReefCheckSurveySubmission) { s.ActivityCode = "" }, false},
		{"missing recorder", func(s *ReefCheckSurveySubmission) { s.Transects[0].Recorders = nil }, false},
		{"missing morphology", func(s *ReefCheckSurveySubmission) { s.Transects[0].CoralMorphology = "" }, false},
		{"classified subtype", func(s *ReefCheckSurveySubmission) {
			s.Transects[0].CoralMorphology = "classified"
			s.SubstratePoints[0].SubstrateCode = "HC-a"
		}, true},
		{"unexpected subtype", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[0].SubstrateCode = "HC-a" }, false},
		{"missing mark", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[0].IsBleached = nil }, false},
		{"one missing point", func(s *ReefCheckSurveySubmission) { s.SubstratePoints = s.SubstratePoints[:159] }, false},
		{"duplicate position", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[1].PositionM = 0 }, false},
		{"wrong segment coordinate", func(s *ReefCheckSurveySubmission) { s.SubstratePoints[0].PositionM = 20 }, false},
		{"paired coordinates", func(s *ReefCheckSurveySubmission) {
			s.EntryMetadata.Latitude = dataPtr(25.0)
			s.EntryMetadata.Longitude = dataPtr(121.5)
		}, true},
		{"unpaired coordinates", func(s *ReefCheckSurveySubmission) { s.EntryMetadata.Latitude = dataPtr(25.0) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tabletFixture()
			tc.change(&s)
			err := s.Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("got %v,valid=%v", err, tc.valid)
			}
		})
	}
}
func TestTabletRKCThresholdUsesValidPoints(t *testing.T) {
	for _, tc := range []struct {
		rkc, na int
		note    string
		valid   bool
	}{{15, 0, "", true}, {16, 0, "", false}, {16, 0, "無法判斷", true}, {15, 10, "", false}, {0, 160, "", false}, {0, 160, "未記錄", true}} {
		s := tabletFixture()
		for i := 0; i < tc.rkc; i++ {
			s.SubstratePoints[i].SubstrateCode = "RKC"
			s.SubstratePoints[i].IsBleached = nil
		}
		for i := 160 - tc.na; i < 160; i++ {
			s.SubstratePoints[i].SubstrateCode = "NA"
			s.SubstratePoints[i].IsBleached = nil
		}
		s.RKCNote = tc.note
		if tc.na > 0 && tc.note != "" {
			s.MissingReason = tc.note
		}
		err := s.Validate()
		if (err == nil) != tc.valid {
			t.Fatalf("RKC=%d NA=%d note=%q: %v", tc.rkc, tc.na, tc.note, err)
		}
	}
}
func TestActivityDateSiteAndExpiryScope(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	expiry := now.Add(time.Hour)
	a := ReefCheckActivity{Active: true, StartDate: "2026-10-14", EndDate: "2026-10-16", SiteIDs: []int{1, 2}, ExpiresAt: &expiry}
	if !a.Allows(1, "2026-10-14", now) || !a.Allows(2, "2026-10-16", now) {
		t.Fatal("inclusive date endpoints rejected")
	}
	if a.Allows(3, "2026-10-14", now) || a.Allows(1, "2026-10-17", now) || a.Allows(1, "2026-10-14", expiry) {
		t.Fatal("outside scope accepted")
	}
	a.Active = false
	if a.Allows(1, "2026-10-14", now) {
		t.Fatal("disabled activity accepted")
	}
	input := ReefCheckActivityInput{Name: "活動", StartDate: "2026-10-14", EndDate: "2026-10-16", SiteIDs: []int{1, 1}}
	if input.Validate() == nil {
		t.Fatal("duplicate site accepted")
	}
}
