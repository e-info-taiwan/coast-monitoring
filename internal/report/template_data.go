package report

import (
	"coast-monitoring/internal/service"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type segmentValues [4]*float64
type eventData struct {
	detail             service.ReefDataDetail
	values             map[string]segmentValues
	line, fish, invert *service.ReefDataTransect
	valid              [4]int
	rawCounts          map[string][4]float64
}

func (d *eventData) put(key string, segment int, v *float64) {
	if segment < 1 || segment > 4 {
		return
	}
	a := d.values[key]
	a[segment-1] = v
	d.values[key] = a
}
func (d *eventData) sum(key string, members ...string) {
	var out segmentValues
	for seg := 0; seg < 4; seg++ {
		sum := 0.0
		ok := true
		for _, m := range members {
			v := d.values[m][seg]
			if v == nil {
				ok = false
				break
			}
			sum += *v
		}
		if ok {
			out[seg] = num(sum)
		}
	}
	d.values[key] = out
}
func (d *eventData) summary(key string) (total, mean, sd, se any) {
	var v []float64
	for _, n := range d.values[key] {
		if n != nil {
			v = append(v, *n)
		}
	}
	return stats(v)
}
func buildEventData(detail service.ReefDataDetail) *eventData {
	d := &eventData{detail: detail, values: map[string]segmentValues{}, rawCounts: map[string][4]float64{}}
	for i := range d.detail.Transects {
		t := &d.detail.Transects[i]
		switch t.Method {
		case "line":
			d.line = t
		case "belt_fish":
			d.fish = t
		case "belt_invert":
			d.invert = t
		}
		if t.Method == "line" {
			for _, p := range t.Points {
				if p.Layer != "surface" || p.Segment < 1 || p.Segment > 4 || p.Code == "NA" || p.Code == "" {
					continue
				}
				seg := p.Segment - 1
				d.valid[seg]++
				for _, key := range []string{p.Code} {
					a := d.rawCounts[key]
					a[seg]++
					d.rawCounts[key] = a
				}
			}
			for _, key := range append(append([]string{}, substrateCodes...), "HC-a", "HC-b", "HC-c", "live_coral") {
				for seg, n := range d.valid {
					if n == 0 {
						continue
					}
					sum := 0.0
					for raw, counts := range d.rawCounts {
						base, _, _ := substrate(raw)
						if base == key || raw == key || (key == "live_coral" && (base == "HC" || base == "SC")) {
							sum += counts[seg]
						}
					}
					d.put("line_"+key, seg+1, num(sum/float64(n)*100))
					d.put("count_"+key, seg+1, num(sum))
				}
			}
			for _, b := range t.Bleaching {
				if observed(b.HCStatus) {
					d.put("hc_bleach", b.Segment, num(float64(b.HC)))
				}
				if observed(b.SCStatus) {
					d.put("sc_bleach", b.Segment, num(float64(b.SC)))
				}
			}
			// Point-derived summaries are authoritative when supplied; otherwise retain legacy manual counts.
			for _, b := range t.DerivedBleaching {
				if b.HC != nil {
					d.put("hc_bleach", b.Segment, num(float64(*b.HC)))
				}
				if b.SC != nil {
					d.put("sc_bleach", b.Segment, num(float64(*b.SC)))
				}
			}
		}
		for _, r := range t.Belt {
			if r.Aggregate {
				continue
			}
			key := beltKey(r)
			if key != "" {
				var v *float64
				if observed(r.RecordStatus) {
					v = num(float64(r.Count))
				}
				d.put(key, r.Segment, v)
			}
		}
		for _, r := range t.Impacts {
			key := impactKey(r)
			if key == "" {
				continue
			}
			var v *float64
			if observed(r.RecordStatus) {
				v = num(r.RawValue)
			}
			d.put(key, r.Segment, v)
			if v != nil && (r.HasRawCount || r.ValueType == "count") {
				level := 0.0
				if *v == 1 {
					level = 1
				} else if *v > 1 && *v <= 4 {
					level = 2
				} else if *v > 4 {
					level = 3
				}
				d.put(key+"_level", r.Segment, num(level))
			} else if r.ValueType == "level" {
				d.put(key+"_level", r.Segment, v)
			}
		}
	}
	// Combine only actual size rows; an absent small-size row is never assumed to be zero.
	for _, base := range []string{"butterfly", "sweetlips", "snapper"} {
		if _, adult := d.values[base+"_combined"]; adult {
			if _, small := d.values[base+"_small"]; small {
				d.values[base+"_large"] = d.values[base+"_combined"]
				delete(d.values, base+"_combined")
			}
		}
		if _, ok := d.values[base+"_combined"]; ok {
			d.values[base+"_total"] = d.values[base+"_combined"]
		} else {
			d.sum(base+"_total", base+"_large", base+"_small")
		}
	}
	d.sum("grouper_large", "grouper_30_40", "grouper_40_50", "grouper_50_60", "grouper_60")
	if _, ok := d.values["grouper_combined"]; ok {
		d.values["grouper_total"] = d.values["grouper_combined"]
	} else {
		d.sum("grouper_total", "grouper_small", "grouper_30_40", "grouper_40_50", "grouper_50_60", "grouper_60")
	}
	d.sum("clam_total", "clam_small", "clam_10_20", "clam_20_30", "clam_30_40", "clam_40_50", "clam_50")
	d.sum("long_spined", "diadema", "echinothrix")
	return d
}

var substrateCodes = []string{"HC", "SC", "RKC", "NIA", "SP", "RC", "RB", "SD", "SI", "OT"}

func normalized(s string) string {
	return strings.NewReplacer(" ", "", "≥", ">=", "＞", ">", "＜", "<", "–", "-", "—", "-", "（", "(", "）", ")").Replace(strings.ToLower(s))
}
func beltKey(r service.ReefDataBelt) string {
	name, size := normalized(r.NameEN), normalized(r.Size)
	zh := r.NameZH
	small := strings.Contains(size, "<") && !strings.Contains(size, ">=")
	if strings.Contains(name, "butterfly") || strings.Contains(zh, "蝴蝶魚") {
		if size == "" {
			return "butterfly_combined"
		}
		if small {
			return "butterfly_small"
		}
		return "butterfly_large"
	}
	if strings.Contains(name, "haemul") || strings.Contains(name, "sweetlip") || strings.Contains(zh, "石鱸") || strings.Contains(zh, "妞妞") {
		if strings.Contains(name, "juv") || strings.Contains(zh, "妞妞") || strings.Contains(zh, "幼") || small {
			return "sweetlips_small"
		}
		if size == "" {
			return "sweetlips_combined"
		}
		return "sweetlips_large"
	}
	if strings.Contains(name, "snapper") || strings.Contains(zh, "笛鯛") {
		if size == "" {
			return "snapper_combined"
		}
		if small {
			return "snapper_small"
		}
		return "snapper_large"
	}
	for _, v := range [][3]string{{"barramundi", "老鼠斑", "barramundi"}, {"humphead", "蘇眉", "humphead"}, {"bumphead", "隆頭鸚哥", "bumphead"}, {"moray", "裸胸鯙", "moray"}, {"bandedcoralshrimp", "清潔蝦", "shrimp"}, {"diadema", "魔鬼海膽", "diadema"}, {"echinothrix", "刺棘海膽", "echinothrix"}, {"pencilurchin", "鉛筆海膽", "pencil"}, {"collectorurchin", "馬糞海膽", "collector"}, {"seacucumber", "海參", "cucumber"}, {"crown-of-thorns", "棘冠海星", "crown"}, {"triton", "大法螺", "triton"}, {"lobster", "龍蝦", "lobster"}, {"shark", "鯊魚", "sharks"}, {"turtle", "海龜", "turtles"}, {"manta", "魟", "mantas"}} {
		if strings.Contains(name, v[0]) || strings.Contains(zh, v[1]) {
			return v[2]
		}
	}
	if strings.Contains(zh, "刺冠海膽") {
		return "echinothrix"
	}
	if strings.Contains(zh, "櫻花蝦") {
		return "shrimp"
	}
	if strings.Contains(name, "parrot") || strings.Contains(zh, "鸚哥") {
		if small {
			return "parrot_small"
		}
		if size == "" {
			return "parrot_combined"
		}
		return "parrot_large"
	}
	if strings.Contains(name, "grouper") || strings.Contains(zh, "石斑") {
		if size == "" {
			return "grouper_combined"
		}
		if small {
			return "grouper_small"
		}
		for _, v := range []string{"30-40", "40-50", "50-60"} {
			if strings.Contains(size, v) {
				return "grouper_" + strings.ReplaceAll(v, "-", "_")
			}
		}
		if strings.Contains(size, "60") {
			return "grouper_60"
		}
	}
	if strings.Contains(name, "giantclam") || strings.Contains(zh, "硨磲") {
		if small {
			return "clam_small"
		}
		for _, v := range []string{"10-20", "20-30", "30-40", "40-50"} {
			if strings.Contains(size, v) {
				return "clam_" + strings.ReplaceAll(v, "-", "_")
			}
		}
		if strings.Contains(size, "50") {
			return "clam_50"
		}
	}
	if r.Group == "rare" && strings.EqualFold(r.NameEN, "Other") {
		return "rare_other"
	}
	return ""
}
func impactKey(r service.ReefDataImpact) string {
	n := normalized(r.NameEN) + r.NameZH
	for _, v := range [][3]string{{"anchor", "船錨", "anchor"}, {"dynamite", "炸魚", "dynamite"}, {"other", "其他", "damage_other"}, {"fishnet", "漁業", "fishnet"}, {"fishnets", "漁網", "fishnet"}, {"general", "一般垃圾", "trash"}, {"population", "總體", "bleach_population"}, {"colony", "群體", "bleach_colony"}, {"blackband", "黑帶", "black_band"}, {"whiteband", "白帶", "white_band"}} {
		if strings.Contains(n, v[0]) || strings.Contains(n, v[1]) {
			return v[2]
		}
	}
	return ""
}
func excelDate(s string) any {
	t, e := time.Parse("2006-01-02", s)
	if e != nil {
		return nil
	}
	return t.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24
}
func excelTime(s string) any {
	s = strings.ReplaceAll(s, "-", ":")
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, e := time.Parse(layout, s); e == nil {
			return float64(t.Hour()*3600+t.Minute()*60+t.Second()) / 86400
		}
	}
	return nil
}
func names(t *service.ReefDataTransect, role string) []string {
	var out []string
	if t == nil {
		return out
	}
	for _, p := range t.Participants {
		if p.Role == role {
			n := p.NameZH
			if n == "" {
				n = p.UserName
			}
			if n == "" {
				n = p.NameEN
			}
			if n != "" {
				out = append(out, n)
			}
		}
	}
	return out
}
func joinedNames(t *service.ReefDataTransect, role string) string {
	return strings.Join(names(t, role), "、")
}
func recorders(s *templateSheet, t *service.ReefDataTransect, a, b string) {
	ns := names(t, "member")
	s.set(a, nil)
	s.set(b, nil)
	if len(ns) > 0 {
		s.set(a, ns[0])
	}
	if len(ns) > 1 {
		s.set(b, strings.Join(ns[1:], "、"))
	}
}
func metadata(d *eventData) (weather string, air, lat, lon any) {
	e := d.detail.Event
	lat, lon = value(e.Latitude), value(e.Longitude)
	if e.EntryMetadata != nil {
		weather = e.EntryMetadata.Weather
		air = value(e.EntryMetadata.AirTemperature)
		if e.EntryMetadata.Latitude != nil {
			lat, lon = value(e.EntryMetadata.Latitude), value(e.EntryMetadata.Longitude)
		}
	}
	return
}
func inputToken(code string) any {
	codes := map[string]string{"HC": "1", "SC": "2", "RKC": "3", "NIA": "4", "SP": "5", "RC": "6", "RB": "7", "SD": "8", "SI": "9", "OT": "0", "HC-a": "a", "HC-b": "b", "HC-c": "c", "SI(HC)": "91", "SI(SC)": "92", "SI(RKC)": "93", "SI(NIA)": "94", "SI(SP)": "95", "SI(RC)": "96", "SI(RB)": "97", "SI(SD)": "98", "SI(HC-a)": "9a", "SI(HC-b)": "9b", "SI(HC-c)": "9c"}
	if v, ok := codes[code]; ok {
		if n, e := strconv.Atoi(v); e == nil {
			return n
		}
		return v
	}
	if code == "" {
		return nil
	}
	return code
}
func refs(row int, cols ...string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c + strconv.Itoa(row)
	}
	return out
}
func statsCells(s *templateSheet, row int, inputCols []string, outputCols []string, vals segmentValues) {
	var present []float64
	for _, v := range vals {
		if v != nil {
			present = append(present, *v)
		}
	}
	total, mean, sd, se := stats(present)
	r := strings.Join(refs(row, inputCols...), ",")
	functions := []string{"SUM", "AVERAGE", "STDEV", "STDEV"}
	values := []any{total, mean, sd, se}
	for i, col := range outputCols {
		min := 1
		if i >= 2 {
			min = 2
		}
		f := fmt.Sprintf(`IF(COUNT(%s)<%d,"",%s(%s))`, r, min, functions[i], r)
		if i == 3 {
			f = fmt.Sprintf(`IF(COUNT(%s)<2,"",STDEV(%s)/SQRT(COUNT(%s)))`, r, r, r)
		}
		s.formula(col+strconv.Itoa(row), f, values[i])
	}
}
func setSeries(s *templateSheet, row int, cols []string, vals segmentValues) {
	for i, col := range cols {
		s.set(col+strconv.Itoa(row), value(vals[i]))
	}
}
func guardedSum(s *templateSheet, ref string, members []string, v any) {
	s.formula(ref, fmt.Sprintf(`IF(COUNT(%s)=%d,SUM(%s),"")`, strings.Join(members, ","), len(members), strings.Join(members, ",")), v)
}
func meanSE(s *templateSheet, row int, key string, d *eventData, meanFormula, seFormula string) {
	_, m, _, se := d.summary(key)
	s.formula("B"+strconv.Itoa(row), meanFormula, m)
	s.formula("C"+strconv.Itoa(row), seFormula, se)
}
func eventTitle(e service.ReefDataEvent) string {
	return fmt.Sprintf("%s %s %gm", e.SiteName, e.SurveyDate, e.DepthM)
}
func (b *templateBook) titles(prefix string, labels map[int]string) {
	for n, label := range labels {
		p := fmt.Sprintf("xl/charts/chart%d.xml", n)
		if _, ok := b.parts[p]; ok {
			b.title(p, prefix+" · "+label)
		}
	}
}
func lineTemplate(d *eventData) ([]byte, error) {
	b, err := openTemplate("line")
	if err != nil {
		return nil, err
	}
	s := b.byName["DATA"]
	e, t := d.detail.Event, d.line
	if t == nil {
		return nil, fmt.Errorf("%w: 未記錄底質", service.ErrValidation)
	}
	s.set("C1", e.SiteName)
	s.set("C2", e.DepthM)
	s.set("C3", joinedNames(t, "team_leader"))
	s.set("C4", excelTime(e.EventTime))
	s.set("K2", excelDate(e.SurveyDate))
	recorders(s, t, "L3", "N3")
	s.set("L4", value(t.VisibilityMax))
	s.set("L5", value(t.WaterTemp))
	weather, air, lat, lon := metadata(d)
	s.set("C8", weather)
	s.set("L6", air)
	s.set("C7", lat)
	s.set("L7", lon)
	s.set("C42", value(t.Comments))
	s.set("I41", value(t.RKCNote))
	s.set("L41", nil)
	s.set("C6", nil)
	classified := t.ClassifyHardCoral || t.CoralMorphology == "classified"
	s.set("C5", "否")
	if classified {
		s.set("C5", "是")
	}
	for _, p := range t.Points {
		if p.Layer != "surface" || p.Segment < 1 || p.Segment > 4 {
			continue
		}
		offset := int(math.Round((p.Position - float64((p.Segment-1)*25)) * 2))
		if offset < 0 || offset >= 40 {
			continue
		}
		col := (p.Segment-1)*4 + 2
		if offset >= 20 {
			col += 2
			offset -= 20
		}
		s.set(column(col)+strconv.Itoa(offset+15), inputToken(p.Code))
	}
	for i, morph := range []string{"a", "b", "c"} {
		row := 23 + i
		s.set("Q"+strconv.Itoa(row), "9"+morph)
		s.set("R"+strconv.Itoa(row), "泥沙(硬珊瑚-"+morph+")")
		s.set("S"+strconv.Itoa(row), "SI(HC-"+morph+")")
	}
	s.raw = strings.ReplaceAll(s.raw, "$Q$2:$Q$22", "$Q$2:$Q$25")
	s.raw = strings.ReplaceAll(s.raw, "$S$2:$S$22", "$S$2:$S$25")
	rawCodes := append(append([]string{}, substrateCodes...), "HC-a", "HC-b", "HC-c", "SI(HC)", "SI(SC)", "SI(RKC)", "SI(NIA)", "SI(SP)", "SI(RC)", "SI(RB)", "SI(SD)")
	for index, code := range rawCodes {
		row := 47 + index
		var vals segmentValues
		for seg := 0; seg < 4; seg++ {
			ref := column(seg*2+2) + strconv.Itoa(row)
			if d.valid[seg] > 0 {
				count := d.rawCounts[code][seg]
				if code == "SI(HC)" {
					for _, m := range []string{"a", "b", "c"} {
						count += d.rawCounts["SI(HC-"+m+")"][seg]
					}
				}
				vals[seg] = num(count)
			}
			s.formula(ref, `IF(`+validPointsFormula(seg)+`=0,"",`+pointCountFormula(seg, code)+`)`, value(vals[seg]))
		}
		statsCells(s, row, []string{"B", "D", "F", "H"}, []string{"J", "M", "O"}, vals)
	}
	// Canonical category counts include morphology and surface silt composites exactly once.
	for i, code := range substrateCodes {
		row := 73 + i
		vals := d.values["count_"+code]
		for seg, col := range []string{"B", "D", "F", "H"} {
			terms := []string{}
			for rawCode := range pointTokens {
				base, _, _ := substrate(rawCode)
				if base == code {
					if code == "SI" && strings.HasPrefix(rawCode, "SI(HC-") {
						continue
					}
					terms = append(terms, pointCountFormula(seg, rawCode))
				}
			}
			sort.Strings(terms)
			s.formula(col+strconv.Itoa(row), `IF(`+validPointsFormula(seg)+`=0,"",`+strings.Join(terms, "+")+`)`, value(vals[seg]))
		}
		statsCells(s, row, []string{"B", "D", "F", "H"}, []string{"J", "M", "O"}, vals)
	}
	for seg, n := range d.valid {
		for _, row := range []int{68, 83} {
			s.formula(column(seg*2+2)+strconv.Itoa(row), validPointsFormula(seg), n)
		}
	}
	for row, key := range map[int]string{37: "hc_bleach", 38: "sc_bleach"} {
		vals := d.values[key]
		setSeries(s, row, []string{"C", "D", "E", "F"}, vals)
		statsCells(s, row, []string{"C", "D", "E", "F"}, []string{"G", "I", "K"}, vals)
	}
	// Disease counts are not collected per point; never inherit the example's zeros.
	for _, c := range []string{"C", "D", "E", "F", "G", "I", "K"} {
		s.set(c+"39", nil)
	}
	totalHC, _, _, _ := d.summary("count_HC")
	totalSC, _, _, _ := d.summary("count_SC")
	s.cache("R42", totalHC)
	s.cache("R43", totalSC)
	helperKeys := append(append([]string{}, substrateCodes...), "HC-a", "HC-b", "HC-c")
	for i, code := range helperKeys {
		sourceRow := 73 + i
		if i >= 10 {
			sourceRow = 57 + i - 10
		}
		helperRow := 86 + i
		for seg, col := range []string{"B", "D", "F", "H"} {
			v := value(d.values["line_"+code][seg])
			s.formula(col+strconv.Itoa(helperRow), fmt.Sprintf(`IF(%s83>0,%s%d/%s83*100,"")`, col, col, sourceRow, col), v)
		}
	}
	g := b.byName["GRAPHS"]
	for i, code := range substrateCodes {
		lineGraphCells(g, 5+i, code, d)
	}
	for i, code := range []string{"HC", "SC", "NIA", "SP", "OT"} {
		lineGraphCells(g, 20+i, code, d)
	}
	for i, code := range []string{"RKC", "RC", "RB", "SD", "SI"} {
		lineGraphCells(g, 29+i, code, d)
	}
	sum := 0.0
	var valid bool
	for _, code := range substrateCodes {
		_, m, _, _ := d.summary("line_" + code)
		if m != nil {
			sum += m.(float64)
			valid = true
		}
	}
	if valid {
		g.formula("B16", `IF(COUNT(B5:B14)=0,"",SUM(B5:B14))`, sum)
	} else {
		g.formula("B16", `IF(COUNT(B5:B14)=0,"",SUM(B5:B14))`, nil)
	}
	for row, key := range map[int]string{39: "HC", 40: "SC"} {
		den := d.values["count_"+key]
		bleached := d.values[strings.ToLower(key)+"_bleach"]
		a, bv := 0.0, 0.0
		for seg, v := range bleached {
			if v != nil && den[seg] != nil {
				a += *v
				bv += *den[seg]
			}
		}
		var ratio, normal any
		if bv > 0 {
			ratio = a / bv
			normal = 1 - a/bv
		}
		bleachRow := 37
		if key == "SC" {
			bleachRow = 38
		}
		countRow := 73
		if key == "SC" {
			countRow = 74
		}
		var denom []string
		for i, col := range []string{"C", "D", "E", "F"} {
			denom = append(denom, fmt.Sprintf(`IF(ISNUMBER(DATA!%s%d),DATA!%s%d,0)`, col, bleachRow, []string{"B", "D", "F", "H"}[i], countRow))
		}
		f := fmt.Sprintf(`IFERROR(SUM(DATA!C%d:F%d)/(%s),"")`, bleachRow, bleachRow, strings.Join(denom, "+"))
		g.formula("C"+strconv.Itoa(row), f, ratio)
		g.formula("B"+strconv.Itoa(row), fmt.Sprintf(`IF(ISNUMBER(C%d),1-C%d,"")`, row, row), normal)
	}
	// Preserve the original morphology sheet, excluding the removed mud-underlayer plots.
	gm := b.byName["GRAPHS_SI"]
	for i, code := range rawCodes {
		row := 105 + i
		var vals segmentValues
		for seg, col := range []string{"B", "D", "F", "H"} {
			count := d.rawCounts[code][seg]
			if code == "SI(HC)" {
				for _, m := range []string{"a", "b", "c"} {
					count += d.rawCounts["SI(HC-"+m+")"][seg]
				}
			}
			if d.valid[seg] > 0 {
				vals[seg] = num(count / float64(d.valid[seg]) * 100)
			}
			s.formula(col+strconv.Itoa(row), fmt.Sprintf(`IF(%s83>0,%s%d/%s83*100,"")`, col, col, 47+i, col), value(vals[seg]))
		}
		_, m, _, se := statsValues(vals)
		rr := fmt.Sprintf("DATA!B%d,DATA!D%d,DATA!F%d,DATA!H%d", row, row, row, row)
		gm.formula("B"+strconv.Itoa(5+i), `IFERROR(AVERAGE(`+rr+`),"")`, m)
		gm.formula("C"+strconv.Itoa(5+i), `IF(COUNT(`+rr+`)<2,"",STDEV(`+rr+`)/SQRT(COUNT(`+rr+`)))`, se)
	}
	for i, code := range []string{"HC-a", "HC-b", "HC-c"} {
		_, m, _, se := d.summary("line_" + code)
		rr := fmt.Sprintf("DATA!B%d,DATA!D%d,DATA!F%d,DATA!H%d", 96+i, 96+i, 96+i, 96+i)
		gm.formula("B"+strconv.Itoa(31+i), `IFERROR(AVERAGE(`+rr+`),"")`, m)
		gm.formula("C"+strconv.Itoa(31+i), `IF(COUNT(`+rr+`)<2,"",STDEV(`+rr+`)/SQRT(COUNT(`+rr+`)))`, se)
	}
	gm.formula("B27", `IF(COUNT(B5:B25)=0,"",SUM(B5:B25))`, g.get("B16"))
	for _, gs := range []*templateSheet{g, gm} {
		gs.set("E6", "Mean Percent cover of Substrate for "+eventTitle(e))
	}
	fillLineExport(b.byName["EXPORT"], d)
	b.titles(eventTitle(e), map[int]string{1: "底質覆蓋率", 2: "活底質覆蓋率", 3: "非生物底質覆蓋率", 4: "活底質比例", 5: "非生物底質比例", 6: "珊瑚白化比例", 7: "表層底質分類", 8: "硬珊瑚型態覆蓋率", 10: "硬珊瑚型態比例"})
	return b.bytes()
}
func statsValues(vals segmentValues) (total, mean, sd, se any) {
	var v []float64
	for _, x := range vals {
		if x != nil {
			v = append(v, *x)
		}
	}
	return stats(v)
}
func sumValues(vals segmentValues) any { t, _, _, _ := statsValues(vals); return t }
func lineGraphCells(g *templateSheet, row int, code string, d *eventData) {
	_, m, _, se := d.summary("line_" + code)
	index := 0
	for i, k := range substrateCodes {
		if k == code {
			index = i
			break
		}
	}
	r := 86 + index
	rr := fmt.Sprintf("DATA!B%d,DATA!D%d,DATA!F%d,DATA!H%d", r, r, r, r)
	g.formula("B"+strconv.Itoa(row), `IFERROR(AVERAGE(`+rr+`),"")`, m)
	g.formula("C"+strconv.Itoa(row), `IF(COUNT(`+rr+`)<2,"",STDEV(`+rr+`)/SQRT(COUNT(`+rr+`)))`, se)
}
func fillLineExport(s *templateSheet, d *eventData) {
	e := d.detail.Event
	dt, _ := time.Parse("2006-01-02", e.SurveyDate)
	_, _, lat, lon := metadata(d)
	s.set("BM1", "event_id")
	for seg := 0; seg < 4; seg++ {
		r := strconv.Itoa(seg + 2)
		for col, v := range map[string]any{"A": dt.Year(), "B": int(dt.Month()), "C": dt.Day(), "D": e.SiteEnglish, "E": e.SiteName, "F": lon, "G": lat, "H": e.DepthM, "S": value(d.line.Comments), "T": joinedNames(d.line, "member"), "U": seg + 1, "BM": e.EventID} {
			s.set(col+r, v)
		}
		for i, code := range substrateCodes {
			var v any
			if p := d.values["line_"+code][seg]; p != nil {
				v = *p / 100
			}
			s.set(column(9+i)+r, v)
		}
	}
	// Refresh the existing morphology/bleaching export block and metadata row.
	for seg := 0; seg < 4; seg++ {
		row := 18 + seg
		s.set("A"+strconv.Itoa(row), seg+1)
		for j, code := range []string{"HC-a", "HC-b", "HC-c", "SI(HC)", "SI(SC)", "SI(RKC)", "SI(NIA)", "SI(SP)", "SI(RC)", "SI(RB)", "SI(SD)"} {
			var v any
			if d.valid[seg] > 0 {
				v = d.rawCounts[code][seg] / float64(d.valid[seg])
			}
			s.set(column(4+j)+strconv.Itoa(row), v)
		}
		for i, key := range []string{"hc", "sc"} {
			var v any
			n, den := d.values[key+"_bleach"][seg], d.values["count_"+strings.ToUpper(key)][seg]
			if n != nil && den != nil && *den > 0 {
				v = *n / *den
			}
			s.set(column(15+i)+strconv.Itoa(row), v)
		}
		s.set("Q"+strconv.Itoa(row), nil)
	}
	s.set("B18", value(d.line.RKCNote))
	s.set("C18", nil)
	for _, c := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		s.set(c+"28", nil)
	}
	weather, air, _, _ := metadata(d)
	s.set("A28", d.detail.Event.EventTime)
	s.set("B28", value(d.line.VisibilityMax))
	s.set("C28", value(d.line.WaterTemp))
	s.set("D28", air)
	s.set("E28", d.line.CoralMorphology)
	s.set("G28", weather)
	// Headquarters grid uses base substrate only. Clear missing input positions first.
	for _, col := range []string{"B", "D", "F", "H", "J", "L", "N", "P"} {
		for row := 44; row <= 63; row++ {
			s.set(col+strconv.Itoa(row), nil)
		}
	}
	// Morphology stays in the block above.
	for _, p := range d.line.Points {
		if p.Layer != "surface" || p.Segment < 1 || p.Segment > 4 {
			continue
		}
		offset := int(math.Round((p.Position - float64((p.Segment-1)*25)) * 2))
		if offset < 0 || offset >= 40 {
			continue
		}
		col := (p.Segment-1)*4 + 2
		if offset >= 20 {
			offset -= 20
			col += 2
		}
		base, _, _ := substrate(p.Code)
		s.set(column(col)+strconv.Itoa(44+offset), base)
	}
	// Append canonical point fields without shifting any original export cells.
	headers := []string{"event_id", "segment", "position_m", "substrate_code", "Base_Substrate", "Morphology", "Under_Substrate", "Health_Status"}
	for i, h := range headers {
		s.set(column(i+1)+"70", h)
	}
	row := 71
	for _, p := range d.line.Points {
		if p.Layer != "surface" {
			continue
		}
		base, morph, under := substrate(p.Code)
		var health any
		if p.IsBleached != nil {
			health = "Normal"
			if *p.IsBleached {
				health = "Bleached"
			}
		}
		for i, v := range []any{e.EventID, p.Segment, p.Position, p.Code, base, morph, under, health} {
			s.set(column(i+1)+strconv.Itoa(row), v)
		}
		row++
	}
}

