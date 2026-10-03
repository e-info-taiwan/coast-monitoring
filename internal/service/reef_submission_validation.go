package service

import (
	"fmt"
	"strings"
	"time"
)

// Normalize only the explicitly identified new field-entry contract. Historical
// imports must identify their source convention before interpreting numeric codes.
func NormalizeEntrySubstrate(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	aliases := map[string]string{"0": "OT", "1": "HC", "2": "SC", "3": "RKC", "4": "NIA", "5": "SP", "6": "RC", "7": "RB", "8": "SD", "9": "SI", "A": "HC-a", "B": "HC-b", "C": "HC-c", "HC-A": "HC-a", "HC-B": "HC-b", "HC-C": "HC-c", "-": "NA", "SI(SI)": "SI"}
	if c, ok := aliases[code]; ok {
		return c
	}
	for i, c := range []string{"HC", "SC", "RKC", "NIA", "SP", "RC", "RB", "SD"} {
		if code == fmt.Sprint(91+i) {
			return "SI(" + c + ")"
		}
	}
	return code
}
func (s *ReefCheckSurveySubmission) validateObservations() error {
	bad := func(msg string) error { return fmt.Errorf("%w: %s", ErrValidation, msg) }
	if s.Event.SiteID <= 0 {
		return bad("請選擇正式樣點")
	}
	if _, err := time.Parse("15:04", strings.ReplaceAll(s.Event.EventTime, "-", ":")); err != nil {
		return bad("請填開始時間 HH:MM")
	}
	classified := map[string]bool{}
	methods := map[string]bool{}
	keys := map[string]string{}
	for _, t := range s.Transects {
		if methods[t.Method] || t.TransectKey == "" || keys[t.TransectKey] != "" {
			return bad("重複方法或無效手板識別")
		}
		classified[t.TransectKey] = t.ClassifyHardCoral
		methods[t.Method] = true
		keys[t.TransectKey] = t.Method
		if len(t.Recorders) == 0 || strings.TrimSpace(strings.Join(t.Recorders, "")) == "" {
			return bad("每個方法需有記錄者")
		}
		if t.Method == "belt_fish" && t.FishSizeMode != "split" && t.FishSizeMode != "combined" {
			return bad("請指定魚類體長記錄模式")
		}
	}
	validCode := map[string]bool{"HC": true, "HC-a": true, "HC-b": true, "HC-c": true, "SC": true, "RKC": true, "NIA": true, "SP": true, "RC": true, "RB": true, "SD": true, "SI": true, "OT": true, "NA": true}
	for _, c := range []string{"HC", "SC", "RKC", "NIA", "SP", "RC", "RB", "SD"} {
		validCode["SI("+c+")"] = true
	}
	positions := map[string]bool{}
	counts := map[string]int{}
	surface := map[float64]string{}
	unknown := false
	for i := range s.SubstratePoints {
		p := &s.SubstratePoints[i]
		p.SubstrateCode = NormalizeEntrySubstrate(p.SubstrateCode)
		if strings.HasPrefix(p.SubstrateCode, "HC-") && !classified[p.TransectKey] {
			return bad("未分類硬珊瑚型態時請使用 HC")
		}
		if keys[p.TransectKey] != "line" || p.Segment < 1 || p.Segment > 4 || !finite(p.PositionM) || p.PositionM < float64((p.Segment-1)*25) || p.PositionM > float64((p.Segment-1)*25)+19.5 || p.PositionM*2 != float64(int(p.PositionM*2)) || p.SubstrateLayer != "surface" || !validCode[p.SubstrateCode] {
			return bad("底質位置、層或代碼無效")
		}
		key := fmt.Sprintf("%s:%g", p.TransectKey, p.PositionM)
		if positions[key] {
			return bad("底質位置重複")
		}
		positions[key] = true
		surface[p.PositionM] = p.SubstrateCode
		c := p.SubstrateCode
		if strings.HasPrefix(c, "HC-") {
			c = "HC"
		}
		counts[fmt.Sprintf("%d:%s", p.Segment, c)]++
		if c == "NA" {
			unknown = true
		}
	}
	if methods["line"] && len(s.SubstratePoints) != 160 {
		return bad("底質需完整填寫 160 個位置；未記錄請填 NA")
	}
	seen := map[string]bool{}
	for _, b := range s.SubstrateBleaching {
		key := fmt.Sprintf("%d:%s", b.Segment, b.SubstrateCode)
		if keys[b.TransectKey] != "line" || b.Segment < 1 || b.Segment > 4 || (b.SubstrateCode != "HC" && b.SubstrateCode != "SC") || seen[key] {
			return bad("白化附表項目無效或重複")
		}
		seen[key] = true
		if b.BleachedPoints == nil {
			if b.RecordStatus != "not_recorded" {
				return bad("白化空白值需明確標記 NA")
			}
			unknown = true
		} else if *b.BleachedPoints < 0 || *b.BleachedPoints > counts[key] {
			return bad("白化點數超過對應底質點數")
		}
	}
	seen = map[string]bool{}
	for _, o := range s.BeltObservations {
		method := keys[o.TransectKey]
		expected := "belt_invert"
		if o.TaxonGroup == "fish" {
			expected = "belt_fish"
		}
		key := fmt.Sprintf("%s:%s:%s:%s:%d", o.TransectKey, o.TaxonGroup, strings.ToLower(strings.TrimSpace(o.TaxonNameENLookup)), strings.ToLower(strings.ReplaceAll(strings.TrimSpace(o.TaxonSizeClassLookup), " ", "")), o.Segment)
		if method != expected || o.Segment < 1 || o.Segment > 4 || o.TaxonNameENLookup == "" || seen[key] {
			return bad("生物觀測項目無效或重複")
		}
		seen[key] = true
		if o.Count == nil {
			if o.RecordStatus != "not_recorded" {
				return bad("空白生物數量不可送出")
			}
			unknown = true
		} else if *o.Count < 0 || *o.Count > 2147483647 || o.RecordStatus != "recorded" {
			return bad("生物數量或狀態無效")
		}
	}
	seen = map[string]bool{}
	for _, o := range s.ImpactObservations {
		key := fmt.Sprintf("%s:%s:%d", o.ImpactGroup, o.ImpactNameENLookup, o.Segment)
		if keys[o.TransectKey] != "belt_invert" || o.Segment < 1 || o.Segment > 4 || seen[key] {
			return bad("環境衝擊項目無效或重複")
		}
		seen[key] = true
		if o.RawValue == nil {
			if o.RecordStatus != "not_recorded" {
				return bad("環境衝擊空白值需明確標記 NA")
			}
			unknown = true
			continue
		}
		v := *o.RawValue
		if !finite(v) || v < 0 {
			return bad("環境衝擊值無效")
		}
		if o.ImpactGroup == "coral_damage" || o.ImpactGroup == "trash" {
			if !integer(v) || v > 2147483647 || o.ImpactValueType != "count" {
				return bad("珊瑚損害及垃圾需填原始件數")
			}
		} else if (o.ImpactGroup != "bleaching" && o.ImpactGroup != "disease") || o.ImpactValueType != "percent" || v > 100 {
			return bad("白化及疾病百分比需介於 0–100")
		}
	}
	seen = map[string]bool{}
	for _, m := range s.MudAudit {
		key := fmt.Sprint(m.PositionM)
		if seen[key] || m.Segment < 1 || m.Segment > 4 || m.PositionM < float64((m.Segment-1)*25) || m.PositionM > float64((m.Segment-1)*25)+19.5 || m.PositionM*2 != float64(int(m.PositionM*2)) {
			return bad("泥下位置無效或重複")
		}
		seen[key] = true
		if NormalizeEntrySubstrate(m.Surface) != "SI" {
			return bad("泥下只能填於 SI 表面")
		}
		d := NormalizeEntrySubstrate(m.Down)
		canonical := "SI(" + d + ")"
		if d == "SI" {
			canonical = "SI"
		}
		if d == "NA" {
			canonical = "NA"
			unknown = true
		}
		if !validCode[canonical] || surface[m.PositionM] != canonical || NormalizeEntrySubstrate(m.Canonical) != canonical {
			return bad("泥下與正式底質不一致")
		}
	}
	if unknown && strings.TrimSpace(s.MissingReason) == "" {
		return bad("含 NA 時需填未記錄原因")
	}
	s.Status = "draft"
	return nil
}
