package report

import (
	"coast-monitoring/internal/service"
	"fmt"
	"math"
	"sort"
	"strings"
)

func value[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
}
func observed(status string) bool { return status != "not_recorded" }
func num(v float64) *float64      { return &v }

// ReefWorkbook exports one or more saved events. Missing observations stay blank.
// Derived IDs are export identifiers; database event IDs are never rewritten.
func ReefWorkbook(details []service.ReefDataDetail, taxa []service.ReefDataTaxon) ([]byte, error) {
	b := workbook{sheets: []sheet{
		{name: "說明"}, {name: "Site Form"}, {name: "底質逐點"}, {name: "魚類與無脊椎"}, {name: "環境影響"}, {name: "白化"}, {name: "Summary"},
	}}
	notes, site, line, belt, impact, bleach, summary := &b.sheets[0], &b.sheets[1], &b.sheets[2], &b.sheets[3], &b.sheets[4], &b.sheets[5], &b.sheets[6]
	notes.add("項目", "說明")
	for _, r := range [][2]string{
		{"依據", "海洋網站-數據格式輸出說明；20260411 合界 Line / Belt 與 Summary 範本。欄位重新整理為可篩選資料表，非原始範本逐格複製。"},
		{"event_id", "保留資料庫氣瓶 event_id；魚類、無脊椎另以 TaiBIF_event_id = event_id + _S1～_S4 識別子樣區。此檔非完整 TaiBIF / Darwin Core 交換封裝。"},
		{"底質", "僅輸出 surface，排除 down。SI(HC-a) 拆為 SI / a / HC。NA 留原始代碼，但不列入覆蓋率分母。"},
		{"Health_Status", "逐點 is_bleached=true → Bleached，false → Normal，未記錄留白；系統無逐點疾病欄位，不推定 Disease。舊手填白化另外列出。"},
		{"統計", "各段有效值計算平均；SD 使用樣本標準差 n−1；SE=SD/√n。n<2 時 SD、SE 留白。底質 Mean/SD/SE 單位為百分比；Total 為點數，Coverage 為全部有效點的加權覆蓋率。"},
		{"缺值", "空白為未記錄，零為觀察到零；缺少方法不補零。n 為有效子樣區數。草稿及不完整資料仍可下載，須檢查警示。"},
		{"總數", "魚類及硨磲貝聚合總數由當前有效物種目錄指定的成員推導；任一成員缺值則該段總數留白。原始手填總數不計入。"},
		{"長刺海膽", "由魔鬼海膽 Diadema + 刺棘海膽 Echinothrix 分段相加，再計算平均、SD、SE；任一缺值則留白。"},
		{"圖表", "Summary 右側為各場次分組長條圖，顯示 Mean ± SE；底質為覆蓋率 %，生物為每段個體數，環境影響依標示單位。"},
		{"Site Form", "列出系統已存的樣點、氣瓶與各調查方法資料；未儲存的範本欄位不推定，亦不含帳號 email 或活動代碼。"},
	} {
		notes.add(r[0], r[1])
	}
	site.add("event_id", "樣點", "Site Name", "調查日期", "時間", "深度(m)", "縣市", "地點", "緯度", "經度", "方法", "水溫(°C)", "能見度下限(m)", "能見度上限(m)", "記錄者", "隊長", "科學指導員", "天氣", "氣溫(°C)", "魚類體長模式", "硬珊瑚型態模式", "備註", "RKC備註", "其他損害說明", "發布狀態")
	line.add("event_id", "segment", "position_m", "substrate_code", "總部代碼", "Base_Substrate", "Morphology", "Under_Substrate", "Health_Status")
	belt.add("event_id", "TaiBIF_event_id", "segment", "method", "taxon_id", "taxon_group", "name_zh", "name_en", "size_class", "count", "record_status", "observed_size", "notes", "is_derived")
	impact.add("event_id", "segment", "name_zh", "name_en", "value_type", "has_raw_count", "raw_value", "derived_level", "record_status")
	bleach.add("event_id", "segment", "類別", "來源", "白化點數", "白化比例(%)", "record_status")
	summary.add("event_id", "樣點", "日期", "深度(m)", "類別／單位", "指標", "0–20m", "25–45m", "50–70m", "75–95m", "Total（底質為點數）", "Mean", "SD", "SE", "n", "Coverage (%)")
	for _, d := range details {
		e := d.Event
		if len(d.Transects) == 0 {
			notes.add("警示 "+e.EventID, "沒有任何調查方法")
		}
		for _, t := range d.Transects {
			latitude, longitude := value(e.Latitude), value(e.Longitude)
			var weather string
			var air any
			if e.EntryMetadata != nil {
				weather = e.EntryMetadata.Weather
				air = value(e.EntryMetadata.AirTemperature)
				if e.EntryMetadata.Latitude != nil {
					latitude = value(e.EntryMetadata.Latitude)
					longitude = value(e.EntryMetadata.Longitude)
				}
			}
			names := func(role string) string {
				var out []string
				for _, p := range t.Participants {
					if p.Role == role {
						n := p.NameZH
						if n == "" {
							n = p.UserName
						}
						out = append(out, n)
					}
				}
				return strings.Join(out, "、")
			}
			site.add(e.EventID, e.SiteName, e.SiteEnglish, e.SurveyDate, e.EventTime, e.DepthM, e.County, e.Location, latitude, longitude, t.Method, value(t.WaterTemp), value(t.VisibilityMin), value(t.VisibilityMax), names("member"), names("team_leader"), names("team_scientist"), weather, air, value(t.FishSizeMode), t.CoralMorphology, value(t.Comments), value(t.RKCNote), t.OtherDamageNote, e.PublicationStatus)
			addSummary := func(category, label string, vals [4]*float64) {
				row := []any{e.EventID, e.SiteName, e.SurveyDate, e.DepthM, category, label}
				var present []float64
				for _, v := range vals {
					row = append(row, value(v))
					if v != nil {
						present = append(present, *v)
					}
				}
				total, mean, sd, se := stats(present)
				row = append(row, total, mean, sd, se, len(present), nil)
				summary.add(row...)
			}
			addChart := func(title string, first int) {
				last := len(summary.rows)
				if last >= first {
					b.charts = append(b.charts, chart{title: fmt.Sprintf("%s %s %gm · %s", e.SiteName, e.SurveyDate, e.DepthM, title), sheet: 7, first: first, last: last})
				}
			}
			switch t.Method {
			case "line":
				var valid [4]int
				counts := map[string][4]float64{}
				for _, p := range t.Points {
					if p.Layer != "surface" || p.Segment < 1 || p.Segment > 4 {
						continue
					}
					base, morph, under := substrate(p.Code)
					var health any
					if (base == "HC" || base == "SC" || under == "HC" || under == "SC") && p.IsBleached != nil {
						health = "Normal"
						if *p.IsBleached {
							health = "Bleached"
						}
					}
					hq := base
					if base == "NA" {
						hq = ""
					}
					line.add(e.EventID, p.Segment, p.Position, p.Code, hq, base, morph, under, health)
					if p.Code == "NA" || p.Code == "" {
						continue
					}
					seg := p.Segment - 1
					valid[seg]++
					v := counts[base]
					v[seg]++
					counts[base] = v
					if base == "HC" || base == "SC" {
						v := counts["live_coral"]
						v[seg]++
						counts["live_coral"] = v
					}
					if base == "HC" && morph != "" {
						v := counts["HC-"+morph]
						v[seg]++
						counts["HC-"+morph] = v
					}
				}
				for seg, n := range valid {
					if n != 40 {
						notes.add("警示 "+e.EventID, fmt.Sprintf("底質 S%d 有效點數 %d/40；統計使用有效點分母。", seg+1, n))
					}
				}
				cover := func(code string) [4]*float64 {
					var vals [4]*float64
					for seg, n := range valid {
						if n > 0 {
							vals[seg] = num(counts[code][seg] / float64(n) * 100)
						}
					}
					return vals
				}
				first := len(summary.rows) + 1
				for _, code := range []string{"HC", "SC", "RKC", "NIA", "SP", "RC", "RB", "SD", "SI", "OT", "live_coral"} {
					addSummary("底質覆蓋率 (%)", code, cover(code))
					setCoverage(summary, counts[code], valid)
				}
				addChart("底質覆蓋率 (%)", first)
				if t.ClassifyHardCoral || t.CoralMorphology == "classified" || counts["HC-a"] != [4]float64{} || counts["HC-b"] != [4]float64{} || counts["HC-c"] != [4]float64{} {
					first = len(summary.rows) + 1
					for _, code := range []string{"HC-a", "HC-b", "HC-c"} {
						addSummary("硬珊瑚型態 (%)", code, cover(code))
						setCoverage(summary, counts[code], valid)
					}
					addChart("硬珊瑚型態 (%)", first)
				}
				for _, r := range t.Bleaching {
					bleach.add(e.EventID, r.Segment, "HC", "手填", recordedCount(r.HC, r.HCStatus), value(r.HCPercent), r.HCStatus)
					bleach.add(e.EventID, r.Segment, "SC", "手填", recordedCount(r.SC, r.SCStatus), value(r.SCPercent), r.SCStatus)
				}
				for _, r := range t.DerivedBleaching {
					bleach.add(e.EventID, r.Segment, "HC", "逐點推導", value(r.HC), nil, derivedStatus(r.HC))
					bleach.add(e.EventID, r.Segment, "SC", "逐點推導", value(r.SC), nil, derivedStatus(r.SC))
				}
			case "belt_fish", "belt_invert":
				rows := derivedBelt(t.Belt, taxa)
				type indicator struct {
					label, group string
					vals         [4]*float64
				}
				var indicators []indicator
				indices := map[int]int{}
				for _, r := range rows {
					if r.Segment < 1 || r.Segment > 4 {
						continue
					}
					belt.add(e.EventID, fmt.Sprintf("%s_S%d", e.EventID, r.Segment), r.Segment, t.Method, r.TaxonID, r.Group, r.NameZH, r.NameEN, r.Size, recordedCount(r.Count, r.RecordStatus), r.RecordStatus, r.ObservedSize, r.Notes, r.Aggregate)
					idx, ok := indices[r.TaxonID]
					if !ok {
						idx = len(indicators)
						indices[r.TaxonID] = idx
						indicators = append(indicators, indicator{label: strings.TrimSpace(r.NameZH + " " + r.Size), group: r.Group})
					}
					if observed(r.RecordStatus) {
						indicators[idx].vals[r.Segment-1] = num(float64(r.Count))
					}
				}
				first := len(summary.rows) + 1
				for _, r := range indicators {
					addSummary(r.group+" (個體/段)", r.label, r.vals)
				}
				if t.Method == "belt_invert" {
					var a, c [4]*float64
					for _, r := range rows {
						if r.Segment < 1 || r.Segment > 4 || !observed(r.RecordStatus) || r.Aggregate {
							continue
						}
						if strings.Contains(strings.ToLower(r.NameEN), "diadema") || strings.Contains(r.NameZH, "魔鬼海膽") {
							a[r.Segment-1] = num(float64(r.Count))
						}
						if strings.Contains(strings.ToLower(r.NameEN), "echinothrix") || strings.Contains(r.NameZH, "刺棘海膽") || strings.Contains(r.NameZH, "刺冠海膽") {
							c[r.Segment-1] = num(float64(r.Count))
						}
					}
					var vals [4]*float64
					for i := range vals {
						if a[i] != nil && c[i] != nil {
							vals[i] = num(*a[i] + *c[i])
						}
					}
					addSummary("invert (個體/段)", "長刺海膽（魔鬼＋刺棘）", vals)
				}
				addChart(t.Method+" (個體/段)", first)
			}
			// Keep impact units separate: counts, ordinal levels and percentages cannot share an axis.
			type impactSeries struct {
				label, unit string
				vals        [4]*float64
			}
			var series []impactSeries
			indices := map[int]int{}
			for _, r := range t.Impacts {
				if r.Segment < 1 || r.Segment > 4 {
					continue
				}
				var raw, level any
				if observed(r.RecordStatus) {
					raw = r.RawValue
					level = value(r.DerivedLevel)
				}
				impact.add(e.EventID, r.Segment, r.NameZH, r.NameEN, r.ValueType, r.HasRawCount, raw, level, r.RecordStatus)
				idx, ok := indices[r.TypeID]
				if !ok {
					idx = len(series)
					indices[r.TypeID] = idx
					unit := r.ValueType
					if r.HasRawCount || r.ValueType == "count" {
						unit = "個數"
					}
					series = append(series, impactSeries{label: r.NameZH, unit: unit})
				}
				if observed(r.RecordStatus) {
					series[idx].vals[r.Segment-1] = num(r.RawValue)
				}
			}
			for _, unit := range []string{"個數", "level", "percent"} {
				first := len(summary.rows) + 1
				for _, r := range series {
					if r.unit == unit {
						addSummary("環境影響 ("+unit+")", r.label, r.vals)
					}
				}
				addChart("環境影響 ("+unit+")", first)
			}
		}
	}
	return b.bytes()
}
func recordedCount(n int, status string) any {
	if !observed(status) {
		return nil
	}
	return n
}
func derivedStatus(n *int) string {
	if n == nil {
		return "not_recorded"
	}
	return "recorded"
}
func stats(v []float64) (total, mean, sd, se any) {
	if len(v) == 0 {
		return
	}
	sum := 0.0
	for _, x := range v {
		sum += x
	}
	m := sum / float64(len(v))
	total, mean = sum, m
	if len(v) > 1 {
		ss := 0.0
		for _, x := range v {
			ss += (x - m) * (x - m)
		}
		s := math.Sqrt(ss / float64(len(v)-1))
		sd, se = s, s/math.Sqrt(float64(len(v)))
	}
	return
}
func substrate(code string) (base, morph, under string) {
	base = code
	if strings.HasPrefix(code, "SI(") && strings.HasSuffix(code, ")") {
		base = "SI"
		under = strings.TrimSuffix(strings.TrimPrefix(code, "SI("), ")")
		if strings.HasPrefix(under, "HC-") {
			morph = strings.TrimPrefix(under, "HC-")
			under = "HC"
		}
	} else if strings.HasPrefix(code, "HC-") {
		base = "HC"
		morph = strings.TrimPrefix(code, "HC-")
	}
	return
}

