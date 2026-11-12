package report

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"coast-monitoring/internal/service"
)

// The international forms use the English site name when it is available.
func fieldSiteName(e service.ReefDataEvent) string {
	if name := strings.TrimSpace(e.SiteEnglish); name != "" {
		return name
	}
	return e.SiteName
}

func fieldEventTitle(e service.ReefDataEvent) string {
	e.SiteName = fieldSiteName(e)
	return eventTitle(e)
}

func fieldPeople(role string, transects ...*service.ReefDataTransect) []string {
	var result []string
	for _, tr := range transects {
		if tr == nil {
			continue
		}
		for _, p := range tr.Participants {
			if p.Role != role {
				continue
			}
			name := strings.TrimSpace(p.NameEN)
			if name == "" {
				name = strings.TrimSpace(p.NameZH)
			}
			if name == "" {
				name = p.UserName
			}
			if p.Code != "" {
				name = strings.TrimSpace(name + " (" + p.Code + ")")
			}
			if name == "" {
				continue
			}
			if !slices.Contains(result, name) {
				result = append(result, name)
			}
		}
	}
	return result
}

func setCoordinate(s *templateSheet, row string, coordinate any, positive, negative string) {
	v, ok := coordinate.(float64)
	if !ok {
		return
	}
	hemisphere := positive
	if v < 0 {
		hemisphere = negative
	}
	tenths := int(math.Round(math.Abs(v) * 36000))
	s.set("B"+row, tenths/36000)
	s.set("D"+row, tenths/600%60)
	s.set("F"+row, float64(tenths%600)/10)
	s.set("H"+row, hemisphere)
}

func siteFieldReport(d *eventData) ([]byte, error) {
	b, err := openTemplate("field-site")
	if err != nil {
		return nil, err
	}
	s := b.byName["DATA"]
	e := d.detail.Event
	s.set("B3", fieldSiteName(e))
	s.set("B6", "Taiwan")
	s.set("E6", e.County)
	s.set("H6", e.Location)
	s.set("B7", excelDate(e.SurveyDate))
	s.set("E7", excelTime(e.EventTime))
	weather, air, lat, lon := metadata(d)
	setCoordinate(s, "8", lat, "N", "S")
	setCoordinate(s, "9", lon, "E", "W")
	s.set("C11", air)
	s.set("B17", weather)
	var vis []string
	var water *float64
	for _, t := range d.detail.Transects {
		if water == nil {
			water = t.WaterTemp
		}
		var visibility string
		switch {
		case t.VisibilityMin != nil && t.VisibilityMax != nil:
			visibility = fmt.Sprintf("%g–%g", *t.VisibilityMin, *t.VisibilityMax)
		case t.VisibilityMin != nil:
			visibility = fmt.Sprintf("≥%g", *t.VisibilityMin)
		case t.VisibilityMax != nil:
			visibility = fmt.Sprintf("≤%g", *t.VisibilityMax)
		}
		if visibility != "" && !slices.Contains(vis, visibility) {
			vis = append(vis, visibility)
		}
	}
	if water == nil {
		water = e.WaterTemp
	}
	switch e.DepthM {
	case 3:
		s.set("G11", value(water))
	case 10:
		s.set("I11", value(water))
	}
	s.set("C18", strings.Join(vis, " / "))
	for role, cell := range map[string]string{"team_leader": "B55", "team_scientist": "B56"} {
		s.set(cell, strings.Join(fieldPeople(role, d.line, d.fish, d.invert), "、"))
	}
	members := fieldPeople("member", d.line, d.fish, d.invert)
	if len(members) > 6 {
		xml := string(b.parts["xl/workbook.xml"])
		b.parts["xl/workbook.xml"] = []byte(strings.Replace(xml, "$J$58", "$J$"+strconv.Itoa(52+len(members)), 1))
	}
	for i, name := range members {
		s.set(fmt.Sprintf("G%d", 53+i), name)
	}
	return b.bytes()
}

// Excel stores 100% as 1. An unobserved segment must stay blank.
func fractions(values segmentValues) segmentValues {
	var out segmentValues
	for i, v := range values {
		if v != nil {
			out[i] = num(*v / 100)
		}
	}
	return out
}

func fieldMean(s *templateSheet, cell, inputs string, cached any) {
	s.formula(cell, fmt.Sprintf(`IF(COUNT(%s)=0,"",AVERAGE(%s))`, inputs, inputs), cached)
}

func fieldGraph(g *templateSheet, row, source int, meanCol, sdCol, countRange string, mean, se any) {
	meanRef := fmt.Sprintf("DATA!%s%d", meanCol, source)
	g.formula(fmt.Sprintf("B%d", row), fmt.Sprintf(`IF(ISNUMBER(%s),%s,"")`, meanRef, meanRef), mean)
	g.formula(fmt.Sprintf("C%d", row), fmt.Sprintf(`IF(COUNT(%s)<2,"",DATA!%s%d/SQRT(COUNT(%s)))`, countRange, sdCol, source, countRange), se)
}

