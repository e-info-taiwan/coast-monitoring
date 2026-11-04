package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Optional full-delivery check: no database connection or external services.
func TestHandoffNumericCorpus(t *testing.T) {
	dir := os.Getenv("REEF_HANDOFF_CSV_DIR")
	if dir == "" {
		t.Skip("set REEF_HANDOFF_CSV_DIR for the delivery corpus")
	}
	rows, err := readCSV(filepath.Join(dir, "impact_observation.csv"))
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		if _, _, _, err := parseImportImpact(r["raw_value"], r["record_status"], r["source_unit"], r["impact_group"]); err != nil {
			t.Fatalf("impact row %d: %v", i+2, err)
		}
	}
	for table, fields := range map[string][]string{"belt_observation": {"count"}, "substrate_bleaching": {"hc_bleached_count", "sc_bleached_count"}, "incidental_sighting": {"count"}} {
		rows, err := readCSV(filepath.Join(dir, table+".csv"))
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range rows {
			for _, field := range fields {
				if _, _, err := parseNullableImportCount(r[field], ""); err != nil {
					t.Fatalf("%s row %d: %v", table, i+2, err)
				}
			}
		}
	}
}

func TestNullableHandoffValues(t *testing.T) {
	for _, v := range []string{"", "NA", "-"} {
		n, s, err := parseNullableImportCount(v, "")
		if err != nil || n != nil || s != "not_recorded" {
			t.Fatalf("missing %q became observed: %v %s %v", v, n, s, err)
		}
	}
	n, s, err := parseNullableImportCount("0", "")
	if err != nil || n == nil || *n != 0 || s != "recorded" {
		t.Fatal("observed zero lost")
	}
	for _, v := range []string{"-1", "0.5", "NaN", "2147483648"} {
		if _, _, err := parseNullableImportCount(v, ""); err == nil {
			t.Fatalf("accepted %s", v)
		}
	}
}

func TestHistoricalImpactUnits(t *testing.T) {
	for _, tc := range []struct {
		value, unit, group string
		valid              bool
	}{
		{"3", "level", "trash", true}, {"5", "level", "trash", false},
		{"8", "count", "trash", true}, {"1.5", "count", "coral_damage", false},
		{"2.5", "percent", "bleaching", true}, {"101", "percent", "disease", false},
		{"1", "percent", "trash", false}, {"1", "count", "disease", false},
		{"NaN", "count", "trash", false}, {"-1", "count", "trash", false},
		{"", "level", "trash", true},
	} {
		v, status, unit, err := parseImportImpact(tc.value, "", tc.unit, tc.group)
		if (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
		if tc.valid && unit != tc.unit {
			t.Fatal("source unit changed")
		}
		if tc.valid && tc.value == "" && (v != nil || status != "not_recorded") {
			t.Fatal("NULL became zero")
		}
		if tc.valid && tc.value == "3" && (v == nil || *v != 3) {
			t.Fatal("historical level was converted")
		}
	}
}
