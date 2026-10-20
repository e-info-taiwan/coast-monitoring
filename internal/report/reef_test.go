package report

import (
	"archive/zip"
	"bytes"
	"coast-monitoring/internal/service"
	"encoding/xml"
	"io"
	"math"
	"path"
	"strings"
	"testing"
)

type xmlCell struct {
	Ref    string `xml:"r,attr"`
	Type   string `xml:"t,attr"`
	Text   string `xml:"is>t"`
	Number string `xml:"v"`
}

func readWorkbook(t *testing.T, data []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		dec := xml.NewDecoder(bytes.NewReader(raw))
		for {
			_, err = dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("invalid XML %s: %v", f.Name, err)
			}
		}
		files[f.Name] = string(raw)
	}
	return files
}
func cells(t *testing.T, raw string) map[string]xmlCell {
	t.Helper()
	var s struct {
		Rows []struct {
			Cells []xmlCell `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	out := map[string]xmlCell{}
	for _, r := range s.Rows {
		for _, c := range r.Cells {
			out[c.Ref] = c
		}
	}
	return out
}

func TestReefWorkbookTemplateStatisticsAndMissingValues(t *testing.T) {
	line := service.ReefDataTransect{Method: "line", CoralMorphology: "classified"}
	hc := [4]int{2, 13, 9, 11}
	for seg, n := range hc {
		for i := 0; i < 40; i++ {
			code := "RC"
			if i < n {
				code = "HC-c"
			}
			line.Points = append(line.Points, service.ReefDataPoint{Segment: seg + 1, Position: float64(seg*25) + float64(i)*0.5, Layer: "surface", Code: code})
		}
	}
	line.Points = append(line.Points, service.ReefDataPoint{Segment: 1, Layer: "down", Code: "HC-a"})
	invert := service.ReefDataTransect{Method: "belt_invert"}
	for seg, n := range []int{12, 5, 5, 9} {
		invert.Belt = append(invert.Belt, service.ReefDataBelt{TaxonID: 1, Segment: seg + 1, NameZH: "魔鬼海膽", NameEN: "Diadema", Group: "invert", Count: n, RecordStatus: "recorded"}, service.ReefDataBelt{TaxonID: 2, Segment: seg + 1, NameEN: "Echinothrix", Group: "invert", Count: []int{1, 2, 1, 1}[seg], RecordStatus: "recorded"})
	}
	invert.Belt = append(invert.Belt, service.ReefDataBelt{TaxonID: 3, Segment: 1, NameZH: "未調查", RecordStatus: "not_recorded", Count: 999})
	e := service.ReefDataEvent{EventID: "Hejie_2026_04_11_1000_10", SiteName: "合界", SurveyDate: "2026-04-11", DepthM: 10, Label: "secret"}
	data, err := ReefWorkbook([]service.ReefDataDetail{{Event: e, Transects: []service.ReefDataTransect{line, invert}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := readWorkbook(t, data)
	c := cells(t, files["xl/worksheets/sheet7.xml"])
	if c["L2"].Number != "21.875" || c["K2"].Number != "35" || c["P2"].Number != "21.875" {
		t.Fatalf("HC disagrees with template: %+v %+v %+v", c["L2"], c["K2"], c["P2"])
	}
	if !strings.Contains(files["xl/worksheets/sheet4.xml"], e.EventID+"_S1") {
		t.Fatal("missing segment export ID")
	}
	rows := cells(t, files["xl/worksheets/sheet3.xml"])
	if len(rows) != 9+160*8 {
		t.Fatalf("down layer leaked or cells lost: %d", len(rows))
	}
	found := false
	for ref, cell := range c {
		if cell.Text == "長刺海膽（魔鬼＋刺棘）" {
			row := strings.TrimPrefix(ref, "F")
			if c["G"+row].Number != "13" || c["L"+row].Number != "9" {
				t.Fatal("urchins not summed per segment")
			}
			found = true
		}
		if cell.Text == "未調查" {
			row := strings.TrimPrefix(ref, "F")
			if _, ok := c["L"+row]; ok {
				t.Fatal("missing count became zero")
			}
			if c["O"+row].Number != "0" {
				t.Fatal("NA counted")
			}
		}
	}
	if !found {
		t.Fatal("no long-spined urchin summary")
	}
	if !strings.Contains(files["xl/charts/chart1.xml"], "<c:errBars>") {
		t.Fatal("missing SE error bars")
	}
}
func TestSubstrateMappingAndStatistics(t *testing.T) {
	for _, r := range [][4]string{{"HC-a", "HC", "a", ""}, {"SI(HC-c)", "SI", "c", "HC"}, {"SI(SC)", "SI", "", "SC"}, {"RC", "RC", "", ""}} {
		a, b, c := substrate(r[0])
		if a != r[1] || b != r[2] || c != r[3] {
			t.Fatal(r, a, b, c)
		}
	}
	_, mean, sd, se := stats([]float64{13, 7, 6, 10})
	if mean != 9.0 || math.Abs(sd.(float64)-math.Sqrt(10)) > 1e-10 || math.Abs(se.(float64)-math.Sqrt(10)/2) > 1e-10 {
		t.Fatal(mean, sd, se)
	}
	_, _, sd, se = stats([]float64{0})
	if sd != nil || se != nil {
		t.Fatal("one observation cannot estimate SD")
	}
}
func TestDerivedAggregateIgnoresHandEnteredTotalsAndRequiresMembers(t *testing.T) {
	taxa := []service.ReefDataTaxon{{ID: 1, TaxonGroup: "fish", AggregateOf: "grouper", IsActive: true}, {ID: 2, TaxonGroup: "fish", AggregateOf: "grouper", IsActive: true}, {ID: 3, TaxonGroup: "fish", AggregateOf: "grouper", IsActive: true, IsAggregate: true}}
	rows := derivedBelt([]service.ReefDataBelt{{TaxonID: 1, Segment: 1, Count: 2}, {TaxonID: 2, Segment: 1, Count: 3}, {TaxonID: 3, Segment: 1, Count: 999, Aggregate: true}, {TaxonID: 1, Segment: 2, Count: 4}}, taxa)
	for _, r := range rows {
		if r.TaxonID == 3 {
			if r.Segment == 1 && r.Count != 5 {
				t.Fatal("raw aggregate leaked")
			}
			if r.Segment == 2 && r.RecordStatus != "not_recorded" {
				t.Fatal("missing aggregate member treated as zero")
			}
		}
	}
}
func TestXLSXTextIsNeverExecutable(t *testing.T) {
	b := workbook{sheets: []sheet{{name: "文字", rows: [][]any{{"標題"}, {`=HYPERLINK("https://example.test","x")`}, {"<&>中文"}}}}}
	data, err := b.bytes()
	if err != nil {
		t.Fatal(err)
	}
	files := readWorkbook(t, data)
	raw := files["xl/worksheets/sheet1.xml"]
	if strings.Contains(raw, "<f>") {
		t.Fatal("text became formula")
	}
	c := cells(t, raw)
	if c["A2"].Type != "inlineStr" || c["A3"].Text != "<&>中文" {
		t.Fatal("text corrupted")
	}
}

func TestWorkbookRelationshipsResolve(t *testing.T) {
	data, err := ReefWorkbook([]service.ReefDataDetail{{Transects: []service.ReefDataTransect{{Method: "line"}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := readWorkbook(t, data)
	for name, raw := range files {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		var rels struct {
			Items []struct {
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		if err := xml.Unmarshal([]byte(raw), &rels); err != nil {
			t.Fatal(err)
		}
		base := ""
		if name != "_rels/.rels" {
			base = path.Dir(path.Dir(name))
		}
		for _, r := range rels.Items {
			target := path.Clean(path.Join(base, r.Target))
			if _, ok := files[target]; !ok {
				t.Fatalf("%s targets missing %s", name, target)
			}
		}
	}
}

func TestImpactCountTypeIncludedWithoutGradeAndMixedUnitsSeparated(t *testing.T) {
	d := service.ReefDataDetail{Transects: []service.ReefDataTransect{{Method: "belt_invert", Impacts: []service.ReefDataImpact{{TypeID: 1, Segment: 1, NameZH: "件數", ValueType: "count", RawValue: 5}, {TypeID: 2, Segment: 1, NameZH: "比例", ValueType: "percent", RawValue: 20}, {TypeID: 3, Segment: 1, NameZH: "未調查", ValueType: "level", RawValue: 2, RecordStatus: "not_recorded"}}}}}
	data, err := ReefWorkbook([]service.ReefDataDetail{d}, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := readWorkbook(t, data)
	c := cells(t, files["xl/worksheets/sheet7.xml"])
	for ref, cell := range c {
		if cell.Text == "件數" {
			row := strings.TrimPrefix(ref, "F")
			if c["L"+row].Number != "5" {
				t.Fatal("count summary missing")
			}
		}
		if cell.Text == "未調查" {
			row := strings.TrimPrefix(ref, "F")
			if _, ok := c["L"+row]; ok {
				t.Fatal("missing impact became numeric")
			}
		}
	}
	if !strings.Contains(files["xl/charts/chart2.xml"], "個數") || !strings.Contains(files["xl/charts/chart4.xml"], "percent") {
		t.Fatal("unit chart grouping incorrect")
	}
}
