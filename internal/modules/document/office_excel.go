package document

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	apperrors "yunjian/internal/platform/errors"
)

var numberPattern = regexp.MustCompile(`^-?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?$`)

func exportXlsx(canonical []byte) ([]byte, error) {
	var content excelContent
	if err := json.Unmarshal(canonical, &content); err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	files := map[string]string{
		"xl/styles.xml": stylesXML(),
		"_rels/.rels": officeXMLHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
	}
	var rels strings.Builder
	rels.WriteString(officeXMLHeader)
	rels.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	var sheets strings.Builder
	var overrides strings.Builder
	for i, sheet := range content.Sheets {
		name := fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		files[name] = worksheetXML(sheet)
		rid := fmt.Sprintf("rId%d", i+1)
		fmt.Fprintf(&rels, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, rid, i+1)
		fmt.Fprintf(&sheets, `<sheet name="%s" sheetId="%d" r:id="%s"/>`, xmlEscape(sheet.Name), i+1, rid)
		fmt.Fprintf(&overrides, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
	}
	fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, len(content.Sheets)+1)
	rels.WriteString(`</Relationships>`)
	files["xl/_rels/workbook.xml.rels"] = rels.String()
	view := ""
	if content.Active > 0 {
		view = fmt.Sprintf(`<bookViews><workbookView activeTab="%d"/></bookViews>`, content.Active)
	}
	files["xl/workbook.xml"] = officeXMLHeader + fmt.Sprintf(`
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  %s
  <sheets>%s</sheets>
</workbook>`, view, sheets.String())
	files["[Content_Types].xml"] = officeXMLHeader + `
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` + overrides.String() + `</Types>`
	return packZip(files)
}

func stylesXML() string {
	var fonts, xfs strings.Builder
	fonts.WriteString(`<fonts count="8">`)
	xfs.WriteString(`<cellXfs count="8">`)
	for i := 0; i < 8; i++ {
		bold, italic, underline := "", "", ""
		if i&1 != 0 {
			bold = "<b/>"
		}
		if i&2 != 0 {
			italic = "<i/>"
		}
		if i&4 != 0 {
			underline = "<u/>"
		}
		fmt.Fprintf(&fonts, `<font>%s%s%s<sz val="11"/><name val="Arial"/></font>`, bold, italic, underline)
		apply := ""
		if i != 0 {
			apply = ` applyFont="1"`
		}
		fmt.Fprintf(&xfs, `<xf numFmtId="0" fontId="%d" fillId="0" borderId="0" xfId="0"%s/>`, i, apply)
	}
	fonts.WriteString(`</fonts>`)
	xfs.WriteString(`</cellXfs>`)
	return officeXMLHeader + `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		fonts.String() +
		`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
		`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		xfs.String() + `</styleSheet>`
}

func worksheetXML(sheet excelSheet) string {
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	row := 0
	for _, cell := range sheet.Cells {
		if cell.Row != row {
			if row != 0 {
				body.WriteString(`</row>`)
			}
			row = cell.Row
			fmt.Fprintf(&body, `<row r="%d">`, row)
		}
		writeSheetCell(&body, cell)
	}
	if row != 0 {
		body.WriteString(`</row>`)
	}
	body.WriteString(`</sheetData>`)
	var merges []string
	for _, cell := range sheet.Cells {
		if ref := mergeRef(cell); ref != "" {
			merges = append(merges, ref)
		}
	}
	if len(merges) > 0 {
		fmt.Fprintf(&body, `<mergeCells count="%d">`, len(merges))
		for _, ref := range merges {
			fmt.Fprintf(&body, `<mergeCell ref="%s"/>`, ref)
		}
		body.WriteString(`</mergeCells>`)
	}
	body.WriteString(`</worksheet>`)
	return body.String()
}

func writeSheetCell(body *strings.Builder, cell excelCell) {
	ref := colName(cell.Col) + strconv.Itoa(cell.Row)
	style := ""
	if bits := fontBits(cell); bits != 0 {
		style = fmt.Sprintf(` s="%d"`, bits)
	}
	if strings.HasPrefix(cell.Input, "=") && strings.TrimSpace(cell.Input) != "=" {
		fmt.Fprintf(body, `<c r="%s"%s><f>%s</f></c>`, ref, style, xmlEscape(strings.TrimPrefix(cell.Input, "=")))
		return
	}
	if cell.Format != "text" {
		if literal, ok := numericLiteral(cell.Input); ok {
			fmt.Fprintf(body, `<c r="%s"%s><v>%s</v></c>`, ref, style, literal)
			return
		}
	}
	fmt.Fprintf(body, `<c r="%s"%s t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, xmlEscape(cell.Input))
}

