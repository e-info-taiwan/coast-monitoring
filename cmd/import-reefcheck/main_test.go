package main

import "testing"

func TestImportRequiresExplicitNumericSourceConvention(t *testing.T) {
	for _, tc := range []struct {
		code, format, want string
		valid              bool
	}{{"0", "canonical", "", false}, {"10", "canonical", "", false}, {"0", "entry0", "OT", true}, {"10", "legacy10", "OT", true}, {"0", "legacy10", "", false}, {"HC-a", "canonical", "HC-a", true}} {
		got, err := normalizeImportSubstrate(tc.code, tc.format)
		if (err == nil) != tc.valid || got != tc.want {
			t.Fatalf("%+v => %s %v", tc, got, err)
		}
	}
}
func TestImportNAAndBlankAreNotObservedZero(t *testing.T) {
	for _, v := range []string{"NA", "-"} {
		n, status, err := parseImportCount(v, "")
		if err != nil || n != 0 || status != "not_recorded" {
			t.Fatal(v, status, err)
		}
	}
	if _, _, err := parseImportCount("", ""); err == nil {
		t.Fatal("blank accepted")
	}
	n, status, err := parseImportCount("0", "")
	if err != nil || n != 0 || status != "recorded" {
		t.Fatal(n, status, err)
	}
}
