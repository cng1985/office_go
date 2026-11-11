package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	apperrors "yunjian/internal/platform/errors"
)

const maxContentBytes = 1 << 20

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type wordContent struct {
	Blocks []wordBlock `json:"blocks"`
}

type wordBlock struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Align   string    `json:"align"`
	Runs    []wordRun `json:"runs"`
	Indent  int       `json:"indent,omitempty"`
	Spacing string    `json:"spacing,omitempty"`
}

type wordRun struct {
	Text      string `json:"text"`
	Bold      bool   `json:"bold"`
	Italic    bool   `json:"italic"`
	Underline bool   `json:"underline"`
	Strike    bool   `json:"strike,omitempty"`
	Color     string `json:"color,omitempty"`
	Highlight string `json:"highlight,omitempty"`
	Size      int    `json:"size,omitempty"`
	Font      string `json:"font,omitempty"`
}

type excelContent struct {
	Active    int          `json:"active,omitempty"`
	Sheets    []excelSheet `json:"sheets,omitempty"`
	SheetName string       `json:"sheet_name,omitempty"`
	RowCount  int          `json:"row_count,omitempty"`
	ColCount  int          `json:"col_count,omitempty"`
	Cells     []excelCell  `json:"cells,omitempty"`
}

type excelSheet struct {
	Name      string      `json:"name"`
	RowCount  int         `json:"row_count"`
	ColCount  int         `json:"col_count"`
	ColWidths []int       `json:"col_widths,omitempty"`
	Cells     []excelCell `json:"cells"`
}

type excelCell struct {
	Row       int    `json:"row"`
	Col       int    `json:"col"`
	Input     string `json:"input"`
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
	Align     string `json:"align,omitempty"`
	Color     string `json:"color,omitempty"`
	Fill      string `json:"fill,omitempty"`
	Format    string `json:"format,omitempty"`
}

type pptContent struct {
	Slides []pptSlide `json:"slides"`
}

type pptSlide struct {
	ID         string       `json:"id"`
	Background string       `json:"background"`
	Elements   []pptElement `json:"elements"`
	Notes      string       `json:"notes,omitempty"`
}

type pptElement struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	W         float64 `json:"w"`
	H         float64 `json:"h"`
	Text      string  `json:"text"`
	FontSize  float64 `json:"font_size"`
	Color     string  `json:"color"`
	Bold      bool    `json:"bold"`
	Align     string  `json:"align"`
	Italic    bool    `json:"italic,omitempty"`
	Underline bool    `json:"underline,omitempty"`
	Fill      string  `json:"fill,omitempty"`
	Shape     string  `json:"shape,omitempty"`
	Font      string  `json:"font,omitempty"`
}

// Canonical 校验并重新序列化文档正文，保证摘要稳定。
func Canonical(kind string, raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, apperrors.BadRequest(400, "文档内容不能为空")
	}
	if len(raw) > maxContentBytes {
		return nil, apperrors.TooLarge(413, "文档内容超过 1MB 限制")
	}
	switch kind {
	case KindWord:
		var content wordContent
		if err := decodeContent(raw, &content); err != nil {
			return nil, err
		}
		if err := validateWord(&content); err != nil {
			return nil, err
		}
		return marshalContent(content)
	case KindExcel:
		var content excelContent
		if err := decodeContent(raw, &content); err != nil {
			return nil, err
		}
		if err := validateExcel(&content); err != nil {
			return nil, err
		}
		return marshalContent(content)
	case KindPPT:
		var content pptContent
		if err := decodeContent(raw, &content); err != nil {
			return nil, err
		}
		if err := validatePPT(&content); err != nil {
			return nil, err
		}
		return marshalContent(content)
	default:
		return nil, apperrors.BadRequest(400, "不支持的文档类型")
	}
}

// Blank 返回新建文档的初始正文。
func Blank(kind string) ([]byte, error) {
	switch kind {
	case KindWord:
		return Canonical(kind, []byte(`{"blocks":[{"id":"b1","type":"paragraph","align":"left","runs":[{"text":"","bold":false,"italic":false,"underline":false}]}]}`))
	case KindExcel:
		return Canonical(kind, []byte(`{"sheets":[{"name":"Sheet1","row_count":40,"col_count":16,"cells":[]}]}`))
	case KindPPT:
		return Canonical(kind, []byte(`{"slides":[{"id":"s1","background":"#ffffff","elements":[{"id":"e1","kind":"text","x":68,"y":132,"w":824,"h":92,"text":"点击可添加标题","font_size":40,"color":"#202124","bold":true,"align":"center"},{"id":"e2","kind":"text","x":68,"y":228,"w":824,"h":68,"text":"点击可添加副标题","font_size":22,"color":"#5f6368","bold":false,"align":"center"}]}]}`))
	default:
		return nil, apperrors.BadRequest(400, "不支持的文档类型")
	}
}