func numericLiteral(input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if !numberPattern.MatchString(trimmed) {
		return "", false
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return "", false
	}
	return strconv.FormatFloat(value, 'f', -1, 64), true
}

func fontBits(cell excelCell) int {
	bits := 0
	if cell.Bold {
		bits |= 1
	}
	if cell.Italic {
		bits |= 2
	}
	if cell.Underline {
		bits |= 4
	}
	return bits
}

func mergeRef(cell excelCell) string {
	rows := cell.MergeRows
	cols := cell.MergeCols
	if rows < 2 && cols < 2 {
		return ""
	}
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	return colName(cell.Col) + strconv.Itoa(cell.Row) + ":" + colName(cell.Col+cols-1) + strconv.Itoa(cell.Row+rows-1)
}

func colName(n int) string {
	var buf []byte
	for n > 0 {
		n--
		buf = append([]byte{byte('A' + n%26)}, buf...)
		n /= 26
	}
	return string(buf)
}

func colIndex(letters string) int {
	n := 0
	for _, r := range strings.ToUpper(letters) {
		if r < 'A' || r > 'Z' {
			return 0
		}
		n = n*26 + int(r-'A'+1)
	}
	return n
}

func importXlsx(filename string, body []byte) (string, string, []byte, error) {
	files, err := readZip(body)
	if err != nil {
		return "", "", nil, err
	}
	if zipKind(files) != KindExcel {
		return "", "", nil, apperrors.BadRequest(400, "这个文件不是有效的表格")
	}
	content, err := parseWorkbook(files)
	if err != nil {
		return "", "", nil, readFailure(err)
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return "", "", nil, apperrors.WrapInternal(err)
	}
	canonical, err := Canonical(KindExcel, raw)
	if err != nil {
		return "", "", nil, err
	}
	return KindExcel, titleFromFilename(filename), canonical, nil
}

type sheetLink struct {
	name string
	path string
}

func parseWorkbook(files map[string][]byte) (*excelContent, error) {
	fonts := parseFonts(files["xl/styles.xml"])
	links, active, err := workbookSheets(files)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 || len(links) > 20 {
		return nil, apperrors.BadRequest(400, "工作表数量无效")
	}
	content := &excelContent{Active: active, Sheets: make([]excelSheet, 0, len(links))}
	used := map[string]struct{}{}
	for i, link := range links {
		data := files[link.path]
		if data == nil {
			return nil, apperrors.BadRequest(400, "表格无法读取")
		}
		sheet, err := parseWorksheet(data, fonts, parseShared(files["xl/sharedStrings.xml"]))
		if err != nil {
			return nil, err
		}
		sheet.Name = cleanSheetName(link.name, used, i+1)
		content.Sheets = append(content.Sheets, *sheet)
	}
	if content.Active < 0 || content.Active >= len(content.Sheets) {
		content.Active = 0
	}
	return content, nil
}

func workbookSheets(files map[string][]byte) ([]sheetLink, int, error) {
	rels := parseRels(files["xl/_rels/workbook.xml.rels"])
	var links []sheetLink
	active := 0
	err := walkXML(files["xl/workbook.xml"], func(token xml.Token) error {
		start, ok := token.(xml.StartElement)
		if !ok {
			return nil
		}
		switch start.Name.Local {
		case "workbookView":
			if value := attr(start, "activeTab"); value != "" {
				if n, err := strconv.Atoi(value); err == nil {
					active = n
				}
			}
		case "sheet":
			rid := relID(start)
			target := strings.TrimPrefix(rels[rid], "/")
			if target != "" && !strings.HasPrefix(target, "xl/") {
				target = pathJoin("xl", target)
			}
			links = append(links, sheetLink{name: attr(start, "name"), path: target})
		}
		return nil
	})
	if err != nil {
		return nil, 0, apperrors.BadRequest(400, "表格无法读取")
	}
	if len(links) == 0 {
		if files["xl/worksheets/sheet1.xml"] != nil {
			return []sheetLink{{name: "工作表1", path: "xl/worksheets/sheet1.xml"}}, 0, nil
		}
		return nil, 0, apperrors.BadRequest(400, "表格无法读取")
	}
	return links, active, nil
}

