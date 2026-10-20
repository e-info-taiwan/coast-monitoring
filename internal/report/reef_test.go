package report

import (
	"archive/zip"
	"bytes"
	"coast-monitoring/internal/service"
	"encoding/xml"
	"io"
	"math"
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
		if !strings.HasSuffix(f.Name, ".xml") && !strings.HasSuffix(f.Name, ".rels") {
			files[f.Name] = string(raw)
			continue
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
func TestXLSXTextIsNeverExecutable(t *testing.T) {
	d := templateFixture()
	d.Event.SiteName = `=HYPERLINK("https://example.test","x")`
	data, err := TemplateWorkbook([]service.ReefDataDetail{d}, "line")
	if err != nil {
		t.Fatal(err)
	}
	files := readWorkbook(t, data)
	cc := cells(t, files["xl/worksheets/sheet1.xml"])
	if cc["C1"].Type != "inlineStr" || cc["C1"].Text != d.Event.SiteName {
		t.Fatal("user text became executable or corrupted")
	}
}
