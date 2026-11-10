package report

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"coast-monitoring/internal/service"
)

func taiLine(d *eventData) ([]byte, error) {
	b, err := openTemplate("taibif-line")
	if err != nil {
		return nil, err
	}
	s := b.sheets[0]
	e := d.detail.Event
	row := 2
	for _, p := range d.line.Points {
		if p.Layer != "surface" {
			continue
		}
		base, morph, under := substrate(p.Code)
		if morph == "" {
			morph = "NA"
		}
		if under == "" {
			under = "None"
		}
		health := "NA"
		if (base == "HC" || base == "SC" || under == "HC" || under == "SC") && p.IsBleached != nil {
			if *p.IsBleached {
				health = "Bleached"
			} else {
				health = "Normal"
			}
		}
		raw := inputToken(p.Code)
		if p.RawInputCode != nil && *p.RawInputCode != "" {
			raw = *p.RawInputCode
		}
		s.copyStyle("C"+strconv.Itoa(row), "C2")
		for i, v := range []any{e.SiteName, e.SiteEnglish, excelDate(e.SurveyDate), e.EventID, p.Segment, p.Position, raw, base, morph, under, health} {
			s.set(column(i+1)+strconv.Itoa(row), v)
		}
		row++
	}
	return b.bytes()
}

func taiSize(raw string) (string, string) {
	val := strings.TrimSpace(raw)
	unit := ""
	for _, suffix := range []struct{ text, unit string }{{"公分", "cm"}, {"公尺", "m"}, {"cm", "cm"}, {"mm", "mm"}, {"m", "m"}} {
		if strings.HasSuffix(val, suffix.text) {
			val = strings.TrimSpace(strings.TrimSuffix(val, suffix.text))
			unit = suffix.unit
			break
		}
	}
	for _, prefix := range []struct{ sign, text string }{{">=", "at least "}, {"<=", "at most "}, {"<", "less than "}, {">", "more than "}} {
		if strings.HasPrefix(val, prefix.sign) {
			return prefix.text + strings.TrimSpace(strings.TrimPrefix(val, prefix.sign)), unit
		}
	}
	if lo, hi, ok := strings.Cut(val, "-"); ok {
		if _, err := strconv.ParseFloat(lo, 64); err == nil {
			if _, err := strconv.ParseFloat(hi, 64); err == nil {
				return "between " + lo + " and " + hi, unit
			}
		}
	}
	return val, unit
}

const beltProtocol = "20 m belt transect with 5 m width, see: Hodgson, G., Hill, J., Kiene, W., Maun, L., Mihaly, J., Liebeler, J., Shuman, C. and Torres, R. Reef Check Instruction Manual: A Guide to Reef Check Coral Reef Monitoring. Reef Check Foundation, Pacific Palisades, California, USA. ISBN 0-9723051-1-4"

func taiRecorders(t *service.ReefDataTransect) string {
	var out []string
	for _, p := range t.Participants {
		if p.Role != "member" {
			continue
		}
		r := []rune(strings.TrimSpace(p.NameZH))
		// The exchange sample masks personal names. Do not fall back to account details.
		if len(r) < 2 || !unicode.Is(unicode.Han, r[0]) {
			continue
		}
		name := string(r[0]) + "〇"
		if len(r) > 2 {
			name += string(r[len(r)-1])
		}
		out = append(out, name)
	}
	return strings.Join(out, " | ")
}

