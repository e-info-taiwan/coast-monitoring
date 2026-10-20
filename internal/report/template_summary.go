package report

import (
	"archive/zip"
	"bytes"
	"coast-monitoring/internal/service"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var fishSummaryKeys = []string{"butterfly_total", "sweetlips_total", "snapper_total", "barramundi", "grouper_large", "grouper_small", "humphead", "bumphead", "parrot_large", "moray"}
var fishSizeKeys = []string{"butterfly_large", "butterfly_small", "sweetlips_large", "sweetlips_small", "snapper_large", "snapper_small", "parrot_large", "parrot_small", "grouper_large", "grouper_small"}
var invertSummaryKeys = []string{"shrimp", "diadema", "echinothrix", "pencil", "collector", "cucumber", "crown", "clam_total", "triton", "lobster"}

func summaryTemplate(data []*eventData) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: 沒有報告資料", service.ErrValidation)
	}
	b, err := openTemplate("summary")
	if err != nil {
		return nil, err
	}
	var depths []float64
	for _, d := range data {
		depth := d.detail.Event.DepthM
		if !containsDepth(depths, depth) {
			depths = append(depths, depth)
		}
	}
	sort.Float64s(depths)
	if len(depths) > 2 {
		return nil, fmt.Errorf("%w: Summary 範本每份可比較兩個深度，請下載 ZIP 套組", service.ErrValidation)
	}
	latest := data[len(data)-1].detail.Event.SurveyDate
	sheetName := strings.ReplaceAll(latest, "-", "")
	if len(sheetName) != 8 {
		sheetName = "本次調查"
	}
	b.rename("20260411", sheetName)
	s := b.byName[sheetName]
	headings := []int{1, 28, 41, 68, 93, 115, 140, 161}
	for i := 0; i < 2; i++ {
		col := column(i + 2)
		for _, row := range headings {
			var v any
			if i < len(depths) {
				v = fmt.Sprintf("%gm", depths[i])
			}
			s.set(col+strconv.Itoa(row), v)
		}
	}
	lineKeys := make([]string, 0, 11)
	for _, k := range append(append([]string{}, substrateCodes...), "live_coral") {
		lineKeys = append(lineKeys, "line_"+k)
	}
	blocks := []struct {
		mean, se int
		keys     []string
	}{{2, 15, lineKeys}, {29, 34, []string{"line_HC-a", "line_HC-b", "line_HC-c"}}, {42, 55, fishSummaryKeys}, {69, 81, fishSizeKeys}, {94, 101, []string{"grouper_small", "grouper_30_40", "grouper_40_50", "grouper_50_60", "grouper_60"}}, {116, 128, invertSummaryKeys}, {141, 149, []string{"clam_small", "clam_10_20", "clam_20_30", "clam_30_40", "clam_40_50", "clam_50"}}, {162, 169, []string{"anchor_level", "dynamite_level", "damage_other_level", "fishnet_level", "trash_level"}}}
	for i, depth := range depths {
		var selected *eventData
		for _, d := range data {
			if d.detail.Event.DepthM == depth && d.detail.Event.SurveyDate == latest {
				selected = d
			}
		}
		if selected == nil {
			continue
		}
		for _, block := range blocks {
			for j, key := range block.keys {
				_, m, _, se := selected.summary(key)
				s.set(column(i+2)+strconv.Itoa(block.mean+j), m)
				if block.se+j <= 24 || block.mean != 2 {
					s.set(column(i+2)+strconv.Itoa(block.se+j), se)
				}
			}
		}
	}
	s.set("A126", "長刺海膽（魔鬼＋刺棘）")
	s.set("A138", "長刺海膽（魔鬼＋刺棘） SE")
	for i, depth := range depths {
		for _, d := range data {
			if d.detail.Event.DepthM == depth && d.detail.Event.SurveyDate == latest {
				_, m, _, se := d.summary("long_spined")
				s.set(column(i+2)+"126", m)
				s.set(column(i+2)+"138", se)
			}
		}
	}
	p := "xl/charts/chart3.xml"
	raw := string(b.parts[p])
	raw = strings.ReplaceAll(raw, "$125", "$126")
	raw = strings.ReplaceAll(raw, "$137", "$138")
	b.parts[p] = []byte(raw)
	// A common timeline aligns both depth blocks without merging separate cylinders.
	var slots []string
	slotDates := map[string]string{}
	slotCounts := map[string]int{}
	byDepth := map[float64]map[string]*eventData{}
	for _, d := range data {
		depth := d.detail.Event.DepthM
		if byDepth[depth] == nil {
			byDepth[depth] = map[string]*eventData{}
		}
		base := d.detail.Event.SurveyDate + " " + d.detail.Event.EventTime
		key := base
		ordinal := 1
		for byDepth[depth][key] != nil {
			ordinal++
			key = fmt.Sprintf("%s #%d", base, ordinal)
		}
		byDepth[depth][key] = d
		if _, ok := slotDates[key]; !ok {
			slots = append(slots, key)
			slotDates[key] = d.detail.Event.SurveyDate
			slotCounts[d.detail.Event.SurveyDate]++
		}
	}
	sort.Strings(slots)
	fill := func(name string, headers, meanRows, seRows []int, keys []string, startCol int) {
		ws := b.byName[name]
		for i, depth := range depths {
			for colIndex, key := range slots {
				col := column(startCol + colIndex)
				header := excelDate(slotDates[key])
				if slotCounts[slotDates[key]] > 1 {
					header = key
				}
				ref := col + strconv.Itoa(headers[i])
				ws.set(ref, header)
				ws.copyStyle(ref, column(startCol)+strconv.Itoa(headers[i]))
				d := byDepth[depth][key]
				for j, metric := range keys {
					var m, se any
					if d != nil {
						_, m, _, se = d.summary(metric)
					}
					ref := col + strconv.Itoa(meanRows[i]+j)
					ws.set(ref, m)
					ws.copyStyle(ref, column(startCol)+strconv.Itoa(meanRows[i]+j))
					ref = col + strconv.Itoa(seRows[i]+j)
					ws.set(ref, se)
					ws.copyStyle(ref, column(startCol)+strconv.Itoa(seRows[i]+j))
				}
			}
			label := fmt.Sprintf("%gm", depth)
			for _, row := range []int{headers[i], seRows[i] - 1} {
				ws.set("A"+strconv.Itoa(row), label)
			}
		}
	}
	fill("Summary (line)", []int{1, 74}, []int{2, 75}, []int{18, 88}, lineKeys[:10], 3)
	// Live coral and its comparison series use the common timeline.
	line := b.byName["Summary (line)"]
	for i, depth := range depths {
		for j, key := range slots {
			d := byDepth[depth][key]
			var m any
			if d != nil {
				_, m, _, _ = d.summary("line_live_coral")
			}
			line.set(column(j+3)+strconv.Itoa([]int{12, 85}[i]), m)
			if i == 1 {
				line.set(column(j+3)+"15", m)
			}
			line.set(column(j+3)+"13", 25)
			line.set(column(j+3)+"14", 50)
		}
	}
	if len(depths) > 1 {
		line.set("B15", fmt.Sprintf("%gm", depths[1]))
	} else {
		line.set("B15", nil)
	}
	fill("Summary (fish)", []int{1, 68}, []int{2, 69}, []int{14, 81}, fishSummaryKeys, 3)
	fill("Summary (fish size)", []int{1, 68}, []int{2, 69}, []int{14, 81}, fishSizeKeys, 3)
	fill("Summary (Invertebrates)", []int{1, 67}, []int{2, 68}, []int{15, 81}, invertSummaryKeys, 3)
	fill("Impacts", []int{1, 51}, []int{2, 52}, []int{10, 60}, []string{"anchor_level", "dynamite_level", "damage_other_level", "fishnet_level", "trash_level"}, 2)
	for _, name := range []string{"Summary (fish)", "Summary (fish size)"} {
		b.byName[name].set("B1", fmt.Sprintf("%gm", depths[0]))
	}
	if len(depths) < 2 {
		for name, refs := range map[string][]string{"Summary (line)": {"A74", "A87"}, "Summary (fish)": {"A68", "A80"}, "Summary (fish size)": {"A68", "A80"}, "Summary (Invertebrates)": {"A67", "A80"}, "Impacts": {"A51", "A59"}} {
			for _, ref := range refs {
				b.byName[name].set(ref, nil)
			}
		}
	}
	// The correlation helper is also refreshed; no example values may remain.
	line.set("B145", "硬珊瑚與魔鬼海膽")
	for i, depth := range depths {
		for j, key := range slots {
			d := byDepth[depth][key]
			var hc, diadema any
			if d != nil {
				_, hc, _, _ = d.summary("line_HC")
				_, diadema, _, _ = d.summary("diadema")
			}
			line.set(column(j+3)+strconv.Itoa(146+i), hc)
			line.set(column(j+3)+strconv.Itoa(148+i), diadema)
		}
		line.set("B"+strconv.Itoa(146+i), fmt.Sprintf("%gm HC", depth))
		line.set("B"+strconv.Itoa(148+i), fmt.Sprintf("%gm 魔鬼海膽", depth))
	}
	// Refresh chart range endpoints, including SE ranges. They had different lengths in the example.
	horizontal := regexp.MustCompile(`(\$?)([A-Z]+)(\$?[0-9]+):(\$?)([A-Z]+)(\$?[0-9]+)`)
	for name, raw := range b.parts {
		if !strings.HasPrefix(name, "xl/charts/chart") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		text := string(raw)
		text = chartFormulaRE.ReplaceAllStringFunc(text, func(f string) string {
			if strings.Contains(f, "'"+sheetName+"'!") {
				return f
			}
			return horizontal.ReplaceAllStringFunc(f, func(r string) string {
				m := horizontal.FindStringSubmatch(r)
				if m[3] != m[6] || colNumber(m[2]) > 3 {
					return r
				}
				start := 3
				if strings.Contains(f, "Impacts!") {
					start = 2
				}
				last := start + len(slots) - 1
				if last < start {
					last = start
				}
				return m[1] + m[2] + m[3] + ":" + m[4] + column(last) + m[6]
			})
		})
		b.parts[name] = []byte(text)
	}
	// The two date-per-series comparison charts must have exactly the current timeline's series.
	for _, n := range []int{19, 26} {
		p := fmt.Sprintf("xl/charts/chart%d.xml", n)
		raw, ok := b.parts[p]
		if !ok {
			continue
		}
		serRE := regexp.MustCompile(`(?s)<c:ser>.*?</c:ser>`)
		all := serRE.FindAllString(string(raw), -1)
		if len(all) == 0 {
			continue
		}
		first := all[0]
		templateCol := "C"
		colRef := regexp.MustCompile(`\$` + templateCol + `\$`)
		var series strings.Builder
		for j := range slots {
			ser := colRef.ReplaceAllStringFunc(first, func(string) string { return "$" + column(j+3) + "$" })
			ser = regexp.MustCompile(`<c:(idx|order) val="[0-9]+"/>`).ReplaceAllStringFunc(ser, func(v string) string {
				return regexp.MustCompile(`"[0-9]+"`).ReplaceAllString(v, fmt.Sprintf(`"%d"`, j))
			})
			series.WriteString(ser)
		}
		text := serRE.ReplaceAllString(string(raw), "")
		at := strings.Index(text, "<c:dLbls>")
		if at < 0 {
			at = strings.Index(text, "<c:gapWidth")
		}
		if at < 0 {
			at = strings.Index(text, "<c:axId")
		}
		if at >= 0 {
			text = text[:at] + series.String() + text[at:]
		}
		b.parts[p] = []byte(text)
	}
	title := data[0].detail.Event.SiteName + " " + latest
	b.titles(title, map[int]string{1: "底質覆蓋率", 2: "指標性魚類族群密度", 3: "指標性無脊椎族群密度", 4: "石斑魚尺寸分布", 5: "硨磲貝尺寸分布", 6: "環境衝擊程度", 7: "魚類尺寸分布", 8: "硬珊瑚型態佔比"})
	historyTitle := data[0].detail.Event.SiteName + " " + data[0].detail.Event.SurveyDate + "～" + latest
	b.titles(historyTitle, map[int]string{9: "歷年底質覆蓋率", 10: "歷年底質覆蓋率", 11: "歷年活珊瑚覆蓋率", 12: "硬珊瑚與魔鬼海膽", 17: "歷年魚類密度", 18: "歷年魚類密度", 19: "魚類密度比較", 24: "歷年魚類尺寸", 25: "歷年魚類尺寸", 26: "魚類尺寸比較", 27: "歷年無脊椎密度", 28: "歷年無脊椎密度", 29: "歷年環境衝擊", 30: "歷年環境衝擊"})
	labels := map[int]string{9: "歷年底質覆蓋率", 10: "歷年底質覆蓋率", 17: "歷年魚類密度", 18: "歷年魚類密度", 19: "魚類密度比較", 24: "歷年魚類尺寸", 25: "歷年魚類尺寸", 26: "魚類尺寸比較", 27: "歷年無脊椎密度", 28: "歷年無脊椎密度", 29: "歷年環境衝擊", 30: "歷年環境衝擊"}
	for _, n := range []int{9, 10, 17, 18, 19, 24, 25, 26, 27, 28, 29, 30} {
		index := 0
		for _, deep := range []int{9, 17, 19, 24, 26, 27, 29} {
			if n == deep {
				index = 1
			}
		}
		depthLabel := "未選取深度"
		if index < len(depths) {
			depthLabel = fmt.Sprintf("%gm", depths[index])
		}
		b.title(fmt.Sprintf("xl/charts/chart%d.xml", n), historyTitle+" · "+depthLabel+" · "+labels[n])
	}
	return b.bytes()
}
func containsDepth(values []float64, v float64) bool {
	for _, n := range values {
		if n == v {
			return true
		}
	}
	return false
}