var beltRows = map[int]string{10: "butterfly_large", 11: "butterfly_small", 12: "sweetlips_large", 13: "sweetlips_small", 14: "snapper_large", 15: "snapper_small", 16: "barramundi", 17: "humphead", 18: "bumphead", 19: "parrot_large", 20: "parrot_small", 21: "moray", 23: "grouper_small", 24: "grouper_30_40", 25: "grouper_40_50", 26: "grouper_50_60", 27: "grouper_60", 28: "grouper_total", 33: "shrimp", 34: "diadema", 35: "echinothrix", 36: "pencil", 37: "collector", 38: "cucumber", 39: "crown", 40: "triton", 41: "lobster", 44: "clam_small", 45: "clam_10_20", 46: "clam_20_30", 47: "clam_30_40", 48: "clam_40_50", 49: "clam_50", 50: "clam_total", 61: "sharks", 62: "turtles", 63: "mantas", 64: "rare_other", 68: "bleach_population", 69: "bleach_colony", 71: "black_band", 72: "white_band"}
var integratedFish = []string{"butterfly_total", "sweetlips_total", "snapper_total", "barramundi", "grouper_large", "humphead", "bumphead", "parrot_large", "moray"}
var impacts = []string{"anchor", "dynamite", "damage_other", "fishnet", "trash"}

