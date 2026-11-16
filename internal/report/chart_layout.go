package report

import (
	"encoding/xml"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Reuse the templates' series, formulas and error bars, but give the exported
// charts explicit dimensions. Legacy XLS anchors and manual legend positions
// are interpreted differently by Excel and Numbers.
type reportChart struct {
	id, x, y, width, height int
	axis, numberFormat      string
}

func reportChartRows(rows ...[]int) []reportChart {
	var charts []reportChart
	y := 0
	for _, row := range rows {
		width := 920
		if len(row) == 2 {
			width = 450
		}
		for i, id := range row {
			charts = append(charts, reportChart{id: id, x: i * 470, y: y, width: width, height: 400})
		}
		y += 420
	}
	return charts
}

func (b *templateBook) presentCharts(kind string) error {
	var layouts map[string][]reportChart
	switch kind {
	case "field-line":
		layouts = map[string][]reportChart{"GRAPHS": reportChartRows([]int{1}, []int{2, 3}, []int{4, 5})}
	case "field-belt":
		layouts = map[string][]reportChart{"GRAPHS": reportChartRows([]int{1}, []int{2}, []int{3}, []int{4}, []int{5}, []int{6, 7}, []int{8})}
	case "line":
		layouts = map[string][]reportChart{
			"GRAPHS":    reportChartRows([]int{1}, []int{2, 3}, []int{4, 5}, []int{6}),
			"GRAPHS_SI": reportChartRows([]int{7}, []int{8, 10}),
		}
	case "belt":
		layouts = map[string][]reportChart{"GRAPHS": reportChartRows([]int{1}, []int{6}, []int{7}, []int{8}, []int{2}, []int{3, 4}, []int{5})}
	}
	for sheet, charts := range layouts {
		for i := range charts {
			c := &charts[i]
			c.axis, c.numberFormat = "Mean Abundance +- SE", "0.00"
			if strings.HasSuffix(kind, "line") {
				c.axis, c.numberFormat = "Mean Percent Cover +- SE", `0.0"%"`
				if kind == "field-line" {
					c.numberFormat = "0%"
				}
				if kind == "line" && c.id == 6 {
					c.axis, c.numberFormat = "Percent of coral points", "0%"
				}
			}
			if (kind == "field-belt" && c.id == 5) || (kind == "belt" && c.id == 2) {
				c.axis, c.numberFormat = "Impact rank +- SE", "0.0"
			}
			if (kind == "field-belt" && c.id == 8) || (kind == "belt" && c.id == 5) {
				c.axis, c.numberFormat = "Percent of coral", `0.0"%"`
				if kind == "field-belt" {
					c.numberFormat = "0%"
				}
			}
		}
		if err := b.layoutCharts(sheet, charts); err != nil {
			return err
		}
	}
	return nil
}

func xmlElement(raw, name string) string {
	return regexp.MustCompile(`(?s)<` + name + `\b[^>]*?/>|<` + name + `\b[^>]*>.*?</` + name + `>`).FindString(raw)
}
func withoutElement(raw, name string) string {
	return regexp.MustCompile(`(?s)<`+name+`\b[^>]*?/>|<`+name+`\b[^>]*>.*?</`+name+`>`).ReplaceAllString(raw, "")
}
func chartText(text string, size int, vertical bool) string {
	rotation := ""
	if vertical {
		rotation = ` rot="-5400000"`
	}
	return fmt.Sprintf(`<c:tx><c:rich><a:bodyPr%s/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="%d"/></a:pPr><a:r><a:rPr lang="en-US"/><a:t>%s</a:t></a:r></a:p></c:rich></c:tx>`, rotation, size, escape(text))
}
func chartFont(size int) string {
	return fmt.Sprintf(`<c:txPr><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="%d"/></a:pPr><a:endParaRPr lang="en-US"/></a:p></c:txPr>`, size)
}

func formatReportChart(raw string, config reportChart) (string, error) {
	series := regexp.MustCompile(`(?s)<c:ser\b[^>]*>.*?</c:ser>`).FindAllString(raw, -1)
	if len(series) == 0 {
		return "", fmt.Errorf("report chart %d has no series", config.id)
	}
	for i, ser := range series {
		for _, tag := range []string{"c:dLbls", "c:explosion", "a:effectLst", "a:effectDag", "a:scene3d", "a:sp3d"} {
			ser = withoutElement(ser, tag)
		}
		// An explicit empty effect list also overrides a theme's shadow.
		ser = strings.ReplaceAll(ser, "</c:spPr>", "<a:effectLst/></c:spPr>")
		// Newly built field charts have no series shape properties. Without an
		// explicit override, Numbers applies the theme's default drop shadow.
		prefix := regexp.MustCompile(`(?s)^<c:ser>\s*<c:idx\b[^>]*/>\s*<c:order\b[^>]*/>(?:\s*<c:tx>.*?</c:tx>)?`).FindString(ser)
		if !strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(ser, prefix)), "<c:spPr") {
			ser = prefix + `<c:spPr><a:effectLst/></c:spPr>` + strings.TrimPrefix(ser, prefix)
		}
		series[i] = ser
	}
	title := xmlText(xmlElement(raw, "c:title"))
	var plot, legend string
	if strings.Contains(raw, "<c:pieChart") || strings.Contains(raw, "<c:pie3DChart") {
		plot = `<c:pieChart><c:varyColors val="1"/>` + strings.Join(series, "") +
			`<c:dLbls><c:numFmt formatCode="0.0%" sourceLinked="0"/>` + chartFont(1100) +
			`<c:dLblPos val="bestFit"/><c:showLegendKey val="0"/><c:showVal val="0"/><c:showCatName val="0"/><c:showSerName val="0"/><c:showPercent val="1"/><c:showLeaderLines val="1"/></c:dLbls><c:firstSliceAng val="0"/></c:pieChart>`
		legend = `<c:legend><c:legendPos val="b"/><c:layout/><c:overlay val="0"/>` + chartFont(1100) + `</c:legend>`
	} else {
		grouping := xmlElement(raw, "c:grouping")
		if grouping == "" {
			grouping = `<c:grouping val="clustered"/>`
		}
		plot = `<c:barChart><c:barDir val="col"/>` + grouping + `<c:varyColors val="0"/>` + strings.Join(series, "") + `<c:gapWidth val="150"/>` + xmlElement(raw, "c:overlap") + `<c:axId val="100"/><c:axId val="200"/></c:barChart>`
		categoryFont := strings.Replace(chartFont(1100), `<a:bodyPr/>`, `<a:bodyPr rot="-2700000"/>`, 1)
		plot += `<c:catAx><c:axId val="100"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:majorTickMark val="none"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/>` + categoryFont + `<c:crossAx val="200"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/></c:catAx>`
		plot += `<c:valAx><c:axId val="200"/><c:scaling><c:orientation val="minMax"/><c:min val="0"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:majorGridlines><c:spPr><a:ln w="9525"><a:solidFill><a:srgbClr val="D9D9D9"/></a:solidFill></a:ln></c:spPr></c:majorGridlines><c:title>` + chartText(config.axis, 1100, true) + `<c:overlay val="0"/></c:title><c:numFmt formatCode="` + escape(config.numberFormat) + `" sourceLinked="0"/><c:majorTickMark val="none"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/>` + chartFont(1100) + `<c:crossAx val="100"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>`
		if len(series) > 1 {
			legend = `<c:legend><c:legendPos val="b"/><c:layout/><c:overlay val="0"/>` + chartFont(1100) + `</c:legend>`
		}
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><c:lang val="en-US"/><c:style val="10"/><c:chart><c:title>` + chartText(title, 1200, false) + `<c:overlay val="0"/></c:title><c:autoTitleDeleted val="0"/><c:plotArea><c:layout/>` + plot + `</c:plotArea>` + legend + `<c:plotVisOnly val="1"/><c:dispBlanksAs val="gap"/></c:chart><c:spPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill><a:ln><a:solidFill><a:srgbClr val="D9D9D9"/></a:solidFill></a:ln><a:effectLst/></c:spPr>` + chartFont(1100) + `</c:chartSpace>`, nil
}

func (b *templateBook) related(part string) (map[string]string, error) {
	var rels struct {
		Items []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	relPath := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
	if err := xml.Unmarshal(b.parts[relPath], &rels); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rels.Items {
		out[r.ID] = path.Join(path.Dir(part), r.Target)
	}
	return out, nil
}

func (b *templateBook) layoutCharts(sheetName string, charts []reportChart) error {
	s := b.byName[sheetName]
	if s == nil {
		return fmt.Errorf("missing chart worksheet %s", sheetName)
	}
	var drawing struct {
		ID string `xml:"id,attr"`
	}
	if err := xml.Unmarshal([]byte(xmlElement(s.raw, "drawing")), &drawing); err != nil {
		return err
	}
	rels, err := b.related(s.path)
	if err != nil {
		return err
	}
	drawingPath := rels[drawing.ID]
	drawingRels, err := b.related(drawingPath)
	if err != nil {
		return err
	}
	var anchors strings.Builder
	anchors.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`)
	endRow := 1
	for _, c := range charts {
		chartPath := fmt.Sprintf("xl/charts/chart%d.xml", c.id)
		id := ""
		for relID, target := range drawingRels {
			if target == chartPath {
				id = relID
			}
		}
		if id == "" {
			return fmt.Errorf("missing drawing for %s", chartPath)
		}
		styled, err := formatReportChart(string(b.parts[chartPath]), c)
		if err != nil {
			return err
		}
		b.parts[chartPath] = []byte(styled)
		fmt.Fprintf(&anchors, `<xdr:oneCellAnchor><xdr:from><xdr:col>%d</xdr:col><xdr:colOff>%d</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from><xdr:ext cx="%d" cy="%d"/><xdr:graphicFrame macro=""><xdr:nvGraphicFramePr><xdr:cNvPr id="%d" name="Chart %d"/><xdr:cNvGraphicFramePr/></xdr:nvGraphicFramePr><xdr:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></xdr:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="%s"/></a:graphicData></a:graphic></xdr:graphicFrame><xdr:clientData/></xdr:oneCellAnchor>`, 4+c.x/80, c.x%80*9525, 1+c.y/20, c.width*9525, c.height*9525, c.id+1, c.id, id)
		if last := (c.y+c.height)/20 + 2; last > endRow {
			endRow = last
		}
	}
	anchors.WriteString(`</xdr:wsDr>`)
	b.parts[drawingPath] = []byte(anchors.String())
	// The source tables in A:C remain available. Remove old chart-area cell
	// borders, merged placeholder headings and fills behind the new drawings.
	s.raw = cellRE.ReplaceAllStringFunc(s.raw, func(cell string) string {
		ref := refRE.FindStringSubmatch(cell)
		if ref != nil && colNumber(ref[1]) > 3 {
			return ""
		}
		return cell
	})
	for ref := range s.patches {
		if colNumber(refRE.FindStringSubmatch(`r="` + ref + `"`)[1]) > 3 {
			delete(s.patches, ref)
			delete(s.cells, ref)
		}
	}
	s.raw = withoutElement(s.raw, "mergeCells")
	// Some old workbooks format hundreds of empty rows below the last chart.
	// Trim only those empty rows, retaining any source values or formulas.
	s.raw = rowRE.ReplaceAllStringFunc(s.raw, func(row string) string {
		match := rowNumberRE.FindStringSubmatch(row)
		if match == nil {
			return row
		}
		n, _ := strconv.Atoi(match[1])
		if n <= endRow {
			return row
		}
		for _, cell := range cellRE.FindAllString(row, -1) {
			ref := refRE.FindStringSubmatch(cell)
			if ref == nil {
				continue
			}
			c := s.cells[ref[1]+ref[2]]
			if c.formula != "" || (c.value != nil && c.value != "") {
				return row
			}
		}
		return ""
	})
	s.raw = regexp.MustCompile(`(?s)<cols>.*?</cols>`).ReplaceAllString(s.raw, `<cols><col min="1" max="1" width="40" customWidth="1"/><col min="2" max="3" width="14" customWidth="1"/><col min="4" max="4" width="3" customWidth="1"/><col min="5" max="16" width="10.7109375" customWidth="1"/></cols>`)
	for r := 1; r <= endRow; r++ {
		s.setRowLayout(r, 15, false)
	}
	for index, ws := range b.sheets {
		if ws != s {
			continue
		}
		wb := string(b.parts["xl/workbook.xml"])
		name := fmt.Sprintf(`<definedName name="_xlnm.Print_Area" localSheetId="%d">'%s'!$E$1:$P$%d</definedName>`, index, sheetName, endRow)
		pattern := regexp.MustCompile(fmt.Sprintf(`(?s)<definedName\b[^>]*name="_xlnm.Print_Area"[^>]*localSheetId="%d"[^>]*>.*?</definedName>`, index))
		if pattern.MatchString(wb) {
			wb = pattern.ReplaceAllStringFunc(wb, func(string) string { return name })
		} else if strings.Contains(wb, "</definedNames>") {
			wb = strings.Replace(wb, "</definedNames>", name+"</definedNames>", 1)
		} else {
			wb = strings.Replace(wb, "</sheets>", "</sheets><definedNames>"+name+"</definedNames>", 1)
		}
		b.parts["xl/workbook.xml"] = []byte(wb)
	}
	return nil
}

