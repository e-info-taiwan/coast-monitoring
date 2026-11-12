package report

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"coast-monitoring/internal/service"
)

func TestReportChartPresentation(t *testing.T) {
	for _, kind := range []string{"line", "belt", "detail-line", "detail-belt"} {
		t.Run(kind, func(t *testing.T) {
			raw, err := TemplateWorkbook([]service.ReefDataDetail{templateFixture()}, kind)
			if err != nil {
				t.Fatal(err)
			}
			parts := readWorkbook(t, raw)
			baseKind := "field-" + kind
			if strings.HasPrefix(kind, "detail-") {
				baseKind = strings.TrimPrefix(kind, "detail-")
			}
			base, err := openTemplate(baseKind)
			if err != nil {
				t.Fatal(err)
			}
			chartCount, drawnCount := 0, 0
			for name, data := range parts {
				if regexp.MustCompile(`^xl/charts/chart\d+\.xml$`).MatchString(name) {
					chartCount++
					// Presentation changes must not change series or error-bar sources.
					if !reflect.DeepEqual(chartFormulaRE.FindAllString(string(base.parts[name]), -1), chartFormulaRE.FindAllString(data, -1)) && !(baseKind == "belt" && strings.HasSuffix(name, "chart7.xml")) {
						t.Fatalf("%s series changed", name)
					}
					for _, forbidden := range []string{"outerShdw", "innerShdw", "manualLayout", "pie3DChart", "c:explosion"} {
						if strings.Contains(data, forbidden) {
							t.Fatalf("%s retained %s", name, forbidden)
						}
					}
					if strings.Contains(data, "<c:pieChart>") {
						if !strings.Contains(data, `<c:showPercent val="1"/>`) || !strings.Contains(data, `<c:legendPos val="b"/>`) || !strings.Contains(data, `<c:overlay val="0"/>`) {
							t.Fatal("pie labels/legend missing")
						}
					} else {
						axis := xmlElement(data, "c:valAx")
						if xmlText(xmlElement(axis, "c:title")) == "" || !strings.Contains(axis, "<c:majorGridlines>") {
							t.Fatal("Y-axis title/gridlines missing")
						}
					}
				}
				if !regexp.MustCompile(`^xl/drawings/drawing\d+\.xml$`).MatchString(name) {
					continue
				}
				if !strings.Contains(data, "<c:chart") {
					continue
				}
				var drawing struct {
					Anchors []struct {
						From struct {
							Col    int `xml:"col"`
							ColOff int `xml:"colOff"`
							Row    int `xml:"row"`
							RowOff int `xml:"rowOff"`
						} `xml:"from"`
						Ext struct {
							Width  int `xml:"cx,attr"`
							Height int `xml:"cy,attr"`
						} `xml:"ext"`
					} `xml:"oneCellAnchor"`
				}
				if err := xml.Unmarshal([]byte(data), &drawing); err != nil {
					t.Fatal(err)
				}
				drawnCount += len(drawing.Anchors)
				type rect struct{ x, y, w, h int }
				var rectangles []rect
				for _, a := range drawing.Anchors {
					r := rect{(a.From.Col-4)*80 + a.From.ColOff/9525, a.From.Row*20 + a.From.RowOff/9525, a.Ext.Width / 9525, a.Ext.Height / 9525}
					if r.w < 450 || r.h < 400 {
						t.Fatal("chart too small", r)
					}
					for _, q := range rectangles {
						if r.x < q.x+q.w && q.x < r.x+r.w && r.y < q.y+q.h && q.y < r.y+r.h {
							t.Fatal("charts overlap", r, q)
						}
					}
					rectangles = append(rectangles, r)
				}
			}
			if chartCount == 0 || chartCount != drawnCount {
				t.Fatal("lost chart anchors", chartCount, drawnCount)
			}
		})
	}
}