func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func decodeContent(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return apperrors.BadRequest(400, "文档内容格式不正确")
	}
	if decoder.More() {
		return apperrors.BadRequest(400, "文档内容格式不正确")
	}
	return nil
}

func marshalContent(value any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	encoded := bytes.TrimRight(buf.Bytes(), "\n")
	if len(encoded) > maxContentBytes {
		return nil, apperrors.TooLarge(413, "文档内容超过 1MB 限制")
	}
	return encoded, nil
}

func validateWord(content *wordContent) error {
	if len(content.Blocks) < 1 || len(content.Blocks) > 400 {
		return apperrors.BadRequest(400, "文稿段落数量无效")
	}
	seen := map[string]struct{}{}
	total := 0
	for i := range content.Blocks {
		block := &content.Blocks[i]
		if !idPattern.MatchString(block.ID) {
			return apperrors.BadRequest(400, "文稿段落标识无效")
		}
		if _, ok := seen[block.ID]; ok {
			return apperrors.BadRequest(400, "文稿段落标识重复")
		}
		seen[block.ID] = struct{}{}
		switch block.Type {
		case "paragraph", "heading1", "heading2", "heading3", "bullet", "numbered", "quote", "break":
		default:
			return apperrors.BadRequest(400, "文稿段落类型无效")
		}
		switch block.Align {
		case "left", "center", "right", "justify":
		default:
			return apperrors.BadRequest(400, "文稿对齐方式无效")
		}
		if block.Indent < 0 || block.Indent > 8 {
			return apperrors.BadRequest(400, "文稿缩进无效")
		}
		switch block.Spacing {
		case "", "1.5", "2":
		default:
			return apperrors.BadRequest(400, "文稿行距无效")
		}
		if len(block.Runs) < 1 || len(block.Runs) > 100 {
			return apperrors.BadRequest(400, "文稿文本片段数量无效")
		}
		for i := range block.Runs {
			if err := normalizeRun(&block.Runs[i]); err != nil {
				return err
			}
		}
		block.Runs = normalizeRuns(block.Runs)
		blockRunes := 0
		for _, run := range block.Runs {
			if hasControl(run.Text, false) {
				return apperrors.BadRequest(400, "文稿包含不支持的控制字符")
			}
			blockRunes += utf8.RuneCountInString(run.Text)
		}
		if blockRunes > 8000 {
			return apperrors.BadRequest(400, "单个段落过长")
		}
		total += blockRunes
	}
	if total > 100000 {
		return apperrors.BadRequest(400, "文稿内容过长")
	}
	return nil
}

func normalizeRuns(runs []wordRun) []wordRun {
	out := make([]wordRun, 0, len(runs))
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		if len(out) > 0 {
			prev := &out[len(out)-1]
			if sameRunStyle(*prev, run) {
				prev.Text += run.Text
				continue
			}
		}
		out = append(out, run)
	}
	if len(out) == 0 {
		return []wordRun{{}}
	}
	return out
}

func normalizeRun(run *wordRun) error {
	run.Color = strings.ToLower(run.Color)
	run.Highlight = strings.ToLower(run.Highlight)
	if run.Color != "" && !colorPattern.MatchString(run.Color) {
		return apperrors.BadRequest(400, "文字颜色无效")
	}
	if run.Highlight != "" && !colorPattern.MatchString(run.Highlight) {
		return apperrors.BadRequest(400, "文字高亮无效")
	}
	if run.Size != 0 && (run.Size < 8 || run.Size > 72) {
		return apperrors.BadRequest(400, "字号无效")
	}
	switch run.Font {
	case "", "serif", "sans", "mono":
	default:
		return apperrors.BadRequest(400, "字体无效")
	}
	return nil
}

