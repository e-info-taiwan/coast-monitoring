package service

import (
	"errors"
	"testing"
)

func dataPtr[T any](v T) *T { return &v }
func dataFixture() ReefDataTransect {
	return ReefDataTransect{ID: 1, Method: "belt_invert", Belt: []ReefDataBelt{{ID: 10, Group: "invert", Count: 0}}, Impacts: []ReefDataImpact{{ID: 20, ValueType: "level", HasRawCount: true, RawValue: 8}, {ID: 21, ValueType: "percent", RawValue: 12.5}, {ID: 22, ValueType: "level"}}}
}
func TestReefDataValidationPreservesRawCountsAndPercentScale(t *testing.T) {
	current := dataFixture()
	for _, test := range []struct {
		name   string
		change ReefDataChange
		valid  bool
	}{
		{"raw count greater than grade ceiling", ReefDataChange{Kind: "impact", ID: 20, Value: dataPtr(8.0)}, true},
		{"fractional percent", ReefDataChange{Kind: "impact", ID: 21, Value: dataPtr(12.5)}, true},
		{"percent maximum", ReefDataChange{Kind: "impact", ID: 21, Value: dataPtr(100.0)}, true},
		{"percent overflow", ReefDataChange{Kind: "impact", ID: 21, Value: dataPtr(100.1)}, false},
		{"fractional raw count", ReefDataChange{Kind: "impact", ID: 20, Value: dataPtr(1.5)}, false},
		{"negative count", ReefDataChange{Kind: "belt", ID: 10, Value: dataPtr(-1.0)}, false},
		{"missing count", ReefDataChange{Kind: "belt", ID: 10}, false},
		{"observed zero", ReefDataChange{Kind: "belt", ID: 10, Value: dataPtr(0.0)}, true},
		{"other transect row", ReefDataChange{Kind: "belt", ID: 999, Value: dataPtr(1.0)}, false},
		{"grade ceiling", ReefDataChange{Kind: "impact", ID: 22, Value: dataPtr(4.0)}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := (ReefDataUpdate{Version: current.Revision(), Changes: []ReefDataChange{test.change}}).Validate(current, nil)
			if (err == nil) != test.valid {
				t.Fatalf("validation=%v valid=%v", err, test.valid)
			}
		})
	}
}
func TestReefDataRevisionAndMethodValidation(t *testing.T) {
	current := dataFixture()
	revision := current.Revision()
	current.Impacts[0].RawValue = 9
	if err := (ReefDataUpdate{Version: revision}).Validate(current, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	current.Method = "belt_fish"
	u := ReefDataUpdate{Version: current.Revision(), Changes: []ReefDataChange{{Kind: "impact", ID: 20, Value: dataPtr(1.0)}}}
	if err := u.Validate(current, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("wrong method accepted: %v", err)
	}
	current.Belt[0].Group = "fish"
	current.Belt[0].Aggregate = true
	u = ReefDataUpdate{Version: current.Revision(), Changes: []ReefDataChange{{Kind: "belt", ID: 10, Value: dataPtr(1.0)}}}
	if err := u.Validate(current, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("aggregate edit accepted: %v", err)
	}
}
func TestReefDataLineCodesAndNullableMetadata(t *testing.T) {
	current := ReefDataTransect{ID: 1, Method: "line", Points: []ReefDataPoint{{ID: 1, Code: "NA", Layer: "surface"}}, Bleaching: []ReefDataBleaching{{ID: 2}}}
	codes := []ReefDataCode{{Code: "NA", Active: true}, {Code: "OT", Active: true}, {Code: "SI(HC)", Active: true}}
	for _, code := range []string{"NA", "OT", "SI(HC)"} {
		u := ReefDataUpdate{Version: current.Revision(), Metadata: &ReefDataMetadata{}, Changes: []ReefDataChange{{Kind: "point", ID: 1, Code: code}}}
		if err := u.Validate(current, codes); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []ReefDataMetadata{{WaterTemp: dataPtr(41.0)}, {VisibilityMin: dataPtr(5.0), VisibilityMax: dataPtr(2.0)}} {
		if err := (ReefDataUpdate{Version: current.Revision(), Metadata: &m}).Validate(current, codes); err == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
}
