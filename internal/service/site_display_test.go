package service

import (
	"math"
	"reflect"
	"testing"
)

func TestEffectiveChartsInheritPerFieldAndPreserveEmptySelection(t *testing.T) {
	hidden := false
	rows := EffectiveChartSettings(4, []ChartSetting{{ChartKey: "taxa_density", Status: "published", Visible: &hidden, TaxonIDs: []int{}}, {ChartKey: "substrate_depth", Status: "draft", Visible: &hidden}})
	if len(rows) != 6 {
		t.Fatal(rows)
	}
	for _, row := range rows {
		if row.ChartKey == "taxa_density" {
			if *row.Visible || row.Source != "site" || row.TaxonIDs == nil {
				t.Fatal(row)
			}
		} else if !*row.Visible || row.Source != "default" {
			t.Fatal(row)
		}
	}
	id := 3
	if ChartAllows(ChartSetting{TaxonIDs: []int{}}, PublicReefSeries{Chart: "taxa", TaxonID: &id, Date: "2026-01-01"}) {
		t.Fatal("empty selection became all taxa")
	}
	if !ChartAllows(ChartSetting{}, PublicReefSeries{Chart: "taxa", TaxonID: &id, Date: "2026-01-01"}) {
		t.Fatal("null must inherit")
	}
}
func TestSharedGroupPreservesMultipleDepthsAndRemovedSites(t *testing.T) {
	g := SharedChartGroup{Title: "G", Members: []SharedChartMember{{SiteID: 1, Depth: 5}, {SiteID: 1, Depth: 10}, {SiteID: 2, Depth: 10}}}
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	next := []SharedChartMember{{SiteID: 1, Depth: 5}, {SiteID: 3, Depth: 10}}
	if !reflect.DeepEqual(AffectedSites(g.Members, next), []int{1, 2, 3}) {
		t.Fatal("removed member missing")
	}
	g.Members = append(g.Members, g.Members[0])
	if g.Validate() == nil {
		t.Fatal("duplicate member accepted")
	}
	g.Members = next
	g.Members[0].Depth = math.NaN()
	if g.Validate() == nil {
		t.Fatal("invalid depth accepted")
	}
}
func TestRareChartUsesExplicitTaxonGroup(t *testing.T) {
	if SeriesChartKey(PublicReefSeries{Chart: "taxa", Name: "fish", TaxonGroup: "rare"}) != "rare_history" {
		t.Fatal("name inferred group")
	}
}