func sameRunStyle(a, b wordRun) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Underline == b.Underline && a.Strike == b.Strike && a.Color == b.Color && a.Highlight == b.Highlight && a.Size == b.Size && a.Font == b.Font
}

func validateExcel(content *excelContent) error {
	if len(content.Sheets) == 0 {
		if content.SheetName == "" && content.RowCount == 0 && content.ColCount == 0 && len(content.Cells) == 0 {
			return apperrors.BadRequest(400, "表格内容无效")
		}
		content.Sheets = []excelSheet{{
			Name:     content.SheetName,
			RowCount: content.RowCount,
			ColCount: content.ColCount,
			Cells:    content.Cells,
		}}
		content.Active = 0
	}
	if len(content.Sheets) > 20 {
		return apperrors.BadRequest(400, "工作表数量无效")
	}
	if content.Active < 0 || content.Active >= len(content.Sheets) {
		return apperrors.BadRequest(400, "当前工作表无效")
	}
	names := map[string]struct{}{}
	for i := range content.Sheets {
		if err := validateSheet(&content.Sheets[i], names); err != nil {
			return err
		}
	}
	content.SheetName = ""
	content.RowCount = 0
	content.ColCount = 0
	content.Cells = nil
	return nil
}

func validateSheet(sheet *excelSheet, names map[string]struct{}) error {
	sheet.Name = trimSpace(sheet.Name)
	if sheet.Name == "" || utf8.RuneCountInString(sheet.Name) > 31 || hasControl(sheet.Name, false) || strings.ContainsAny(sheet.Name, `:\/?*[]`) {
		return apperrors.BadRequest(400, "工作表名称无效")
	}
	if _, ok := names[strings.ToLower(sheet.Name)]; ok {
		return apperrors.BadRequest(400, "工作表名称重复")
	}
	names[strings.ToLower(sheet.Name)] = struct{}{}
	if sheet.RowCount < 1 || sheet.RowCount > 120 || sheet.ColCount < 1 || sheet.ColCount > 30 {
		return apperrors.BadRequest(400, "表格行列范围无效")
	}
	if err := validateWidths(sheet); err != nil {
		return err
	}
	if sheet.Cells == nil {
		sheet.Cells = []excelCell{}
	}
	kept := make([]excelCell, 0, len(sheet.Cells))
	seen := map[[2]int]struct{}{}
	for i := range sheet.Cells {
		cell := sheet.Cells[i]
		cell.Input = strings.ReplaceAll(strings.ReplaceAll(cell.Input, "\r\n", "\n"), "\r", "\n")
		cell.Color = strings.ToLower(cell.Color)
		cell.Fill = strings.ToLower(cell.Fill)
		if cell.Format == "general" {
			cell.Format = ""
		}
		if cell.Row < 1 || cell.Row > sheet.RowCount || cell.Col < 1 || cell.Col > sheet.ColCount {
			return apperrors.BadRequest(400, "单元格位置无效")
		}
		key := [2]int{cell.Row, cell.Col}
		if _, ok := seen[key]; ok {
			return apperrors.BadRequest(400, "单元格重复")
		}
		seen[key] = struct{}{}
		if utf8.RuneCountInString(cell.Input) > 2000 || hasControl(cell.Input, true) {
			return apperrors.BadRequest(400, "单元格内容过长或包含控制字符")
		}
		if cell.Align != "" && cell.Align != "left" && cell.Align != "center" && cell.Align != "right" {
			return apperrors.BadRequest(400, "单元格对齐无效")
		}
		if cell.Format != "" && cell.Format != "number" && cell.Format != "percent" && cell.Format != "date" && cell.Format != "text" {
			return apperrors.BadRequest(400, "单元格格式无效")
		}
		if cell.Color != "" && !colorPattern.MatchString(cell.Color) {
			return apperrors.BadRequest(400, "单元格颜色无效")
		}
		if cell.Fill != "" && !colorPattern.MatchString(cell.Fill) {
			return apperrors.BadRequest(400, "单元格填充无效")
		}
		if cell.Input == "" && !cell.Bold && !cell.Italic && !cell.Underline && cell.Align == "" && cell.Color == "" && cell.Fill == "" && cell.Format == "" {
			continue
		}
		kept = append(kept, cell)
	}
	if len(kept) > sheet.RowCount*sheet.ColCount {
		return apperrors.BadRequest(400, "单元格数量无效")
	}
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].Row == kept[j].Row {
			return kept[i].Col < kept[j].Col
		}
		return kept[i].Row < kept[j].Row
	})
	sheet.Cells = kept
	return nil
}