func TestInternalReportWorkingRowsAndLongSpinedLabel(t *testing.T) {
	line, err := TemplateWorkbook([]service.ReefDataDetail{templateFixture()}, "detail-line")
	if err != nil {
		t.Fatal(err)
	}
	data := readWorkbook(t, line)["xl/worksheets/sheet1.xml"]
	for row := 86; row <= 125; row++ {
		pattern := regexp.MustCompile(fmt.Sprintf(`<row\b[^>]*r="%d"[^>]*>`, row))
		if !strings.Contains(pattern.FindString(data), `hidden="1"`) {
			t.Fatalf("helper row %d is exposed", row)
		}
	}
	if cells(t, data)["B86"].Number != "5" {
		t.Fatal("hidden helper calculation was removed")
	}
	belt, err := TemplateWorkbook([]service.ReefDataDetail{templateFixture()}, "detail-belt")
	if err != nil {
		t.Fatal(err)
	}
	parts := readWorkbook(t, belt)
	data = parts["xl/worksheets/sheet1.xml"]
	c := cells(t, data)["A42"]
	if c.Text != "Long-spined urchin\n長刺海膽（魔鬼＋刺棘）" {
		t.Fatal("label missing", c)
	}
	if !strings.Contains(regexp.MustCompile(`<row\b[^>]*r="42"[^>]*>`).FindString(data), `ht="42"`) {
		t.Fatal("long label row too short")
	}
	cell := regexp.MustCompile(`(?s)<c\b[^>]*r="A42"[^>]*>.*?</c>`).FindString(data)
	i, _ := strconv.Atoi(styleRE.FindStringSubmatch(cell)[1])
	xfs := regexpTag("xf").FindAllString(xmlElement(parts["xl/styles.xml"], "cellXfs"), -1)
	if !strings.Contains(xfs[i], `wrapText="1"`) {
		t.Fatal("long label does not wrap")
	}
}

func TestHeadquartersEnglishNamesAndFallback(t *testing.T) {
	for _, english := range []bool{true, false} {
		d := templateFixture()
		name, person := "Test Coast", "Example Diver"
		p := service.ReefDataParticipant{Role: "member", NameEN: " Example Diver ", NameZH: "測試人員"}
		if !english {
			d.Event.SiteEnglish = "  "
			p.NameEN = "  "
			name, person = d.Event.SiteName, p.NameZH
		}
		for i := range d.Transects {
			d.Transects[i].Participants = []service.ReefDataParticipant{p}
		}
		for _, kind := range []string{"line", "belt", "site"} {
			raw, err := TemplateWorkbook([]service.ReefDataDetail{d}, kind)
			if err != nil {
				t.Fatal(err)
			}
			parts := readWorkbook(t, raw)
			data := cells(t, parts["xl/worksheets/sheet1.xml"])
			ref := map[string]string{"line": "D1", "belt": "B1", "site": "B3"}[kind]
			if data[ref].Text != name {
				t.Fatal(kind, "site name", data[ref])
			}
			found := false
			for _, c := range data {
				if c.Text == person {
					found = true
				}
			}
			if !found {
				t.Fatal(kind, "recorder name absent", person)
			}
			for file, content := range parts {
				if !regexp.MustCompile(`^xl/charts/chart\d+\.xml$`).MatchString(file) {
					continue
				}
				if title := xmlText(xmlElement(content, "c:title")); !strings.Contains(title, name) || (english && strings.Contains(title, d.Event.SiteName)) {
					t.Fatal(kind, title)
				}
			}
		}
	}
}

func TestReportBleachingPieValues(t *testing.T) {
	d := templateFixture()
	for seg := 1; seg <= 4; seg++ {
		d.Transects[2].Impacts = append(d.Transects[2].Impacts,
			service.ReefDataImpact{Segment: seg, NameEN: "Bleaching (% of population)", ValueType: "percent", RawValue: 25},
			service.ReefDataImpact{Segment: seg, NameEN: "Bleaching (% of colony)", ValueType: "percent", RawValue: 0})
	}
	for _, kind := range []string{"belt", "detail-belt"} {
		raw, err := TemplateWorkbook([]service.ReefDataDetail{d}, kind)
		if err != nil {
			t.Fatal(err)
		}
		parts := readWorkbook(t, raw)
		chart, expected := 3, []string{"25", "75"}
		if kind == "belt" {
			chart, expected = 6, []string{"0.25", "0.75"}
		}
		for offset := 0; offset < 2; offset++ {
			if offset == 1 {
				expected = []string{"0", "100"}
				if kind == "belt" {
					expected[1] = "1"
				}
			}
			values := xmlElement(parts[fmt.Sprintf("xl/charts/chart%d.xml", chart+offset)], "c:val")
			for i, value := range expected {
				if !strings.Contains(values, fmt.Sprintf(`<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, value)) {
					t.Fatal("pie values changed", kind, values)
				}
			}
		}
		if dir := os.Getenv("REEF_REPORT_PREVIEW_DIR"); dir != "" {
			if err := os.WriteFile(filepath.Join(dir, "bleaching-"+kind+".xlsx"), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