func beltTemplate(d *eventData) ([]byte, error) {
	b, err := openTemplate("belt")
	if err != nil {
		return nil, err
	}
	s := b.byName["DATA"]
	e := d.detail.Event
	t := d.fish
	if t == nil {
		t = d.invert
	}
	if t == nil {
		return nil, fmt.Errorf("%w: 未記錄魚類或無脊椎", service.ErrValidation)
	}
	s.set("B1", e.SiteName)
	s.set("B2", e.DepthM)
	s.set("B3", excelDate(e.SurveyDate))
	s.set("B4", joinedNames(t, "team_scientist"))
	s.set("G2", joinedNames(t, "team_leader"))
	s.set("G3", excelTime(e.EventTime))
	s.set("G4", value(t.WaterTemp))
	s.set("I4", value(t.VisibilityMax))
	recorders(s, d.fish, "B7", "E7")
	recorders(s, d.invert, "B30", "E30")
	s.set("E8", "未記錄")
	if d.fish != nil {
		mode := value(d.fish.FishSizeMode)
		if mode == "split" || d.fish.FishLengthMode == "split" {
			s.set("E8", "是")
		} else {
			s.set("E8", "否")
		}
	}
	for row, key := range beltRows {
		vals := d.values[key]
		setSeries(s, row, []string{"B", "C", "D", "E"}, vals)
		if row != 68 && row != 69 {
			statsCells(s, row, []string{"B", "C", "D", "E"}, []string{"F", "G", "H"}, vals)
		}
	}
	for row, base := range map[int]string{10: "butterfly", 12: "sweetlips", 14: "snapper", 19: "parrot"} {
		if vals, ok := d.values[base+"_combined"]; ok {
			setSeries(s, row, []string{"B", "C", "D", "E"}, vals)
			statsCells(s, row, []string{"B", "C", "D", "E"}, []string{"F", "G", "H"}, vals)
			s.set("A"+strconv.Itoa(row), base+"（未分體長）")
		}
	}
	// Recreate aggregate formulas with completeness guards, so blanks never become zeros.
	for _, agg := range []struct {
		row     int
		members []int
		key     string
	}{{28, []int{23, 24, 25, 26, 27}, "grouper_total"}, {50, []int{44, 45, 46, 47, 48, 49}, "clam_total"}} {
		if agg.row == 28 {
			if vals, ok := d.values["grouper_combined"]; ok {
				setSeries(s, 28, []string{"B", "C", "D", "E"}, vals)
				continue
			}
		}
		for seg, col := range []string{"B", "C", "D", "E"} {
			var members []string
			for _, r := range agg.members {
				members = append(members, col+strconv.Itoa(r))
			}
			guardedSum(s, col+strconv.Itoa(agg.row), members, value(d.values[agg.key][seg]))
		}
	}
	for i, key := range integratedFish {
		row := 10 + i
		vals := d.values[key]
		for seg, col := range []string{"K", "L", "M", "N"} {
			input := []string{"B", "C", "D", "E"}[seg]
			var members []string
			switch key {
			case "butterfly_total", "sweetlips_total", "snapper_total":
				r := map[string]int{"butterfly_total": 10, "sweetlips_total": 12, "snapper_total": 14}[key]
				members = []string{input + strconv.Itoa(r), input + strconv.Itoa(r+1)}
				if _, ok := d.values[strings.TrimSuffix(key, "_total")+"_combined"]; ok {
					members = members[:1]
				}
			case "grouper_large":
				for _, r := range []int{24, 25, 26, 27} {
					members = append(members, input+strconv.Itoa(r))
				}
			default:
				source := map[string]int{"barramundi": 16, "humphead": 17, "bumphead": 18, "parrot_large": 19, "moray": 21}[key]
				members = []string{input + strconv.Itoa(source)}
			}
			if key == "parrot_large" {
				if _, combined := d.values["parrot_combined"]; combined {
					s.set(col+strconv.Itoa(row), nil)
					continue
				}
			}
			guardedSum(s, col+strconv.Itoa(row), members, value(vals[seg]))
		}
		statsCells(s, row, []string{"K", "L", "M", "N"}, []string{"O", "P", "Q"}, vals)
	}
	for i, key := range impacts {
		row := 55 + i
		vals := d.values[key]
		setSeries(s, row, []string{"B", "C", "D", "E"}, vals)
		statsCells(s, row, []string{"B", "C", "D", "E"}, []string{"F", "G", "H", "I"}, vals)
		for seg, col := range []string{"K", "L", "M", "N"} {
			input := []string{"B", "C", "D", "E"}[seg] + strconv.Itoa(row)
			formula := fmt.Sprintf(`IF(ISNUMBER(%s),IF(%s=0,0,IF(%s=1,1,IF(%s<=4,2,3))),"")`, input, input, input, input)
			for _, t := range d.detail.Transects {
				for _, impact := range t.Impacts {
					if impactKey(impact) == key && !impact.HasRawCount && impact.ValueType == "level" {
						formula = fmt.Sprintf(`IF(ISNUMBER(%s),%s,"")`, input, input)
					}
				}
			}
			s.formula(col+strconv.Itoa(row), formula, value(d.values[key+"_level"][seg]))
		}
		statsCells(s, row, []string{"K", "L", "M", "N"}, []string{"O", "P", "Q"}, d.values[key+"_level"])
	}
	var comments []string
	for _, t := range []*service.ReefDataTransect{d.fish, d.invert} {
		if t != nil && t.Comments != nil {
			comments = append(comments, *t.Comments)
		}
		if t != nil && t.OtherDamageNote != "" {
			comments = append(comments, t.OtherDamageNote)
		}
	}
	s.set("B66", strings.Join(comments, "\n"))
	g := b.byName["GRAPHS"]
	graphKeys := map[int]string{5: "butterfly_total", 6: "sweetlips_total", 7: "snapper_total", 8: "barramundi", 9: "grouper_large", 10: "grouper_small", 11: "humphead", 12: "bumphead", 13: "parrot_large", 14: "moray", 18: "butterfly_large", 19: "butterfly_small", 20: "sweetlips_large", 21: "sweetlips_small", 22: "snapper_large", 23: "snapper_small", 24: "parrot_large", 25: "parrot_small", 26: "grouper_large", 27: "grouper_small", 35: "shrimp", 36: "diadema", 37: "echinothrix", 38: "pencil", 39: "collector", 40: "cucumber", 41: "crown", 42: "clam_total", 43: "triton", 44: "lobster", 47: "clam_small", 48: "clam_10_20", 49: "clam_20_30", 50: "clam_30_40", 51: "clam_40_50", 52: "clam_50"}
	for row, key := range graphKeys {
		if strings.HasSuffix(key, "_large") {
			if _, combined := d.values[strings.TrimSuffix(key, "_large")+"_combined"]; combined {
				g.set("B"+strconv.Itoa(row), nil)
				g.set("C"+strconv.Itoa(row), nil)
				continue
			}
		}
		_, m, _, se := d.summary(key)
		sourceRow := 0
		for r, k := range beltRows {
			if k == key {
				sourceRow = r
				break
			}
		}
		meanCol, sdCol := "G", "H"
		inputCols := []string{"B", "C", "D", "E"}
		for i, k := range integratedFish {
			if k == key {
				sourceRow = 10 + i
				meanCol, sdCol = "P", "Q"
				inputCols = []string{"K", "L", "M", "N"}
				break
			}
		}
		if sourceRow > 0 {
			rr := strings.Join(refs(sourceRow, inputCols...), ",")
			rr = strings.ReplaceAll(rr, ",", ",DATA!")
			rr = "DATA!" + rr
			g.formula("B"+strconv.Itoa(row), fmt.Sprintf(`IF(ISNUMBER(DATA!%s%d),DATA!%s%d,"")`, meanCol, sourceRow, meanCol, sourceRow), m)
			g.formula("C"+strconv.Itoa(row), fmt.Sprintf(`IF(COUNT(%s)<2,"",DATA!%s%d/SQRT(COUNT(%s)))`, rr, sdCol, sourceRow, rr), se)
		} else {
			g.set("B"+strconv.Itoa(row), m)
			g.set("C"+strconv.Itoa(row), se)
		}
	}
	for i, key := range impacts {
		_, m, _, se := d.summary(key + "_level")
		row := 55 + i
		rr := fmt.Sprintf("DATA!K%d,DATA!L%d,DATA!M%d,DATA!N%d", row, row, row, row)
		g.formula("B"+strconv.Itoa(row), fmt.Sprintf(`IF(ISNUMBER(DATA!P%d),DATA!P%d,"")`, row, row), m)
		g.formula("C"+strconv.Itoa(row), fmt.Sprintf(`IF(COUNT(%s)<2,"",DATA!Q%d/SQRT(COUNT(%s)))`, rr, row, rr), se)
	}
	for row, key := range map[int]string{62: "bleach_population", 66: "bleach_colony", 70: "black_band", 71: "white_band"} {
		_, m, _, _ := d.summary(key)
		g.set("B"+strconv.Itoa(row), m)
		if row == 62 || row == 66 {
			var complement any
			if m != nil {
				complement = 100 - m.(float64)
			}
			g.set("B"+strconv.Itoa(row+1), complement)
		}
	}
	g.cache("B16", s.get("E8"))
	g.set("E7", "Reef Check Fish Graphs for "+eventTitle(e))
	g.set("E49", "Reef Check Invertebrates Graphs for "+eventTitle(e))
	g.set("E81", "Reef Check Impacts, Bleaching and Disease for "+eventTitle(e))
	s.set("A42", "Long-spined urchin 長刺海膽（魔鬼＋刺棘）")
	setSeries(s, 42, []string{"B", "C", "D", "E"}, d.values["long_spined"])
	for i, col := range []string{"B", "C", "D", "E"} {
		guardedSum(s, col+"42", []string{col + "34", col + "35"}, value(d.values["long_spined"][i]))
	}
	statsCells(s, 42, []string{"B", "C", "D", "E"}, []string{"F", "G", "H"}, d.values["long_spined"])
	g.set("A45", "Long-spined urchin 長刺海膽")
	_, longMean, _, longSE := d.summary("long_spined")
	g.formula("B45", `IFERROR(DATA!G42,"")`, longMean)
	g.formula("C45", `IF(COUNT(DATA!B42:E42)<2,"",DATA!H42/SQRT(COUNT(DATA!B42:E42)))`, longSE)
	b.parts["xl/charts/chart7.xml"] = []byte(strings.ReplaceAll(string(b.parts["xl/charts/chart7.xml"]), "$44", "$45"))
	fillBeltExport(b.byName["EXPORT"], d)
	b.titles(eventTitle(e), map[int]string{1: "魚類族群密度", 2: "環境衝擊程度", 3: "珊瑚總體白化比例", 4: "珊瑚群體白化比例", 5: "珊瑚疾病比例", 6: "魚類尺寸", 7: "無脊椎動物族群密度", 8: "硨磲貝尺寸"})
	return b.bytes()
}
func fillBeltExport(s *templateSheet, d *eventData) {
	e := d.detail.Event
	dt, _ := time.Parse("2006-01-02", e.SurveyDate)
	_, _, lat, lon := metadata(d)
	s.set("BT1", "event_id")
	s.set("BU1", "TaiBIF_event_id")
	for i, key := range []string{"long_spined", "butterfly_combined", "sweetlips_combined", "snapper_combined", "parrot_combined", "grouper_combined"} {
		col := column(74 + i)
		s.set(col+"1", key)
		for seg, v := range d.values[key] {
			s.set(col+strconv.Itoa(seg+2), value(v))
		}
	}
	for seg := 0; seg < 4; seg++ {
		row := strconv.Itoa(seg + 2)
		for col, v := range map[string]any{"A": dt.Year(), "B": int(dt.Month()), "C": dt.Day(), "D": e.County, "E": e.Location, "F": e.SiteName, "G": lon, "H": lat, "I": e.DepthM, "V": seg + 1, "AO": joinedNames(d.fish, "member"), "BS": joinedNames(d.invert, "member"), "BT": e.EventID, "BU": fmt.Sprintf("%s_S%d", e.EventID, seg+1)} {
			s.set(col+row, v)
		}
	}
	// The template's EXPORT headings are authoritative; fill by heading to prevent shifted columns.
	headerMap := map[string]string{"butterflyfish": "butterfly_large", "butterflyfish_less5": "butterfly_small", "sweetlips": "sweetlips_large", "sweetlips_juv": "sweetlips_small", "snapper": "snapper_large", "snapper_less20": "snapper_small", "barramundi cod": "barramundi", "humphead wrasse": "humphead", "bumphead parrotfish": "bumphead", "otherParrotfish": "parrot_large", "otherParrotfish_less20": "parrot_small", "moray eel": "moray", "grouper_less30": "grouper_small", "grouper3040": "grouper_30_40", "grouper4050": "grouper_40_50", "grouper5060": "grouper_50_60", "grouper60": "grouper_60", "banded coral shrimp": "shrimp", "diadema": "diadema", "Echinothrix": "echinothrix", "pencil urchin": "pencil", "collector urchin": "collector", "seacucumber": "cucumber", "crown of thorns": "crown", "triton": "triton", "lobster": "lobster", "giantclam_less10": "clam_small", "giantclam1020": "clam_10_20", "giantclam2030": "clam_20_30", "giantclam3040": "clam_30_40", "giantclam4050": "clam_40_50", "giantclam50": "clam_50", "boat anchor": "anchor", "dynamite": "dynamite", "other coral damage": "damage_other", "fishnet trash": "fishnet", "general trash": "trash", "bleaching%population": "bleach_population", "bleaching%colony": "bleach_colony", "disease%coral black band": "black_band", "disease%coral white band": "white_band", "sharks": "sharks", "turtles": "turtles", "mantas": "mantas", "other": "rare_other"}
	for c := 1; c <= 71; c++ {
		col := column(c)
		header := fmt.Sprint(s.get(col + "1"))
		key, ok := headerMap[header]
		if !ok {
			continue
		}
		for seg := 0; seg < 4; seg++ {
			v := value(d.values[key][seg])
			if strings.HasSuffix(key, "_large") {
				if _, ok := d.values[strings.TrimSuffix(key, "_large")+"_combined"]; ok {
					v = nil
				}
			}
			s.set(col+strconv.Itoa(seg+2), v)
		}
	}
	// Full raw rows retain custom taxa, recording states and rare-animal size/notes.
	for i, h := range []string{"event_id", "TaiBIF_event_id", "segment", "taxon_group", "name_zh", "name_en", "size_class", "count", "record_status", "observed_size", "notes"} {
		s.set(column(i+1)+"12", h)
	}
	row := 13
	for _, t := range []*service.ReefDataTransect{d.fish, d.invert} {
		if t == nil {
			continue
		}
		for _, r := range t.Belt {
			if r.Aggregate {
				continue
			}
			for i, v := range []any{e.EventID, fmt.Sprintf("%s_S%d", e.EventID, r.Segment), r.Segment, r.Group, r.NameZH, r.NameEN, r.Size, recordedCount(r.Count, r.RecordStatus), r.RecordStatus, r.ObservedSize, r.Notes} {
				s.set(column(i+1)+strconv.Itoa(row), v)
			}
			row++
		}
	}
}

