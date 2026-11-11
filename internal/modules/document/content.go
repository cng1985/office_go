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
	ID    string    `json:"id"`
	Type  string    `json:"type"`
	Align string    `json:"align"`
	Runs  []wordRun `json:"runs"`
}

type wordRun struct {
	Text      string `json:"text"`
	Bold      bool   `json:"bold"`
	Italic    bool   `json:"italic"`
	Underline bool   `json:"underline"`
}

type excelContent struct {
	SheetName string      `json:"sheet_name"`
	RowCount  int         `json:"row_count"`
	ColCount  int         `json:"col_count"`
	Cells     []excelCell `json:"cells"`
}

type excelCell struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Input string `json:"input"`
	Bold  bool   `json:"bold"`
}

type pptContent struct {
	Slides []pptSlide `json:"slides"`
}

type pptSlide struct {
	ID         string       `json:"id"`
	Background string       `json:"background"`
	Elements   []pptElement `json:"elements"`
}

type pptElement struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	W        float64 `json:"w"`
	H        float64 `json:"h"`
	Text     string  `json:"text"`
	FontSize float64 `json:"font_size"`
	Color    string  `json:"color"`
	Bold     bool    `json:"bold"`
	Align    string  `json:"align"`
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
		return Canonical(kind, []byte(`{"sheet_name":"Sheet1","row_count":30,"col_count":12,"cells":[]}`))
	case KindPPT:
		return Canonical(kind, []byte(`{"slides":[{"id":"s1","background":"#fffdf8","elements":[{"id":"e1","kind":"text","x":72,"y":78,"w":760,"h":92,"text":"单击编辑标题","font_size":40,"color":"#1c1915","bold":true,"align":"left"}]}]}`))
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
		case "paragraph", "heading1", "heading2", "bullet", "numbered":
		default:
			return apperrors.BadRequest(400, "文稿段落类型无效")
		}
		switch block.Align {
		case "left", "center", "right":
		default:
			return apperrors.BadRequest(400, "文稿对齐方式无效")
		}
		if len(block.Runs) < 1 || len(block.Runs) > 100 {
			return apperrors.BadRequest(400, "文稿文本片段数量无效")
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
			if prev.Bold == run.Bold && prev.Italic == run.Italic && prev.Underline == run.Underline {
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

func validateExcel(content *excelContent) error {
	content.SheetName = trimSpace(content.SheetName)
	if content.SheetName == "" || utf8.RuneCountInString(content.SheetName) > 31 || hasControl(content.SheetName, false) {
		return apperrors.BadRequest(400, "工作表名称无效")
	}
	if content.RowCount < 1 || content.RowCount > 100 || content.ColCount < 1 || content.ColCount > 26 {
		return apperrors.BadRequest(400, "表格行列范围无效")
	}
	if content.Cells == nil {
		content.Cells = []excelCell{}
	}
	if len(content.Cells) > content.RowCount*content.ColCount {
		return apperrors.BadRequest(400, "单元格数量无效")
	}
	seen := map[[2]int]struct{}{}
	for i := range content.Cells {
		cell := &content.Cells[i]
		if cell.Row < 1 || cell.Row > content.RowCount || cell.Col < 1 || cell.Col > content.ColCount {
			return apperrors.BadRequest(400, "单元格位置无效")
		}
		key := [2]int{cell.Row, cell.Col}
		if _, ok := seen[key]; ok {
			return apperrors.BadRequest(400, "单元格重复")
		}
		seen[key] = struct{}{}
		if utf8.RuneCountInString(cell.Input) > 1000 || hasControl(cell.Input, false) {
			return apperrors.BadRequest(400, "单元格内容过长或包含控制字符")
		}
	}
	sort.Slice(content.Cells, func(i, j int) bool {
		if content.Cells[i].Row == content.Cells[j].Row {
			return content.Cells[i].Col < content.Cells[j].Col
		}
		return content.Cells[i].Row < content.Cells[j].Row
	})
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
			if element.Kind != "text" {
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
			if !colorPattern.MatchString(element.Color) {
				return apperrors.BadRequest(400, "文字颜色无效")
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