// TemplateWorkbook exposes the reference workbooks individually, preserving their sheet names.
func TemplateWorkbook(details []service.ReefDataDetail, kind string) ([]byte, error) {
	data := sortedEvents(details)
	switch kind {
	case "summary":
		if len(data) == 0 {
			return nil, fmt.Errorf("%w: 沒有報告資料", service.ErrValidation)
		}
		for _, d := range data {
			if d.detail.Event.SiteID != data[0].detail.Event.SiteID {
				return nil, fmt.Errorf("%w: 多樣點請下載 ZIP 套組", service.ErrValidation)
			}
		}
		return summaryTemplate(data)
	case "line", "belt":
		if len(data) != 1 {
			return nil, fmt.Errorf("%w: 逐次範本需指定單一場次", service.ErrValidation)
		}
		if kind == "line" {
			return lineTemplate(data[0])
		}
		return beltTemplate(data[0])
	default:
		return nil, fmt.Errorf("%w: 無效報告類型", service.ErrValidation)
	}
}
func safeReportName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "ReefCheck"
	}
	return name
}

// TemplateArchive returns separate original-format files, avoiding sheet-name collisions.
func TemplateArchive(details []service.ReefDataDetail) ([]byte, error) {
	data := sortedEvents(details)
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	write := func(name string, data []byte) error {
		w, e := z.Create(name)
		if e != nil {
			return e
		}
		_, e = w.Write(data)
		return e
	}
	groups := map[int][]*eventData{}
	var siteIDs []int
	for _, d := range data {
		e := d.detail.Event
		if _, ok := groups[e.SiteID]; !ok {
			siteIDs = append(siteIDs, e.SiteID)
		}
		groups[e.SiteID] = append(groups[e.SiteID], d)
		base := safeReportName(fmt.Sprintf("%s_%s_%s_%gm_%d", e.SiteName, e.SurveyDate, e.EventTime, e.DepthM, e.ID))
		if d.line != nil {
			raw, err := lineTemplate(d)
			if err != nil {
				return nil, err
			}
			if err = write(base+"_LineTransect.xlsx", raw); err != nil {
				return nil, err
			}
		}
		if d.fish != nil || d.invert != nil {
			raw, err := beltTemplate(d)
			if err != nil {
				return nil, err
			}
			if err = write(base+"_BeltTransect.xlsx", raw); err != nil {
				return nil, err
			}
		}
	}
	sort.Ints(siteIDs)
	for _, id := range siteIDs {
		events := groups[id]
		var depths []float64
		for _, d := range events {
			if !containsDepth(depths, d.detail.Event.DepthM) {
				depths = append(depths, d.detail.Event.DepthM)
			}
		}
		sort.Float64s(depths)
		for start := 0; start < len(depths); start += 2 {
			end := start + 2
			if end > len(depths) {
				end = len(depths)
			}
			pair := depths[start:end]
			var subset []*eventData
			for _, d := range events {
				if containsDepth(pair, d.detail.Event.DepthM) {
					subset = append(subset, d)
				}
			}
			raw, err := summaryTemplate(subset)
			if err != nil {
				return nil, err
			}
			name := safeReportName(fmt.Sprintf("%s_%d_Summary_%gm", events[0].detail.Event.SiteName, id, pair[0]))
			if len(pair) > 1 {
				name += fmt.Sprintf("_%gm", pair[1])
			}
			if err = write(name+".xlsx", raw); err != nil {
				return nil, err
			}
		}
	}
	if err := write("報告說明.txt", []byte("各檔使用原始 Line、Belt、Summary 範本。Summary 僅包含本次選取場次，每份最多比較兩個深度。\n空白＝未記錄；0＝觀察到零。SD 使用樣本標準差，SE=SD/√有效段數。\n底質不輸出泥下模式。魚類與無脊椎 EXPORT 另含子樣區 TaiBIF_event_id。\n範本沒有欄位的完整原始記錄保留在 EXPORT 附表，未分體長者不推估體型。\n")); err != nil {
		return nil, err
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
