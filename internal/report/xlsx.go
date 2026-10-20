// Package report builds downloadable Reef Check workbooks without external services.
package report

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type sheet struct {
	name string
	rows [][]any
}
type chart struct {
	title              string
	sheet, first, last int
}
type workbook struct {
	sheets []sheet
	charts []chart
}

func (s *sheet) add(values ...any) { s.rows = append(s.rows, values) }
func escape(v string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(v))
	return b.String()
}
func column(n int) string {
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

// All user text is inlineStr, never a formula or an external link.
func (b workbook) bytes() ([]byte, error) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	put := func(name, content string) error {
		w, e := z.Create(name)
		if e != nil {
			return e
		}
		_, e = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+content)
		return e
	}
	const main = `http://schemas.openxmlformats.org/spreadsheetml/2006/main`
	const rel = `http://schemas.openxmlformats.org/officeDocument/2006/relationships`
	const pkg = `http://schemas.openxmlformats.org/package/2006/relationships`
	types := `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`
	book := `<workbook xmlns="` + main + `" xmlns:r="` + rel + `"><sheets>`
	links := `<Relationships xmlns="` + pkg + `"><Relationship Id="styles" Type="` + rel + `/styles" Target="styles.xml"/>`
	for i, s := range b.sheets {
		n := i + 1
		types += fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, n)
		book += fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="s%d"/>`, escape(s.name), n, n)
		links += fmt.Sprintf(`<Relationship Id="s%d" Type="%s/worksheet" Target="worksheets/sheet%d.xml"/>`, n, rel, n)
		var data strings.Builder
		fmt.Fprintf(&data, `<worksheet xmlns="%s" xmlns:r="%s"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`, main, rel)
		if s.name == "說明" {
			data.WriteString(`<cols><col min="1" max="1" width="28" customWidth="1"/><col min="2" max="2" width="110" customWidth="1"/></cols>`)
		} else {
			data.WriteString(`<cols><col min="1" max="40" width="22" customWidth="1"/></cols>`)
		}
		data.WriteString(`<sheetData>`)
		for ri, row := range s.rows {
			if s.name == "說明" && ri > 0 {
				fmt.Fprintf(&data, `<row r="%d" ht="60" customHeight="1">`, ri+1)
			} else {
				fmt.Fprintf(&data, `<row r="%d">`, ri+1)
			}
			for ci, v := range row {
				if v == nil {
					continue
				}
				ref := column(ci+1) + strconv.Itoa(ri+1)
				style := 0
				if s.name == "說明" {
					style = 3
				}
				if ri == 0 {
					style = 1
				}
				switch value := v.(type) {
				case int:
					fmt.Fprintf(&data, `<c r="%s" s="%d"><v>%d</v></c>`, ref, style, value)
				case float64:
					fmt.Fprintf(&data, `<c r="%s" s="2"><v>%s</v></c>`, ref, strconv.FormatFloat(value, 'g', -1, 64))
				default:
					fmt.Fprintf(&data, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, escape(fmt.Sprint(v)))
				}
			}
			data.WriteString(`</row>`)
		}
		data.WriteString(`</sheetData>`)
		if len(s.rows) > 1 {
			fmt.Fprintf(&data, `<autoFilter ref="A1:%s%d"/>`, column(len(s.rows[0])), len(s.rows))
		}
		data.WriteString(`<pageSetup paperSize="9" orientation="landscape" fitToWidth="1" fitToHeight="0"/>`)
		hasCharts := false
		for _, c := range b.charts {
			if c.sheet == n {
				hasCharts = true
			}
		}
		if hasCharts {
			data.WriteString(`<drawing r:id="drawing"/>`)
		}
		data.WriteString(`</worksheet>`)
		if err := put(fmt.Sprintf("xl/worksheets/sheet%d.xml", n), data.String()); err != nil {
			return nil, err
		}
		if hasCharts {
			types += fmt.Sprintf(`<Override PartName="/xl/drawings/drawing%d.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`, n)
			if err := put(fmt.Sprintf("xl/worksheets/_rels/sheet%d.xml.rels", n), fmt.Sprintf(`<Relationships xmlns="%s"><Relationship Id="drawing" Type="%s/drawing" Target="../drawings/drawing%d.xml"/></Relationships>`, pkg, rel, n)); err != nil {
				return nil, err
			}
			drawing := `<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`
			dl := `<Relationships xmlns="` + pkg + `">`
			index := 0
			for j, c := range b.charts {
				if c.sheet != n {
					continue
				}
				index++
				types += fmt.Sprintf(`<Override PartName="/xl/charts/chart%d.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`, j+1)
				drawing += fmt.Sprintf(`<xdr:twoCellAnchor><xdr:from><xdr:col>17</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from><xdr:to><xdr:col>30</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to><xdr:graphicFrame macro=""><xdr:nvGraphicFramePr><xdr:cNvPr id="%d" name="Chart %d"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><xdr:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></xdr:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="%s" r:id="c%d"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:twoCellAnchor>`, (index-1)*22, index*22, index, index, rel, j+1)
				dl += fmt.Sprintf(`<Relationship Id="c%d" Type="%s/chart" Target="../charts/chart%d.xml"/>`, j+1, rel, j+1)
				if err := put(fmt.Sprintf("xl/charts/chart%d.xml", j+1), chartXML(c, b.sheets[c.sheet-1])); err != nil {
					return nil, err
				}
			}
			if err := put(fmt.Sprintf("xl/drawings/drawing%d.xml", n), drawing+`</xdr:wsDr>`); err != nil {
				return nil, err
			}
			if err := put(fmt.Sprintf("xl/drawings/_rels/drawing%d.xml.rels", n), dl+`</Relationships>`); err != nil {
				return nil, err
			}
		}
	}
	for _, v := range [][2]string{
		{"[Content_Types].xml", types + `</Types>`},
		{"_rels/.rels", `<Relationships xmlns="` + pkg + `"><Relationship Id="workbook" Type="` + rel + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", book + `</sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", links + `</Relationships>`},
		{"xl/styles.xml", `<styleSheet xmlns="` + main + `"><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF146C78"/></patternFill></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf/></cellStyleXfs><cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyAlignment="1"><alignment wrapText="1"/></xf><xf numFmtId="2" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment wrapText="1" vertical="top"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`},
	} {
		if err := put(v[0], v[1]); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Summary columns: F label, G:J segments, K total, L mean, M SD, N SE, O n.
func chartXML(c chart, s sheet) string {
	formula := func(col string) string {
		return escape(fmt.Sprintf("'%s'!$%s$%d:$%s$%d", strings.ReplaceAll(s.name, "'", "''"), col, c.first, col, c.last))
	}
	numeric := func(col string, ci int) string {
		var cache strings.Builder
		fmt.Fprintf(&cache, `<c:f>%s</c:f><c:numCache><c:formatCode>0.00</c:formatCode><c:ptCount val="%d"/>`, formula(col), c.last-c.first+1)
		for r := c.first; r <= c.last; r++ {
			v := s.rows[r-1][ci]
			if v != nil {
				fmt.Fprintf(&cache, `<c:pt idx="%d"><c:v>%v</c:v></c:pt>`, r-c.first, v)
			}
		}
		return cache.String() + `</c:numCache>`
	}
	var labels strings.Builder
	fmt.Fprintf(&labels, `<c:f>%s</c:f><c:strCache><c:ptCount val="%d"/>`, formula("F"), c.last-c.first+1)
	for r := c.first; r <= c.last; r++ {
		fmt.Fprintf(&labels, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, r-c.first, escape(fmt.Sprint(s.rows[r-1][5])))
	}
	labels.WriteString(`</c:strCache>`)
	return `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:chart><c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>` + escape(c.title) + `</a:t></a:r></a:p></c:rich></c:tx></c:title><c:plotArea><c:layout/><c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:ser><c:idx val="0"/><c:order val="0"/><c:tx><c:v>Mean ± SE</c:v></c:tx><c:spPr><a:solidFill><a:srgbClr val="146C78"/></a:solidFill></c:spPr><c:errBars><c:errDir val="y"/><c:errBarType val="both"/><c:errValType val="cust"/><c:noEndCap val="0"/><c:plus><c:numRef>` + numeric("N", 13) + `</c:numRef></c:plus><c:minus><c:numRef>` + numeric("N", 13) + `</c:numRef></c:minus></c:errBars><c:cat><c:strRef>` + labels.String() + `</c:strRef></c:cat><c:val><c:numRef>` + numeric("L", 11) + `</c:numRef></c:val></c:ser><c:gapWidth val="80"/><c:axId val="1"/><c:axId val="2"/></c:barChart><c:catAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:axPos val="b"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/></c:catAx><c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:axPos val="l"/><c:majorGridlines/><c:numFmt formatCode="0.00" sourceLinked="0"/><c:crossAx val="1"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx></c:plotArea><c:plotVisOnly val="1"/><c:dispBlanksAs val="gap"/></c:chart></c:chartSpace>`
}