func pathJoin(base, target string) string {
	target = strings.TrimPrefix(target, "/")
	if strings.HasPrefix(target, "xl/") {
		return target
	}
	return strings.TrimPrefix(base+"/"+target, "/")
}

func parseRels(data []byte) map[string]string {
	rels := map[string]string{}
	if len(data) == 0 {
		return rels
	}
	_ = walkXML(data, func(token xml.Token) error {
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" {
			return nil
		}
		id := attr(start, "Id", "id")
		target := attr(start, "Target", "target")
		if id != "" && target != "" {
			rels[id] = target
		}
		return nil
	})
	return rels
}

type fontStyle struct {
	bold      bool
	italic    bool
	underline bool
}

func parseFonts(data []byte) []fontStyle {
	if len(data) == 0 {
		return nil
	}
	var fonts []fontStyle
	var cellFonts []int
	inFont := 0
	inCellXfs := 0
	current := fontStyle{}
	_ = walkXML(data, func(token xml.Token) error {
		switch node := token.(type) {
		case xml.StartElement:
			switch node.Name.Local {
			case "font":
				inFont++
				current = fontStyle{}
			case "cellXfs":
				inCellXfs++
			case "b":
				if inFont > 0 {
					current.bold = flagOn(attr(node, "val"), true)
				}
			case "i":
				if inFont > 0 {
					current.italic = flagOn(attr(node, "val"), true)
				}
			case "u":
				if inFont > 0 {
					value := strings.ToLower(attr(node, "val"))
					current.underline = value != "none" && value != "0" && value != "false"
				}
			case "xf":
				if inCellXfs > 0 {
					id, _ := strconv.Atoi(attr(node, "fontId"))
					cellFonts = append(cellFonts, id)
				}
			}
		case xml.EndElement:
			switch node.Name.Local {
			case "font":
				if inFont > 0 {
					fonts = append(fonts, current)
					inFont--
				}
			case "cellXfs":
				if inCellXfs > 0 {
					inCellXfs--
				}
			}
		}
		return nil
	})
	if len(cellFonts) == 0 {
		return fonts
	}
	mapped := make([]fontStyle, len(cellFonts))
	for i, id := range cellFonts {
		if id >= 0 && id < len(fonts) {
			mapped[i] = fonts[id]
		}
	}
	return mapped
}

func parseShared(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var items []string
	var buf strings.Builder
	inPhonetic, inText := 0, 0
	_ = walkXML(data, func(token xml.Token) error {
		switch node := token.(type) {
		case xml.StartElement:
			switch node.Name.Local {
			case "si":
				buf.Reset()
			case "rPh":
				inPhonetic++
			case "t":
				if inPhonetic == 0 {
					inText++
				}
			}
		case xml.EndElement:
			switch node.Name.Local {
			case "rPh":
				if inPhonetic > 0 {
					inPhonetic--
				}
			case "t":
				if inText > 0 {
					inText--
				}
			case "si":
				items = append(items, buf.String())
			}
		case xml.CharData:
			if inText > 0 && inPhonetic == 0 {
				buf.Write(node)
			}
		}
		return nil
	})
	return items
}

type sheetCell struct {
	excelCell
	style       int
	kind        string
	formulaText string
	formulaType string
	formulaSI   string
	cached      string
	inlineText  string
}