// Ensure deterministic date/time ordering without combining distinct gas-cylinder events.
func sortedEvents(details []service.ReefDataDetail) []*eventData {
	out := make([]*eventData, 0, len(details))
	for _, d := range details {
		out = append(out, buildEventData(d))
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].detail.Event, out[j].detail.Event
		if a.SurveyDate != b.SurveyDate {
			return a.SurveyDate < b.SurveyDate
		}
		if a.EventTime != b.EventTime {
			return a.EventTime < b.EventTime
		}
		return a.ID < b.ID
	})
	return out
}

var pointTokens = map[string]string{"HC": "1", "SC": "2", "RKC": "3", "NIA": "4", "SP": "5", "RC": "6", "RB": "7", "SD": "8", "SI": "9", "OT": "0", "HC-a": "a", "HC-b": "b", "HC-c": "c", "SI(HC)": "91", "SI(SC)": "92", "SI(RKC)": "93", "SI(NIA)": "94", "SI(SP)": "95", "SI(RC)": "96", "SI(RB)": "97", "SI(SD)": "98", "SI(HC-a)": "9a", "SI(HC-b)": "9b", "SI(HC-c)": "9c"}

func segmentRanges(seg int) (string, string) {
	return fmt.Sprintf("%s15:%s34", column(seg*4+2), column(seg*4+2)), fmt.Sprintf("%s15:%s34", column(seg*4+4), column(seg*4+4))
}
func validPointsFormula(seg int) string {
	a, b := segmentRanges(seg)
	return fmt.Sprintf(`COUNTA(%s,%s)-COUNTIF(%s,"NA")-COUNTIF(%s,"NA")`, a, b, a, b)
}
func pointCountFormula(seg int, code string) string {
	a, b := segmentRanges(seg)
	token := pointTokens[code]
	if code == "SI(HC)" {
		return fmt.Sprintf(`COUNTIF(%s,"91")+COUNTIF(%s,"91")+COUNTIF(%s,"9a")+COUNTIF(%s,"9a")+COUNTIF(%s,"9b")+COUNTIF(%s,"9b")+COUNTIF(%s,"9c")+COUNTIF(%s,"9c")`, a, b, a, b, a, b, a, b)
	}
	return fmt.Sprintf(`COUNTIF(%s,"%s")+COUNTIF(%s,"%s")`, a, token, b, token)
}