func lineFieldReport(d *eventData) ([]byte, error) {
	if d.line == nil {
		return nil, fmt.Errorf("%w: 未記錄底質", service.ErrValidation)
	}
	b, err := openTemplate("field-line")
	if err != nil {
		return nil, err
	}
	s, g := b.byName["DATA"], b.byName["GRAPHS"]
	e := d.detail.Event
	s.set("D1", fieldSiteName(e))
	s.set("L1", "Taiwan")
	s.set("D2", fmt.Sprintf("%gm", e.DepthM))
	s.set("L2", excelDate(e.SurveyDate))
	s.set("D3", strings.Join(fieldPeople("team_leader", d.line), ", "))
	s.set("L3", strings.Join(fieldPeople("member", d.line), ", "))
	s.set("D4", excelTime(e.EventTime))
	s.set("C45", value(d.line.Comments))
	s.set("J43", value(d.line.RKCNote))
	for _, p := range d.line.Points {
		if p.Layer != "surface" || p.Segment < 1 || p.Segment > 4 {
			continue
		}
		offset := int(math.Round((p.Position - float64((p.Segment-1)*25)) * 2))
		if offset < 0 || offset >= 40 {
			continue
		}
		base, _, _ := substrate(p.Code)
		s.set(fmt.Sprintf("%s%d", column(2+(p.Segment-1)*4+2*(offset/20)), 15+offset%20), base)
	}
	cols := []string{"B", "D", "F", "H"}
	for seg, n := range d.valid {
		var parts []string
		for _, col := range []int{2 + 4*seg, 4 + 4*seg} {
			rng := fmt.Sprintf("%s15:%s34", column(col), column(col))
			parts = append(parts, fmt.Sprintf(`COUNTA(%s)-COUNTIF(%s,"NA")`, rng, rng))
		}
		s.formula(cols[seg]+"62", strings.Join(parts, "+"), n)
	}
	s.formula("I62", "SUM(B62,D62,F62,H62)", d.valid[0]+d.valid[1]+d.valid[2]+d.valid[3])
	graphRows := [][]int{{5, 20}, {6, 21}, {7, 27}, {8, 22}, {9, 23}, {10, 28}, {11, 29}, {12, 30}, {13, 31}, {14, 24}}
	for i, code := range substrateCodes {
		row := 52 + i
		counts := d.values["count_"+code]
		ratios := fractions(d.values["line_"+code])
		for seg, col := range cols {
			left, right := column(seg*4+2), column(seg*4+4)
			ref := fmt.Sprintf("%s%d", col, row)
			s.formula(ref, fmt.Sprintf(`IF(%s62=0,"",COUNTIF(%s15:%s34,$A%d)+COUNTIF(%s15:%s34,$A%d))`, col, left, left, row, right, right, row), value(counts[seg]))
			s.formula(column(16+seg)+strconv.Itoa(row), fmt.Sprintf(`IF(%s62=0,"",%s/%s62)`, col, ref, col), value(ratios[seg]))
		}
		statsCells(s, row, cols, []string{"J", "L"}, counts)
		rng := fmt.Sprintf("P%d:S%d", row, row)
		_, mean, sd, se := statsValues(ratios)
		fieldMean(s, fmt.Sprintf("M%d", row), rng, mean)
		s.formula(fmt.Sprintf("O%d", row), fmt.Sprintf(`IF(COUNT(%s)<2,"",STDEV(%s))`, rng, rng), sd)
		for _, gr := range graphRows[i] {
			fieldGraph(g, gr, row, "M", "O", "DATA!"+rng, mean, se)
		}
	}
	var total any
	if d.valid != [4]int{} {
		total = 1
	}
	g.formula("B16", `IF(COUNT(B5:B14)=0,"",SUM(B5:B14))`, total)
	var bleached segmentValues
	for i, col := range []string{"F", "H", "J", "L"} {
		hc := d.values["count_HC"][i]
		count := d.values["hc_bleach"][i]
		if hc != nil && *hc > 0 && count != nil {
			bleached[i] = num(*count / *hc)
		}
		s.set(col+"37", value(count))
		s.formula(col+"39", fmt.Sprintf(`IF(OR(NOT(ISNUMBER(%s37)),%s52=0),"",%s37/%s52)`, col, cols[i], col, cols[i]), value(bleached[i]))
		from := column(16+i) + "54"
		s.formula(col+"40", fmt.Sprintf(`IF(ISNUMBER(%s),%s,"")`, from, from), value(fractions(d.values["line_RKC"])[i]))
	}
	for row, vals := range map[int]segmentValues{37: d.values["hc_bleach"], 39: bleached, 40: fractions(d.values["line_RKC"])} {
		_, m, _, _ := statsValues(vals)
		fieldMean(s, "N"+strconv.Itoa(row), strings.Join(refs(row, "F", "H", "J", "L"), ","), m)
	}
	g.set("E6", "Substrate cover · "+fieldEventTitle(e))
	b.titles(fieldEventTitle(e), map[int]string{1: "Substrate cover", 2: "Live reef cover", 3: "Non-living reef cover", 4: "Live reef cover", 5: "Non-living reef cover"})
	if err := b.presentCharts("field-line"); err != nil {
		return nil, err
	}
	return b.bytes()
}