func parseWorksheet(data []byte, fonts []fontStyle, shared []string) (*excelSheet, error) {
	cells := map[[2]int]sheetCell{}
	var order [][2]int
	row := 0
	col := 0
	var cell *sheetCell
	inFormula, inValue, inInline, inText := 0, 0, 0, 0
	var formula, value, inline strings.Builder
	formulaType, formulaSI := "", ""
	err := walkXML(data, func(token xml.Token) error {
		switch node := token.(type) {
		case xml.StartElement:
			switch node.Name.Local {
			case "row":
				if value := attr(node, "r"); value != "" {
					row, _ = strconv.Atoi(value)
				} else {
					row++
				}
				col = 0
			case "c":
				col++
				ref := attr(node, "r")
				cellRow, cellCol := row, col
				if ref != "" {
					if r, c, ok := splitRef(ref); ok {
						cellRow, cellCol = r, c
					}
				}
				style, _ := strconv.Atoi(attr(node, "s"))
				cell = &sheetCell{excelCell: excelCell{Row: cellRow, Col: cellCol}, style: style, kind: attr(node, "t")}
				formula.Reset()
				value.Reset()
				inline.Reset()
				formulaType, formulaSI = "", ""
			case "f":
				inFormula++
				formulaType = attr(node, "t")
				formulaSI = attr(node, "si")
			case "v":
				inValue++
			case "is":
				inInline++
			case "t":
				if inInline > 0 {
					inText++
				}
			case "mergeCell":
				if r, c, rows, cols, ok := parseMerge(attr(node, "ref")); ok {
					key := [2]int{r, c}
					item, found := cells[key]
					if !found {
						item = sheetCell{excelCell: excelCell{Row: r, Col: c}}
						order = append(order, key)
					}
					item.MergeRows = rows
					item.MergeCols = cols
					cells[key] = item
				}
			}
		case xml.EndElement:
			switch node.Name.Local {
			case "f":
				if inFormula > 0 {
					inFormula--
				}
			case "v":
				if inValue > 0 {
					inValue--
				}
			case "t":
				if inText > 0 {
					inText--
				}
			case "is":
				if inInline > 0 {
					inInline--
				}
			case "c":
				if cell != nil {
					cell.formulaText = formula.String()
					cell.formulaType = formulaType
					cell.formulaSI = formulaSI
					cell.cached = value.String()
					cell.inlineText = inline.String()
					applyFont(cell, fonts)
					key := [2]int{cell.Row, cell.Col}
					if _, found := cells[key]; !found {
						order = append(order, key)
					}
					if prev, found := cells[key]; found {
						cell.MergeRows = prev.MergeRows
						cell.MergeCols = prev.MergeCols
					}
					cells[key] = *cell
				}
				cell = nil
			}
		case xml.CharData:
			switch {
			case inFormula > 0:
				formula.Write(node)
			case inValue > 0 && inInline == 0:
				value.Write(node)
			case inText > 0:
				inline.Write(node)
			}
		}
		return nil
	})
	if err != nil {
		return nil, readFailure(err)
	}
	if err := applyInputs(order, cells, shared); err != nil {
		return nil, err
	}
	return sheetFromCells(order, cells)
}

type formulaAnchor struct {
	text string
	row  int
	col  int
}

func applyInputs(order [][2]int, cells map[[2]int]sheetCell, shared []string) error {
	anchors := map[string]formulaAnchor{}
	for _, key := range order {
		cell := cells[key]
		if cell.formulaType == "shared" && strings.TrimSpace(cell.formulaText) != "" && cell.formulaSI != "" {
			anchors[cell.formulaSI] = formulaAnchor{text: strings.TrimSpace(cell.formulaText), row: cell.Row, col: cell.Col}
		}
	}
	for _, key := range order {
		cell := cells[key]
		var input string
		var err error
		switch {
		case cell.formulaType == "shared":
			anchor, ok := anchors[cell.formulaSI]
			if !ok {
				return apperrors.BadRequest(400, "表格包含无法识别的共享公式")
			}
			input = shiftFormula(anchor.text, cell.Row-anchor.row, cell.Col-anchor.col)
		case strings.TrimSpace(cell.formulaText) != "":
			input, err = cellInput("", cell.formulaText, "", "", nil)
		default:
			input, err = cellInput(cell.kind, "", cell.cached, cell.inlineText, shared)
		}
		if err != nil {
			return err
		}
		cell.Input = cleanMultiline(input)
		cells[key] = cell
	}
	return nil
}