func validateWidths(sheet *excelSheet) error {
	if len(sheet.ColWidths) == 0 {
		sheet.ColWidths = nil
		return nil
	}
	if len(sheet.ColWidths) != sheet.ColCount {
		return apperrors.BadRequest(400, "列宽数量无效")
	}
	used := false
	for i, width := range sheet.ColWidths {
		if width == 0 {
			continue
		}
		if width < 48 || width > 360 {
			return apperrors.BadRequest(400, "列宽无效")
		}
		sheet.ColWidths[i] = width
		used = true
	}
	if !used {
		sheet.ColWidths = nil
	}
	return nil
}

func validatePPT(content *pptContent) error {
	if len(content.Slides) < 1 || len(content.Slides) > 30 {
		return apperrors.BadRequest(400, "幻灯片数量无效")
	}
	slides := map[string]struct{}{}
	for i := range content.Slides {
		slide := &content.Slides[i]
		if !idPattern.MatchString(slide.ID) {
			return apperrors.BadRequest(400, "幻灯片标识无效")
		}
		if _, ok := slides[slide.ID]; ok {
			return apperrors.BadRequest(400, "幻灯片标识重复")
		}
		slides[slide.ID] = struct{}{}
		if !colorPattern.MatchString(slide.Background) {
			return apperrors.BadRequest(400, "幻灯片背景色无效")
		}
		slide.Background = strings.ToLower(slide.Background)
		if utf8.RuneCountInString(slide.Notes) > 4000 || hasControl(slide.Notes, true) {
			return apperrors.BadRequest(400, "幻灯片备注过长或包含不支持的字符")
		}
		if slide.Elements == nil {
			slide.Elements = []pptElement{}
		}
		if len(slide.Elements) > 40 {
			return apperrors.BadRequest(400, "幻灯片元素过多")
		}
		elements := map[string]struct{}{}
		for j := range slide.Elements {
			element := &slide.Elements[j]
			if !idPattern.MatchString(element.ID) {
				return apperrors.BadRequest(400, "幻灯片元素标识无效")
			}
			if _, ok := elements[element.ID]; ok {
				return apperrors.BadRequest(400, "幻灯片元素标识重复")
			}
			elements[element.ID] = struct{}{}
			switch element.Kind {
			case "text":
				if element.Shape != "" {
					return apperrors.BadRequest(400, "幻灯片元素类型无效")
				}
			case "shape":
				switch element.Shape {
				case "rect", "ellipse", "round":
				default:
					return apperrors.BadRequest(400, "幻灯片形状无效")
				}
			default:
				return apperrors.BadRequest(400, "幻灯片元素类型无效")
			}
			if !finite(element.X) || !finite(element.Y) || !finite(element.W) || !finite(element.H) || !finite(element.FontSize) {
				return apperrors.BadRequest(400, "幻灯片元素尺寸无效")
			}
			if element.X < -200 || element.X > 1100 || element.Y < -200 || element.Y > 700 || element.W < 24 || element.W > 960 || element.H < 24 || element.H > 540 {
				return apperrors.BadRequest(400, "幻灯片元素尺寸超出画布")
			}
			if element.FontSize < 12 || element.FontSize > 96 {
				return apperrors.BadRequest(400, "字号无效")
			}
			element.Color = strings.ToLower(element.Color)
			element.Fill = strings.ToLower(element.Fill)
			if !colorPattern.MatchString(element.Color) {
				return apperrors.BadRequest(400, "文字颜色无效")
			}
			if element.Fill != "" && !colorPattern.MatchString(element.Fill) {
				return apperrors.BadRequest(400, "形状填充无效")
			}
			switch element.Font {
			case "", "serif", "sans", "mono":
			default:
				return apperrors.BadRequest(400, "字体无效")
			}
			switch element.Align {
			case "left", "center", "right":
			default:
				return apperrors.BadRequest(400, "文字对齐无效")
			}
			if utf8.RuneCountInString(element.Text) > 2000 || hasControl(element.Text, true) {
				return apperrors.BadRequest(400, "幻灯片文字过长或包含不支持的字符")
			}
		}
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func trimSpace(value string) string {
	return strings.TrimSpace(value)
}