type reportRowLayout struct {
	height float64
	hidden bool
}

func (s *templateSheet) setRowLayout(row int, height float64, hidden bool) {
	if s.rowLayouts == nil {
		s.rowLayouts = map[int]reportRowLayout{}
	}
	s.rowLayouts[row] = reportRowLayout{height, hidden}
}

// Clone only this cell's format; changing a shared XF would wrap unrelated rows.
func (b *templateBook) wrapCell(s *templateSheet, ref string) error {
	raw := string(b.parts["xl/styles.xml"])
	xfs := xmlElement(raw, "cellXfs")
	entries := regexp.MustCompile(`(?s)<xf\b[^>]*?/>|<xf\b[^>]*>.*?</xf>`).FindAllString(xfs, -1)
	c := s.cells[ref]
	i, err := strconv.Atoi(c.style)
	if err != nil || i >= len(entries) {
		return fmt.Errorf("missing style for %s", ref)
	}
	xf := withoutElement(entries[i], "alignment")
	xf = strings.TrimSuffix(xf, "/>")
	if !strings.HasSuffix(xf, "</xf>") {
		xf += "></xf>"
	}
	xf = regexp.MustCompile(`\s+applyAlignment="[^"]*"`).ReplaceAllString(xf, "")
	xf = strings.Replace(xf, "<xf ", `<xf applyAlignment="1" `, 1)
	xf = strings.Replace(xf, "</xf>", `<alignment vertical="center" wrapText="1"/></xf>`, 1)
	newXfs := regexp.MustCompile(`count="\d+"`).ReplaceAllString(xfs, fmt.Sprintf(`count="%d"`, len(entries)+1))
	newXfs = strings.Replace(newXfs, "</cellXfs>", xf+"</cellXfs>", 1)
	b.parts["xl/styles.xml"] = []byte(strings.Replace(raw, xfs, newXfs, 1))
	c.style = strconv.Itoa(len(entries))
	s.cells[ref], s.patches[ref] = c, c
	return nil
}