func derivedBelt(raw []service.ReefDataBelt, taxa []service.ReefDataTaxon) []service.ReefDataBelt {
	out := []service.ReefDataBelt{}
	present := map[int]map[int]service.ReefDataBelt{}
	for _, r := range raw {
		if r.Aggregate {
			continue
		}
		out = append(out, r)
		if present[r.TaxonID] == nil {
			present[r.TaxonID] = map[int]service.ReefDataBelt{}
		}
		present[r.TaxonID][r.Segment] = r
	}
	for _, agg := range taxa {
		if !agg.IsAggregate || !agg.IsActive || agg.AggregateOf == "" {
			continue
		}
		var members []int
		hasAny := false
		for _, m := range taxa {
			if !m.IsAggregate && m.IsActive && m.TaxonGroup == agg.TaxonGroup && m.AggregateOf == agg.AggregateOf {
				members = append(members, m.ID)
				hasAny = hasAny || len(present[m.ID]) > 0
			}
		}
		if !hasAny {
			continue
		}
		for seg := 1; seg <= 4; seg++ {
			r := service.ReefDataBelt{TaxonID: agg.ID, Group: agg.TaxonGroup, NameZH: agg.NameZH, NameEN: agg.NameEN, Size: agg.SizeClass, Aggregate: true, Segment: seg, RecordStatus: "recorded"}
			for _, id := range members {
				m, ok := present[id][seg]
				if !ok || !observed(m.RecordStatus) {
					r.RecordStatus = "not_recorded"
				}
				r.Count += m.Count
			}
			out = append(out, r)
		}
	}
	// Stable ordering preserves catalogue ordering where available and supports historical taxa.
	order := map[int]int{}
	for _, t := range taxa {
		order[t.ID] = t.SortOrder
	}
	sort.SliceStable(out, func(i, j int) bool {
		if order[out[i].TaxonID] != order[out[j].TaxonID] {
			return order[out[i].TaxonID] < order[out[j].TaxonID]
		}
		if out[i].TaxonID != out[j].TaxonID {
			return out[i].TaxonID < out[j].TaxonID
		}
		return out[i].Segment < out[j].Segment
	})
	return out
}

func setCoverage(s *sheet, counts [4]float64, valid [4]int) {
	total, denominator := 0.0, 0
	for i, n := range valid {
		total += counts[i]
		denominator += n
	}
	row := s.rows[len(s.rows)-1]
	if denominator > 0 {
		row[10] = total
		row[15] = total / float64(denominator) * 100
	}
}