func beltFieldReport(d *eventData) ([]byte, error) {
	if d.fish == nil && d.invert == nil {
		return nil, fmt.Errorf("%w: 未記錄魚類或無脊椎", service.ErrValidation)
	}
	b, err := openTemplate("field-belt")
	if err != nil {
		return nil, err
	}
	s, g := b.byName["DATA"], b.byName["GRAPHS"]
	e := d.detail.Event
	s.set("B1", fieldSiteName(e))
	s.set("G1", "Taiwan")
	s.set("B2", fmt.Sprintf("%gm", e.DepthM))
	s.set("G2", strings.Join(fieldPeople("team_leader", d.fish, d.invert), "、"))
	s.set("B3", excelDate(e.SurveyDate))
	s.set("G3", excelTime(e.EventTime))
	s.set("B7", strings.Join(fieldPeople("member", d.fish), ", "))
	s.set("B26", strings.Join(fieldPeople("member", d.invert), ", "))
	var notes []string
	for _, t := range []*service.ReefDataTransect{d.fish, d.invert} {
		if t == nil {
			continue
		}
		if t.Comments != nil {
			notes = append(notes, *t.Comments)
		}
		if t.OtherDamageNote != "" {
			notes = append(notes, t.OtherDamageNote)
		}
	}
	s.set("B66", strings.Join(notes, "\n"))
	// Row in DATA, matching row in GRAPHS, and the observed measure.
	rows := []struct {
		data, graph int
		key         string
	}{
		{10, 5, "butterfly_total"}, {11, 6, "sweetlips_total"}, {12, 7, "snapper_total"}, {13, 8, "barramundi"},
		{14, 10, "humphead"}, {15, 11, "bumphead"}, {16, 12, "parrot_combined"}, {17, 13, "moray"},
		{20, 16, "grouper_30_40"}, {21, 17, "grouper_40_50"}, {22, 18, "grouper_50_60"}, {23, 19, "grouper_60"}, {24, 9, "grouper_large"},
		{29, 22, "shrimp"}, {30, 23, "diadema"}, {31, 24, "pencil"}, {32, 25, "collector"}, {33, 26, "cucumber"}, {34, 27, "crown"}, {35, 29, "triton"}, {36, 30, "lobster"},
		{39, 33, "clam_small"}, {40, 34, "clam_10_20"}, {41, 35, "clam_20_30"}, {42, 36, "clam_30_40"}, {43, 37, "clam_40_50"}, {44, 38, "clam_50"}, {45, 28, "clam_total"},
		{50, 41, "anchor_level"}, {51, 42, "dynamite_level"}, {52, 43, "damage_other_level"}, {53, 44, "fishnet_level"}, {54, 45, "trash_level"},
		{55, 48, "bleach_population"}, {56, 52, "bleach_colony"}, {58, 56, "black_band"}, {59, 57, "white_band"},
		{61, 0, "sharks"}, {62, 0, "turtles"}, {63, 0, "mantas"}, {64, 0, "rare_other"},
	}
	cols := []string{"B", "C", "D", "E"}
	for _, r := range rows {
		vals := d.values[r.key]
		if r.data == 16 && vals == (segmentValues{}) {
			d.sum("field_parrot_total", "parrot_large", "parrot_small")
			vals = d.values["field_parrot_total"]
		}
		if r.data == 24 && d.values["grouper_combined"] != (segmentValues{}) {
			vals = d.values["grouper_combined"]
		}
		percent := r.data >= 55 && r.data <= 59
		if percent {
			vals = fractions(vals)
		}
		setSeries(s, r.data, cols, vals)
		statsCells(s, r.data, cols, []string{"F", "G", "H"}, vals)
		if r.data >= 50 && r.data <= 59 {
			s.set(fmt.Sprintf("F%d", r.data), nil)
		}
		if r.graph == 0 {
			continue
		}
		_, m, _, se := statsValues(vals)
		rng := fmt.Sprintf("DATA!B%d:E%d", r.data, r.data)
		fieldGraph(g, r.graph, r.data, "G", "H", rng, m, se)
		if percent {
			g.set(fmt.Sprintf("C%d", r.graph), nil)
			if r.data == 55 || r.data == 56 {
				var remaining any
				if n, ok := m.(float64); ok {
					remaining = 1 - n
				}
				g.formula(fmt.Sprintf("B%d", r.graph+1), fmt.Sprintf(`IF(ISNUMBER(B%d),1-B%d,"")`, r.graph, r.graph), remaining)
			}
		}
	}
	for row, label := range map[int]string{7: "Fish", 58: "Invertebrates", 77: "Impacts, bleaching and disease"} {
		g.set(fmt.Sprintf("E%d", row), label+" · "+fieldEventTitle(e))
	}
	b.titles(fieldEventTitle(e), map[int]string{1: "Fish", 2: "Grouper", 3: "Invertebrates", 4: "Giant clam", 5: "Impacts", 6: "Bleaching – population", 7: "Bleaching – colony", 8: "Coral disease"})
	if err := b.presentCharts("field-belt"); err != nil {
		return nil, err
	}
	return b.bytes()
}