func shiftFormula(formula string, deltaRow, deltaCol int) string {
	source := strings.TrimSpace(formula)
	source = strings.TrimPrefix(source, "=")
	var out strings.Builder
	out.WriteByte('=')
	for i := 0; i < len(source); {
		if source[i] == '"' {
			end := i + 1
			for end < len(source) {
				if source[end] == '"' {
					end++
					if end < len(source) && source[end] == '"' {
						end++
						continue
					}
					break
				}
				end++
			}
			out.WriteString(source[i:end])
			i = end
			continue
		}
		if raw, end, ok := sheetToken(source, i); ok {
			out.WriteString(raw)
			i = end
			continue
		}
		if ref, end, ok := parseFormulaRef(source, i); ok {
			next := ref
			if !ref.absRow {
				next.row += deltaRow
			}
			if !ref.absCol {
				next.col += deltaCol
			}
			if next.row < 1 || next.col < 1 || next.row > 120 || next.col > 30 {
				out.WriteString("#REF!")
			} else {
				out.WriteString(formatFormulaRef(next))
			}
			i = end
			continue
		}
		out.WriteByte(source[i])
		i++
	}
	return out.String()
}

type formulaRef struct {
	col    int
	row    int
	absCol bool
	absRow bool
}

func parseFormulaRef(source string, index int) (formulaRef, int, bool) {
	if index > 0 && isFormulaName(source[index-1]) {
		return formulaRef{}, 0, false
	}
	i := index
	absCol := false
	if i < len(source) && source[i] == '$' {
		absCol = true
		i++
	}
	colStart := i
	for i < len(source) && i < colStart+3 && isFormulaLetter(source[i]) {
		i++
	}
	if i == colStart {
		return formulaRef{}, 0, false
	}
	absRow := false
	if i < len(source) && source[i] == '$' {
		absRow = true
		i++
	}
	rowStart := i
	if rowStart >= len(source) || source[rowStart] < '1' || source[rowStart] > '9' {
		return formulaRef{}, 0, false
	}
	i++
	for i < len(source) && source[i] >= '0' && source[i] <= '9' {
		i++
	}
	col := colIndex(source[colStart:rowStart])
	if absRow {
		col = colIndex(source[colStart : rowStart-1])
	}
	row, err := strconv.Atoi(source[rowStart:i])
	if err != nil || col < 1 || col > 30 || row < 1 || row > 120 {
		return formulaRef{}, 0, false
	}
	return formulaRef{col: col, row: row, absCol: absCol, absRow: absRow}, i, true
}

func formatFormulaRef(ref formulaRef) string {
	col := ""
	row := ""
	if ref.absCol {
		col = "$"
	}
	if ref.absRow {
		row = "$"
	}
	return col + colName(ref.col) + row + strconv.Itoa(ref.row)
}

func sheetToken(source string, index int) (string, int, bool) {
	if index < len(source) && source[index] == '\'' {
		i := index + 1
		for i < len(source) {
			if source[i] == '\'' {
				if i+1 < len(source) && source[i+1] == '\'' {
					i += 2
					continue
				}
				i++
				break
			}
			i++
		}
		if i >= len(source) || source[i] != '!' {
			return "", 0, false
		}
		return source[index : i+1], i + 1, true
	}
	if index > 0 && isFormulaName(source[index-1]) {
		return "", 0, false
	}
	i := index
	if i >= len(source) || !isFormulaLetter(source[i]) {
		return "", 0, false
	}
	i++
	for i < len(source) && (isFormulaName(source[i]) || source[i] == '.') {
		i++
	}
	if i >= len(source) || source[i] != '!' {
		return "", 0, false
	}
	return source[index : i+1], i + 1, true
}

func isFormulaLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isFormulaName(value byte) bool {
	return isFormulaLetter(value) || value >= '0' && value <= '9' || value == '_'
}

func cellInput(kind, formula, value, inline string, shared []string) (string, error) {
	formula = strings.TrimSpace(formula)
	if formula != "" {
		if strings.HasPrefix(formula, "=") {
			return formula, nil
		}
		return "=" + formula, nil
	}
	switch kind {
	case "inlineStr":
		return inline, nil
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || index < 0 || index >= len(shared) {
			return "", apperrors.BadRequest(400, "表格无法读取")
		}
		return shared[index], nil
	case "b":
		if strings.TrimSpace(value) == "1" {
			return "TRUE", nil
		}
		return "FALSE", nil
	default:
		if inline != "" {
			return inline, nil
		}
		return value, nil
	}
}

func applyFont(cell *sheetCell, fonts []fontStyle) {
	if cell.style < 0 || cell.style >= len(fonts) {
		return
	}
	font := fonts[cell.style]
	cell.Bold = font.bold
	cell.Italic = font.italic
	cell.Underline = font.underline
}

