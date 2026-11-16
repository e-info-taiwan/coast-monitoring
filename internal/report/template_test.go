package report

import (
	"archive/zip"
	"bytes"
	"coast-monitoring/internal/service"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func templateFixture() service.ReefDataDetail {
	e := service.ReefDataEvent{ID: 42, SiteID: 1, SiteName: "測試海岸", SiteEnglish: "Test Coast", SurveyDate: "2025-12-12", EventTime: "09:30", DepthM: 5, EventID: "Test_2025_12_12_0930_5"}
	line := service.ReefDataTransect{Method: "line", CoralMorphology: "classified"}
	hc := [4]int{2, 13, 9, 11}
	for seg, n := range hc {
		for i := 0; i < 40; i++ {
			code := "RC"
			if i < n {
				code = "HC-c"
			}
			if seg == 2 && i == 22 { // 61 m, right column of the third segment
				code = "OT"
			}
			line.Points = append(line.Points, service.ReefDataPoint{Segment: seg + 1, Position: float64(seg*25) + float64(i)/2, Layer: "surface", Code: code})
		}
	}
	fish := service.ReefDataTransect{Method: "belt_fish"}
	for seg, n := range []int{5, 8, 12, 7} {
		fish.Belt = append(fish.Belt, service.ReefDataBelt{Segment: seg + 1, TaxonID: 1, NameEN: "Butterflyfish", Size: ">=5cm", Count: n}, service.ReefDataBelt{Segment: seg + 1, TaxonID: 2, NameEN: "Butterflyfish", Size: "<5cm", Count: []int{4, 0, 1, 0}[seg]})
	}
	invert := service.ReefDataTransect{Method: "belt_invert"}
	for seg, n := range []int{12, 5, 5, 9} {
		invert.Belt = append(invert.Belt, service.ReefDataBelt{Segment: seg + 1, TaxonID: 3, NameEN: "Diadema", Count: n}, service.ReefDataBelt{Segment: seg + 1, TaxonID: 4, NameEN: "Echinothrix", Count: 1})
		invert.Impacts = append(invert.Impacts, service.ReefDataImpact{Segment: seg + 1, NameEN: "Trash: general", HasRawCount: true, ValueType: "count", RawValue: float64(seg)})
	}
	return service.ReefDataDetail{Event: e, Transects: []service.ReefDataTransect{line, fish, invert}}
}
func TestReferenceTemplateWorkbooks(t *testing.T) {
	d := templateFixture()
	for _, kind := range []string{"line", "belt", "summary"} {
		t.Run(kind, func(t *testing.T) {
			downloadKind := kind
			if kind != "summary" {
				downloadKind = "detail-" + kind
			}
			raw, err := TemplateWorkbook([]service.ReefDataDetail{d}, downloadKind)
			if err != nil {
				t.Fatal(err)
			}
			files := readWorkbook(t, raw)
			base, err := openTemplate(kind)
			if err != nil {
				t.Fatal(err)
			}
			beforeStyles, afterStyles := string(base.parts["xl/styles.xml"]), files["xl/styles.xml"]
			if kind == "belt" {
				beforeXF, afterXF := xmlElement(beforeStyles, "cellXfs"), xmlElement(afterStyles, "cellXfs")
				xfs := regexpTag("xf")
				old, updated := xfs.FindAllString(beforeXF, -1), xfs.FindAllString(afterXF, -1)
				if len(updated) != len(old)+1 || strings.Join(old, "") != strings.Join(updated[:len(old)], "") || withoutElement(beforeStyles, "cellXfs") != withoutElement(afterStyles, "cellXfs") {
					t.Fatal("wrapping the long-spined row changed existing template styles")
				}
			} else if beforeStyles != afterStyles {
				t.Fatal("template styles were rewritten")
			}
			for _, s := range base.sheets {
				for _, tag := range []string{"cols", "mergeCells", "pageMargins", "pageSetup", "headerFooter", "drawing"} {
					if kind != "summary" && strings.HasPrefix(s.name, "GRAPHS") && (tag == "cols" || tag == "mergeCells") {
						continue
					}
					re := regexpTag(tag)
					before, after := re.FindString(s.raw), re.FindString(files[s.path])
					if before != after {
						t.Fatalf("%s layout %s changed", s.name, tag)
					}
				}
			}
			for _, name := range []string{"謝宗宏", "林沛臻", "蔡嘉和", "邱一平", "吳兆凱", "曾郁淇", "鄭毓毅", "黃靖雲", "2016-202604"} {
				for part, content := range files {
					if strings.Contains(content, name) {
						t.Fatalf("example value %q leaked in %s", name, part)
					}
				}
			}
			if kind == "line" {
				data := cells(t, files["xl/worksheets/sheet1.xml"])
				if data["L17"].Number != "10" || data["Q11"].Number != "10" || data["F56"].Number != "1" {
					t.Fatal("OT handboard value, legend or segment count wrong", data["L17"], data["Q11"], data["F56"])
				}
				if data["C1"].Text != "測試海岸" || data["C2"].Number != "5" {
					t.Fatal("line metadata wrong")
				}
				g := cells(t, files["xl/worksheets/sheet2.xml"])
				if g["B5"].Number != "21.875" {
					t.Fatal("coverage wrong", g["B5"])
				}
				if _, ok := files["xl/charts/chart9.xml"]; ok {
					t.Fatal("mud-underlayer chart exported")
				}
			}
			if kind == "belt" {
				data := cells(t, files["xl/worksheets/sheet1.xml"])
				if data["B1"].Text != "測試海岸" || data["P10"].Number != "9.25" {
					t.Fatal("belt metadata/combined fish wrong", data["P10"])
				}
				if data["B33"].Number != "" {
					t.Fatal("unrecorded shrimp became zero")
				}
			}
			if kind == "summary" {
				if !strings.Contains(files["xl/workbook.xml"], `name="20251212"`) {
					t.Fatal("snapshot sheet not renamed")
				}
				c := cells(t, files["xl/worksheets/sheet1.xml"])
				if c["B2"].Number != "21.875" || c["B42"].Number != "9.25" {
					t.Fatal("summary values wrong", c["B2"], c["B42"])
				}
				if strings.Contains(files["xl/workbook.xml"], "202009珊瑚計數") {
					t.Fatal("historical scratch sheet leaked")
				}
			}
			if dir := os.Getenv("REEF_REPORT_PREVIEW_DIR"); dir != "" {
				if err = os.WriteFile(filepath.Join(dir, kind+".xlsx"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func regexpTag(tag string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)<` + tag + `\b[^>]*?/>|<` + tag + `\b[^>]*>.*?</` + tag + `>`)
}
func TestTemplateArchiveAndHistory(t *testing.T) {
	a := templateFixture()
	b := templateFixture()
	b.Event.ID = 43
	b.Event.EventID = "Test_2026_04_11_0930_10"
	b.Event.DepthM = 10
	b.Event.SurveyDate = "2026-04-11"
	c := templateFixture()
	c.Event.ID = 44
	c.Event.DepthM = 15
	raw, err := TemplateArchive([]service.ReefDataDetail{a, b, c})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	line, belt, site, summary := 0, 0, 0, 0
	for _, f := range z.File {
		if strings.Contains(f.Name, "_LineTransect.xlsx") {
			line++
		}
		if strings.Contains(f.Name, "_BeltTransect.xlsx") {
			belt++
		}
		if strings.HasSuffix(f.Name, "_Site.xlsx") {
			site++
		}
		if strings.Contains(f.Name, "_Summary_") {
			summary++
		}
	}
	if line != 3 || belt != 3 || site != 3 || summary != 2 {
		t.Fatal("missing report files", line, belt, summary)
	}
	raw, err = TemplateWorkbook([]service.ReefDataDetail{a, b}, "summary")
	if err != nil {
		t.Fatal(err)
	}
	files := readWorkbook(t, raw)
	cells := cells(t, files["xl/worksheets/sheet2.xml"])
	if cells["C2"].Number != "21.875" || cells["D75"].Number != "21.875" {
		t.Fatal("history not aligned by date")
	}
	if _, err = TemplateWorkbook([]service.ReefDataDetail{a, b, c}, "summary"); err == nil {
		t.Fatal("third depth silently truncated")
	}
}

func TestTemplateChartReferencesAndBlankRecalculation(t *testing.T) {
	d := templateFixture()
	// A missing segment must stay absent in the cache and in the exported Excel formulas.
	d.Transects[0].Points = d.Transects[0].Points[:120]
	raw, err := TemplateWorkbook([]service.ReefDataDetail{d}, "detail-line")
	if err != nil {
		t.Fatal(err)
	}
	files := readWorkbook(t, raw)
	cc := cells(t, files["xl/worksheets/sheet1.xml"])
	if cc["H73"].Number != "" || cc["H83"].Number != "0" {
		t.Fatal("missing segment treated as recorded zero")
	}
	if !strings.Contains(files["xl/worksheets/sheet2.xml"], "COUNT(DATA!B86,DATA!D86,DATA!F86,DATA!H86)") {
		t.Fatal("SE recalculation still assumes four segments")
	}
	for _, kind := range []string{"line", "belt", "summary"} {
		downloadKind := kind
		if kind != "summary" {
			downloadKind = "detail-" + kind
		}
		raw, err := TemplateWorkbook([]service.ReefDataDetail{d}, downloadKind)
		if err != nil {
			t.Fatal(err)
		}
		parts := readWorkbook(t, raw)
		wb, err := openTemplate(kind)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, s := range wb.sheets {
			names[s.name] = true
		}
		if kind == "summary" {
			delete(names, "20260411")
			names["20251212"] = true
		}
		for name, part := range parts {
			if !strings.HasPrefix(name, "xl/charts/chart") || !strings.HasSuffix(name, ".xml") {
				continue
			}
			for _, f := range chartFormulaRE.FindAllStringSubmatch(part, -1) {
				m := rangeRE.FindStringSubmatch(f[1])
				if m == nil {
					t.Fatalf("%s %s invalid chart reference %s", kind, name, f[1])
				}
				target := m[1]
				if target == "" {
					target = m[2]
				}
				if !names[target] {
					t.Fatalf("chart %s references missing sheet %s", name, target)
				}
			}
		}
	}
}
func TestSiltMorphologyAndCombinedFishDoNotInventSize(t *testing.T) {
	d := templateFixture()
	d.Transects[0].Points[0].Code = "SI(HC-a)"
	d.Transects[1].Belt = []service.ReefDataBelt{{NameEN: "Parrotfish", Segment: 1, Count: 7}, {NameEN: "Grouper", Segment: 1, Count: 3}, {NameEN: "Butterflyfish", Segment: 1, Count: 8}}
	m := buildEventData(d)
	if *m.values["count_SI"][0] != 1 || m.values["parrot_large"][0] != nil || *m.values["butterfly_total"][0] != 8 {
		t.Fatal("silt/combined mapping wrong")
	}
	raw, err := TemplateWorkbook([]service.ReefDataDetail{d}, "detail-belt")
	if err != nil {
		t.Fatal(err)
	}
	parts := readWorkbook(t, raw)
	cc := cells(t, parts["xl/worksheets/sheet1.xml"])
	if cc["K17"].Number != "" || cc["B28"].Number != "3" {
		t.Fatal("combined sizes silently inferred")
	}
	if strings.Contains(parts["xl/worksheets/sheet1.xml"], `<c r="K17" s="`) && strings.Contains(regexp.MustCompile(`(?s)<c\b[^>]*r="K17"[^>]*>.*?</c>`).FindString(parts["xl/worksheets/sheet1.xml"]), "<f>") {
		t.Fatal("combined parrotfish can recalculate as >=20cm")
	}
}

func TestFieldReportWorkbooks(t *testing.T) {
	d := templateFixture()
	d.Event.Latitude = num(-25.125)
	d.Event.Longitude = num(121.5)
	d.Transects[0].PointMarks = true
	d.Transects[0].Bleaching = []service.ReefDataBleaching{{Segment: 1, HC: 2}}
	d.Transects[0].Points[0].Code = "NA"
	d.Transects[0].Points[1].Code = "SI(HC-a)"
	d.Transects[0].Points = d.Transects[0].Points[:120] // fourth segment not sampled
	d.Transects[2].Impacts = append(d.Transects[2].Impacts, service.ReefDataImpact{Segment: 1, NameEN: "Bleaching (% of population)", ValueType: "percent", RawValue: 25})
	for kind, sheets := range map[string][]string{
		"line": {"DATA", "GRAPHS", "INSTRUCTIONS", "FIELD SHEET"},
		"belt": {"DATA", "GRAPHS", "INSTRUCTIONS", "FIELD SHEET FISH", "FIELD SHEET INVERTS"},
		"site": {"DATA", "FIELD SHEET SITE"},
	} {
		t.Run(kind, func(t *testing.T) {
			raw, err := TemplateWorkbook([]service.ReefDataDetail{d}, kind)
			if err != nil {
				t.Fatal(err)
			}
			parts := readWorkbook(t, raw)
			wb := parts["xl/workbook.xml"]
			for _, sheet := range sheets {
				if !strings.Contains(wb, `name="`+sheet+`"`) {
					t.Fatal("missing worksheet", sheet)
				}
			}
			data := cells(t, parts["xl/worksheets/sheet1.xml"])
			switch kind {
			case "line":
				if data["F37"].Number != "" || data["D1"].Text != "Test Coast" || data["B15"].Text != "NA" || data["B16"].Text != "SI" || data["L17"].Text != "OT" || data["D15"].Text != "RC" || data["B62"].Number != "39" || data["H52"].Number != "" || data["B52"].Number != "0" {
					t.Fatal("incorrect point grid or coverage", data["L17"], data["B62"])
				}
			case "belt":
				if data["B10"].Number != "9" || data["F10"].Number != "37" || data["G10"].Number != "9.25" || data["B55"].Number != "0.25" || data["B29"].Number != "" || data["B54"].Number != "0" {
					t.Fatal("incorrect belt measures", data["B10"], data["F10"], data["B55"])
				}
			case "site":
				if data["B3"].Text != "Test Coast" || data["B8"].Number != "25" || data["D8"].Number != "7" || data["F8"].Number != "30" || data["H8"].Text != "S" || data["H9"].Text != "E" || data["H7"].Number != "" {
					t.Fatal("incorrect site metadata", data["B3"], data["D8"])
				}
			}
			for _, part := range parts {
				if strings.Contains(part, "Yeliu") || strings.Contains(part, "Taiwan-124") {
					t.Fatal("example observations retained")
				}
			}
			if kind != "site" && !strings.Contains(parts["xl/charts/chart1.xml"], "Test Coast") {
				t.Fatal("chart missing event title")
			}
			if dir := os.Getenv("REEF_REPORT_PREVIEW_DIR"); dir != "" {
				if err = os.WriteFile(filepath.Join(dir, "field-"+kind+".xlsx"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
