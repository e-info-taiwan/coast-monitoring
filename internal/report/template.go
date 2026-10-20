package report

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/*.xlsx
var templates embed.FS

var cellRE = regexp.MustCompile(`(?s)<c\b[^>]*?/>|<c\b[^>]*>.*?</c>`)
var refRE = regexp.MustCompile(`\br="([A-Z]+)([0-9]+)"`)
var rowRE = regexp.MustCompile(`(?s)<row\b[^>]*?/>|<row\b[^>]*>.*?</row>`)
var rowNumberRE = regexp.MustCompile(`\br="([0-9]+)"`)
var valueRE = regexp.MustCompile(`(?s)<(?:v|is)\b[^>]*>.*?</(?:v|is)>`)
var formulaRE = regexp.MustCompile(`(?s)<f\b[^>]*(?:/>|>.*?</f>)`)
var typeRE = regexp.MustCompile(`\s+t="[^"]*"`)
var styleRE = regexp.MustCompile(`\bs="([0-9]+)"`)

type templateCell struct {
	value   any
	formula string
	style   string
}
type templateSheet struct {
	name, path, raw string
	cells           map[string]templateCell
	patches         map[string]templateCell
}
type templateBook struct {
	parts  map[string][]byte
	sheets []*templateSheet
	byName map[string]*templateSheet
}

func openTemplate(kind string) (*templateBook, error) {
	raw, err := templates.ReadFile("templates/" + kind + ".xlsx")
	if err != nil {
		return nil, err
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	b := &templateBook{parts: map[string][]byte{}, byName: map[string]*templateSheet{}}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, err
		}
		b.parts[f.Name] = data
	}
	var ss struct {
		Strings []struct {
			XML string `xml:",innerxml"`
		} `xml:"si"`
	}
	if err = xml.Unmarshal(b.parts["xl/sharedStrings.xml"], &ss); err != nil {
		return nil, err
	}
	var stringsTable []string
	for _, s := range ss.Strings {
		stringsTable = append(stringsTable, xmlText(s.XML))
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err = xml.Unmarshal(b.parts["xl/workbook.xml"], &wb); err != nil {
		return nil, err
	}
	if err = xml.Unmarshal(b.parts["xl/_rels/workbook.xml.rels"], &rels); err != nil {
		return nil, err
	}
	targets := map[string]string{}
	for _, r := range rels.Items {
		targets[r.ID] = path.Join("xl", r.Target)
	}
	for _, ws := range wb.Sheets {
		p := targets[ws.ID]
		s := &templateSheet{name: ws.Name, path: p, raw: string(b.parts[p]), cells: map[string]templateCell{}, patches: map[string]templateCell{}}
		for _, raw := range cellRE.FindAllString(s.raw, -1) {
			m := refRE.FindStringSubmatch(raw)
			if m == nil {
				continue
			}
			ref := m[1] + m[2]
			var c struct {
				Type   string `xml:"t,attr"`
				Style  string `xml:"s,attr"`
				V      string `xml:"v"`
				F      string `xml:"f"`
				Inline struct {
					XML string `xml:",innerxml"`
				} `xml:"is"`
			}
			if err = xml.Unmarshal([]byte(raw), &c); err != nil {
				return nil, err
			}
			cell := templateCell{formula: c.F, style: c.Style}
			if c.Type == "s" {
				i, e := strconv.Atoi(c.V)
				if e == nil && i < len(stringsTable) {
					cell.value = stringsTable[i]
				}
			} else if c.Type == "inlineStr" {
				cell.value = xmlText(c.Inline.XML)
			} else if c.V != "" {
				v, e := strconv.ParseFloat(c.V, 64)
				if e == nil {
					cell.value = v
				}
			}
			s.cells[ref] = cell
		}
		b.sheets = append(b.sheets, s)
		b.byName[s.name] = s
	}
	return b, nil
}
func xmlText(raw string) string {
	d := xml.NewDecoder(strings.NewReader("<root>" + raw + "</root>"))
	var out strings.Builder
	inText := false
	for {
		t, e := d.Token()
		if e != nil {
			break
		}
		switch v := t.(type) {
		case xml.StartElement:
			if v.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			if v.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				out.Write(v)
			}
		}
	}
	return out.String()
}
func (s *templateSheet) set(ref string, v any) {
	c := s.cells[ref]
	c.value = v
	c.formula = ""
	s.cells[ref] = c
	s.patches[ref] = c
}
func (s *templateSheet) formula(ref, f string, v any) {
	c := s.cells[ref]
	c.value = v
	c.formula = f
	s.cells[ref] = c
	s.patches[ref] = c
}
func (s *templateSheet) cache(ref string, v any) {
	c := s.cells[ref]
	c.value = v
	s.cells[ref] = c
	s.patches[ref] = c
}
func (s *templateSheet) get(ref string) any { return s.cells[ref].value }
func cellXML(ref string, c templateCell, original string) string {
	opening := `<c r="` + ref + `"`
	if original != "" {
		opening = strings.TrimSuffix(strings.SplitN(original, ">", 2)[0], "/")
		opening = typeRE.ReplaceAllString(opening, "")
		if c.style != "" {
			opening = regexp.MustCompile(`\s+s="[^"]*"`).ReplaceAllString(opening, "")
			opening += ` s="` + c.style + `"`
		}
	} else if c.style != "" {
		opening += ` s="` + c.style + `"`
	}
	formula := ""
	if c.formula != "" {
		formula = "<f>" + escape(c.formula) + "</f>"
	}
	if c.value == nil {
		return opening + ">" + formula + "</c>"
	}
	switch v := c.value.(type) {
	case float64:
		return opening + ">" + formula + "<v>" + strconv.FormatFloat(v, 'g', -1, 64) + "</v></c>"
	case int:
		return opening + ">" + formula + "<v>" + strconv.Itoa(v) + "</v></c>"
	default:
		if c.formula != "" {
			return opening + ` t="str">` + formula + "<v>" + escape(fmt.Sprint(v)) + "</v></c>"
		}
		return opening + ` t="inlineStr"><is><t xml:space="preserve">` + escape(fmt.Sprint(v)) + "</t></is></c>"
	}
}
func (s *templateSheet) render() string {
	rows := map[int]string{}
	attrs := map[int]string{}
	cells := map[int]map[int]string{}
	for _, row := range rowRE.FindAllString(s.raw, -1) {
		m := rowNumberRE.FindStringSubmatch(row)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		rows[n] = row
		attrs[n] = strings.TrimSuffix(strings.SplitN(row, ">", 2)[0], "/") + ">"
		cells[n] = map[int]string{}
		for _, c := range cellRE.FindAllString(row, -1) {
			ref := refRE.FindStringSubmatch(c)
			if ref != nil {
				cells[n][colNumber(ref[1])] = c
			}
		}
	}
	for ref, c := range s.patches {
		m := refRE.FindStringSubmatch(`r="` + ref + `"`)
		n, _ := strconv.Atoi(m[2])
		col := colNumber(m[1])
		if cells[n] == nil {
			cells[n] = map[int]string{}
			attrs[n] = fmt.Sprintf(`<row r="%d">`, n)
		}
		cells[n][col] = cellXML(ref, c, cells[n][col])
	}
	var nums []int
	for n := range cells {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var body strings.Builder
	for _, n := range nums {
		body.WriteString(attrs[n])
		var cols []int
		for c := range cells[n] {
			cols = append(cols, c)
		}
		sort.Ints(cols)
		for _, c := range cols {
			body.WriteString(cells[n][c])
		}
		body.WriteString("</row>")
	}
	start := strings.Index(s.raw, "<sheetData>")
	end := strings.Index(s.raw, "</sheetData>")
	if start < 0 {
		return s.raw
	}
	raw := s.raw[:start] + "<sheetData>" + body.String() + s.raw[end:]
	// New history columns must be in the worksheet's used range.
	maxRow, maxCol := 1, 1
	for _, n := range nums {
		if n > maxRow {
			maxRow = n
		}
		for c := range cells[n] {
			if c > maxCol {
				maxCol = c
			}
		}
	}
	dim := regexp.MustCompile(`<dimension\b[^>]*/>`)
	raw = dim.ReplaceAllString(raw, fmt.Sprintf(`<dimension ref="A1:%s%d"/>`, column(maxCol), maxRow))
	return raw
}
func colNumber(s string) int {
	n := 0
	for _, r := range s {
		n = n*26 + int(r-'A'+1)
	}
	return n
}
func (b *templateBook) rename(old, new string) {
	s := b.byName[old]
	delete(b.byName, old)
	s.name = new
	b.byName[new] = s
	b.parts["xl/workbook.xml"] = []byte(strings.ReplaceAll(string(b.parts["xl/workbook.xml"]), `name="`+old+`"`, `name="`+new+`"`))
	for name, raw := range b.parts {
		if strings.HasPrefix(name, "xl/charts/") {
			b.parts[name] = []byte(strings.ReplaceAll(string(raw), "'"+old+"'!", "'"+new+"'!"))
		}
	}
}
func (b *templateBook) bytes() ([]byte, error) {
	for _, s := range b.sheets {
		b.parts[s.path] = []byte(s.render())
	}
	for name, raw := range b.parts {
		if strings.HasPrefix(name, "xl/charts/chart") && strings.HasSuffix(name, ".xml") {
			b.parts[name] = []byte(b.chartCaches(string(raw)))
		}
	}
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	var names []string
	for name := range b.parts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write(b.parts[name]); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

var chartRefRE = regexp.MustCompile(`(?s)<c:(numRef|strRef)\b[^>]*>.*?</c:(?:numRef|strRef)>`)
var chartFormulaRE = regexp.MustCompile(`(?s)<c:f>(.*?)</c:f>`)
var rangeRE = regexp.MustCompile(`^(?:'((?:[^']|'')+)'|([^!]+))!\$?([A-Z]+)\$?([0-9]+)(?::\$?([A-Z]+)\$?([0-9]+))?$`)