func taiBelt(d *eventData, write func(string, []byte) error, base string) error {
	e := d.detail.Event
	_, _, lat, lon := metadata(d)
	coord := func(v any) string {
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
	dateTime := e.SurveyDate
	if excelTime(e.EventTime) != nil {
		dateTime += " " + strings.ReplaceAll(e.EventTime, "-", ":")
	}
	events := [][]string{{"eventID", "eventDate", "samplingProtocol", "sampleSizeValue", "sampleSizeUnit", "countryCode", "decimalLatitude", "decimalLongitude", "geodeticDatum", "coordinateUncertaintyInMeters", "parentEventID", "verbatimDepth", "verbatimEventDate", "county", "municipality", "locality", "verbatimLocality", "eventRemarks"}}
	occ := [][]string{{"occurrenceID", "eventID", "scientificName", "taxonRank", "vernacularName", "taxonRemarks", "basisOfRecord", "recordedBy", "individualCount", "lifeStage"}}
	mof := [][]string{{"eventID", "measurementID", "measurementType", "measurementValue", "measurementUnit", "measurementRemarks"}}
	emof := [][]string{{"occurrenceID", "measurementID", "measurementType", "measurementValue", "measurementUnit"}}
	for seg := 1; seg <= 4; seg++ {
		eventID := fmt.Sprintf("%s_S%d", e.EventID, seg)
		remark := fmt.Sprintf("segment %d", seg)
		if d.invert != nil && d.invert.OtherDamageNote != "" {
			remark += ", " + d.invert.OtherDamageNote
		}
		events = append(events, []string{eventID, e.SurveyDate, beltProtocol, "100", "square metre", "TW", coord(lat), coord(lon), "WGS84", "", e.EventID, fmt.Sprint(e.DepthM), dateTime, e.County, e.Location, e.SiteEnglish, e.SiteName, remark})
		for _, t := range []*service.ReefDataTransect{d.fish, d.invert} {
			if t == nil {
				continue
			}
			recorders := taiRecorders(t)
			for _, r := range t.Belt {
				if r.Segment != seg || r.Aggregate {
					continue
				}
				id := fmt.Sprintf("%s_OCC%d", eventID, r.ID)
				info := taiTaxa[beltKey(r)]
				if info.name == "" {
					info.name = r.NameEN
					if info.name == "" {
						info.name = r.NameZH
					}
				}
				count := ""
				if observed(r.RecordStatus) {
					count = strconv.Itoa(r.Count)
				}
				occ = append(occ, []string{id, eventID, info.scientific, info.rank, info.name, info.remarks, "HumanObservation", recorders, count, info.stage})
				if count == "" || r.Count == 0 {
					continue
				}
				size := r.ObservedSize
				if size == "" {
					size = r.Size
				}
				if size != "" {
					kind := "total length"
					if strings.HasPrefix(beltKey(r), "clam_") {
						kind = "shell length"
					}
					v, unit := taiSize(size)
					emof = append(emof, []string{id, fmt.Sprintf("%s_MID%d", id, r.ID), kind, v, unit})
				}
			}
			for _, r := range t.Impacts {
				if r.Segment != seg {
					continue
				}
				key := impactKey(r)
				name := taiImpactNames[key]
				if name == "" {
					name = r.NameEN
					if name == "" {
						name = r.NameZH
					}
				}
				unit := r.ValueType
				if r.HasRawCount {
					unit = "count"
				} else if unit == "percent" {
					unit = "percentage"
				}
				value := ""
				if observed(r.RecordStatus) {
					value = strconv.FormatFloat(r.RawValue, 'f', -1, 64)
				}
				note := ""
				if key == "damage_other" {
					note = t.OtherDamageNote
				}
				mof = append(mof, []string{eventID, fmt.Sprintf("%s_Impact_MID%d", eventID, r.ID), name, value, unit, note})
			}
		}
	}
	for _, file := range []struct {
		name string
		rows [][]string
	}{{"Event-Core", events}, {"Occurance", occ}, {"MoF", mof}, {"eMoF", emof}} {
		var data bytes.Buffer
		c := csv.NewWriter(&data)
		c.UseCRLF = true
		if err := c.WriteAll(file.rows); err != nil {
			return err
		}
		if err := write(base+"_"+file.name+".csv", data.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

var taiImpactNames = map[string]string{"anchor": "boat anchor", "dynamite": "dynamite", "damage_other": "other coral damage", "fishnet": "fishnet trash", "trash": "general trash", "bleach_population": "bleaching%population", "bleach_colony": "bleaching%colony", "black_band": "disease%coral black band", "white_band": "disease%coral white band"}