func sheetFromCells(order [][2]int, cells map[[2]int]sheetCell) (*excelSheet, error) {
	maxRow, maxCol := 0, 0
	kept := make([]excelCell, 0, len(order))
	for _, key := range order {
		cell := cells[key].excelCell
		if cell.Row < 1 || cell.Col < 1 {
			continue
		}
		endRow := cell.Row
		endCol := cell.Col
		if cell.MergeRows > 1 {
			endRow = cell.Row + cell.MergeRows - 1
		}
		if cell.MergeCols > 1 {
			endCol = cell.Col + cell.MergeCols - 1
		}
		if endRow > maxRow {
			maxRow = endRow
		}
		if endCol > maxCol {
			maxCol = endCol
		}
		kept = append(kept, cell)
	}
	if maxRow > 120 || maxCol > 30 {
		return nil, apperrors.BadRequest(400, "表格超过 120 行或 30 列，无法导入")
	}
	rowCount := maxRow
	if rowCount < 50 {
		rowCount = 50
	}
	colCount := maxCol
	if colCount < 26 {
		colCount = 26
	}
	return &excelSheet{RowCount: rowCount, ColCount: colCount, Cells: kept}, nil
}

func splitRef(ref string) (int, int, bool) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, false
	}
	col := colIndex(ref[:i])
	row, err := strconv.Atoi(ref[i:])
	if err != nil || col < 1 || row < 1 {
		return 0, 0, false
	}
	return row, col, true
}

func parseMerge(ref string) (int, int, int, int, bool) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}
	r1, c1, ok1 := splitRef(parts[0])
	r2, c2, ok2 := splitRef(parts[1])
	if !ok1 || !ok2 || r2 < r1 || c2 < c1 {
		return 0, 0, 0, 0, false
	}
	return r1, c1, r2 - r1 + 1, c2 - c1 + 1, true
}

func cleanSheetName(name string, used map[string]struct{}, index int) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:\/?*[]`, r) || r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("工作表%d", index)
	}
	runes := []rune(name)
	if len(runes) > 31 {
		name = string(runes[:31])
	}
	base := name
	for n := 2; ; n++ {
		if _, ok := used[strings.ToLower(name)]; !ok {
			used[strings.ToLower(name)] = struct{}{}
			return name
		}
		suffix := strconv.Itoa(n)
		cut := []rune(base)
		if len(cut)+len(suffix) > 31 {
			cut = cut[:31-len(suffix)]
		}
		name = string(cut) + suffix
	}
}

func importCSV(filename string, body []byte) (string, string, []byte, error) {
	if !utf8.Valid(body) {
		return "", "", nil, apperrors.BadRequest(400, "请使用 UTF-8 文本")
	}
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	comma := ','
	if !bytes.Contains(body, []byte{','}) && bytes.Contains(body, []byte{';'}) {
		comma = ';'
	}
	reader := csv.NewReader(bytes.NewReader(body))
	reader.Comma = comma
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return "", "", nil, apperrors.BadRequest(400, "表格无法读取")
	}
	for len(records) > 0 && len(records[len(records)-1]) == 1 && strings.TrimSpace(records[len(records)-1][0]) == "" {
		records = records[:len(records)-1]
	}
	maxCol := 0
	cells := make([]excelCell, 0)
	for r, row := range records {
		if len(row) > maxCol {
			maxCol = len(row)
		}
		for c, value := range row {
			value = cleanMultiline(value)
			if value == "" {
				continue
			}
			cells = append(cells, excelCell{Row: r + 1, Col: c + 1, Input: value})
		}
	}
	if len(records) > 120 || maxCol > 30 {
		return "", "", nil, apperrors.BadRequest(400, "表格超过 120 行或 30 列，无法导入")
	}
	rowCount := len(records)
	if rowCount < 50 {
		rowCount = 50
	}
	colCount := maxCol
	if colCount < 26 {
		colCount = 26
	}
	raw, err := json.Marshal(excelContent{Sheets: []excelSheet{{
		Name: "工作表1", RowCount: rowCount, ColCount: colCount, Cells: cells,
	}}})
	if err != nil {
		return "", "", nil, apperrors.WrapInternal(err)
	}
	canonical, err := Canonical(KindExcel, raw)
	if err != nil {
		return "", "", nil, err
	}
	return KindExcel, titleFromFilename(filename), canonical, nil
}