func (b *templateBook) rangeValues(f string) []any {
	var decoded string
	_ = xml.Unmarshal([]byte("<f>"+f+"</f>"), &decoded)
	m := rangeRE.FindStringSubmatch(decoded)
	if m == nil {
		return nil
	}
	name := strings.ReplaceAll(m[1], "''", "'")
	if name == "" {
		name = m[2]
	}
	s := b.byName[name]
	if s == nil {
		return nil
	}
	c1, r1 := colNumber(m[3]), mustInt(m[4])
	c2, r2 := c1, r1
	if m[5] != "" {
		c2, r2 = colNumber(m[5]), mustInt(m[6])
	}
	var out []any
	for r := r1; r <= r2; r++ {
		for c := c1; c <= c2; c++ {
			out = append(out, s.get(column(c)+strconv.Itoa(r)))
		}
	}
	return out
}
func mustInt(s string) int { n, _ := strconv.Atoi(s); return n }
func (b *templateBook) chartCaches(raw string) string {
	return chartRefRE.ReplaceAllStringFunc(raw, func(ref string) string {
		m := chartFormulaRE.FindStringSubmatch(ref)
		if m == nil {
			return ref
		}
		kind := "num"
		if strings.HasPrefix(ref, "<c:strRef") {
			kind = "str"
		}
		vals := b.rangeValues(m[1])
		if kind == "num" {
			for _, v := range vals {
				if _, isText := v.(string); isText {
					kind = "str"
					break
				}
			}
		}
		var cache strings.Builder
		cache.WriteString("<c:" + kind + "Cache>")
		if kind == "num" {
			format := regexp.MustCompile(`(?s)<c:formatCode>.*?</c:formatCode>`).FindString(ref)
			if format == "" {
				format = "<c:formatCode>General</c:formatCode>"
			}
			cache.WriteString(format)
		}
		fmt.Fprintf(&cache, `<c:ptCount val="%d"/>`, len(vals))
		for i, v := range vals {
			if v == nil {
				continue
			}
			if kind == "num" {
				switch v.(type) {
				case int, float64:
				default:
					continue
				}
			}
			if kind == "str" {
				if n, ok := v.(float64); ok && n > 15000 && n < 100000 {
					v = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(n*24) * time.Hour).Format("2006-01-02")
				}
			}
			fmt.Fprintf(&cache, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, escape(fmt.Sprint(v)))
		}
		cache.WriteString("</c:" + kind + "Cache>")
		return "<c:" + kind + "Ref><c:f>" + m[1] + "</c:f>" + cache.String() + "</c:" + kind + "Ref>"
	})
}

var chartTitleRE = regexp.MustCompile(`(?s)<c:title\b[^>]*>.*?</c:title>`)
var drawingTextRE = regexp.MustCompile(`(?s)<a:t>.*?</a:t>`)

func (b *templateBook) title(chartName, title string) {
	raw := string(b.parts[chartName])
	first := true
	raw = chartTitleRE.ReplaceAllStringFunc(raw, func(t string) string {
		if !first {
			return t
		}
		first = false
		run := true
		return drawingTextRE.ReplaceAllStringFunc(t, func(_ string) string {
			if run {
				run = false
				return "<a:t>" + escape(title) + "</a:t>"
			}
			return "<a:t></a:t>"
		})
	})
	b.parts[chartName] = []byte(raw)
}

// Extend the template's existing row format when history gains additional columns.
func (s *templateSheet) copyStyle(ref, source string) {
	c := s.cells[ref]
	c.style = s.cells[source].style
	s.cells[ref] = c
	s.patches[ref] = c
}
