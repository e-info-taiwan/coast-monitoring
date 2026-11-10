package report

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"testing"

	"coast-monitoring/internal/service"
)

func TestTaiBIFExchangeRows(t *testing.T) {
	d := templateFixture()
	rawCode := "0"
	bleached := true
	d.Transects[0].Points = []service.ReefDataPoint{
		{Segment: 3, Position: 61, Layer: "surface", Code: "OT", RawInputCode: &rawCode},
		{Segment: 3, Position: 61, Layer: "down", Code: "SC"},
		{Segment: 3, Position: 61.5, Layer: "surface", Code: "SI(HC-a)", IsBleached: &bleached},
	}
	fish := &d.Transects[1]
	fish.Belt = []service.ReefDataBelt{
		{ID: 71, Segment: 3, NameEN: "Butterflyfish", Size: ">=5cm", Count: 4},
		{ID: 72, Segment: 3, NameEN: "Butterflyfish", Size: "<5cm", Count: 0},
		{ID: 73, Segment: 4, NameZH: "測試", RecordStatus: "not_recorded"},
	}
	fish.Participants = []service.ReefDataParticipant{{Role: "member", NameZH: "測試員"}}
	d.Transects[2].Belt = nil
	raw, err := FormatArchive([]service.ReefDataDetail{d}, "taibif")
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	csvs := map[string][][]string{}
	if len(z.File) != 5 {
		t.Fatal(len(z.File))
	}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(f.Name, ".xlsx") {
			sheet := cells(t, readWorkbook(t, body)["xl/worksheets/sheet1.xml"])
			if sheet["G2"].Text != "0" || sheet["H2"].Text != "OT" || sheet["C2"].Number == "" || sheet["H3"].Text != "SI" || sheet["I3"].Text != "a" || sheet["J3"].Text != "HC" || sheet["K3"].Text != "Bleached" || sheet["D4"].Text != "" {
				t.Fatal("unexpected substrate rows")
			}
			continue
		}
		rows, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		csvs[f.Name[strings.LastIndex(f.Name, "_")+1:]] = rows
	}
	core, occ, mof, emof := csvs["Event-Core.csv"], csvs["Occurance.csv"], csvs["MoF.csv"], csvs["eMoF.csv"]
	if len(core) != 5 || len(occ) != 4 || len(mof) != 5 || len(emof) != 2 {
		t.Fatal("record counts", len(core), len(occ), len(mof), len(emof))
	}
	if core[3][10] != d.Event.EventID || core[3][0] != occ[1][1] || occ[1][2] != "Chaetodontidae" || occ[1][7] != "測〇員" || occ[1][8] != "4" || occ[2][8] != "0" || occ[3][8] != "" {
		t.Fatal(occ)
	}
	if emof[1][0] != occ[1][0] || emof[1][3] != "at least 5" || emof[1][4] != "cm" || mof[1][0] != core[1][0] || mof[1][3] != "0" {
		t.Fatal(emof, mof)
	}
	if v, u := taiSize("約1m"); v != "約1" || u != "m" {
		t.Fatal(v, u)
	}
	if _, err = FormatArchive([]service.ReefDataDetail{d}, "unknown"); !errors.Is(err, service.ErrValidation) {
		t.Fatal(err)
	}
}
