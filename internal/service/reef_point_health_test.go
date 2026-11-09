package service

import "testing"

func TestPointHealthStatusValidationAndLegacyCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		status     *string
		flag       *bool
		valid      bool
	}{
		{"new positive", "HC", dataPtr("bleached"), nil, true},
		{"new confirmed negative", "SC", dataPtr("not_bleached"), nil, true},
		{"legacy positive", "HC", nil, dataPtr(true), true},
		{"legacy negative", "HC", nil, dataPtr(false), true},
		{"matching old and new", "HC", dataPtr("bleached"), dataPtr(true), true},
		{"conflicting fields", "HC", dataPtr("bleached"), dataPtr(false), false},
		{"not a health diagnosis", "HC", dataPtr("healthy"), nil, false},
		{"empty is not negative", "HC", dataPtr(""), nil, false},
		{"unknown coral", "HC", nil, nil, false},
		{"not applicable", "RC", nil, nil, true},
		{"noncoral cannot mark", "RC", dataPtr("not_bleached"), nil, false},
		{"mud composite excluded", "SI(HC)", dataPtr("bleached"), nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tabletFixture()
			p := &s.SubstratePoints[0]
			p.SubstrateCode = tc.code
			p.HealthStatus = tc.status
			p.IsBleached = tc.flag
			err := s.Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
			if err == nil && p.HealthStatus != nil {
				if p.IsBleached == nil || *p.IsBleached != (*p.HealthStatus == HealthStatusBleached) {
					t.Fatal("health and legacy flag diverged")
				}
			}
		})
	}
}
