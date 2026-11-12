package document

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "yunjian/internal/platform/errors"
)

const (
	maxZipEntries   = 200
	docxMediaType   = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	xlsxMediaType   = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	pptxMediaType   = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	officeXMLHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
)

// OfficeFile 是导出后的办公文件。
type OfficeFile struct {
	Filename  string
	MediaType string
	Body      []byte
}

// ExportFile 把当前正文写成 docx、xlsx、pptx 或 pdf。
func ExportFile(kind, title string, content []byte) (OfficeFile, error) {
	canonical, err := Canonical(kind, content)
	if err != nil {
		return OfficeFile{}, err
	}
	switch kind {
	case KindWord:
		body, err := exportDocx(canonical)
		if err != nil {
			return OfficeFile{}, err
		}
		return OfficeFile{Filename: safeFilename(title, ".docx"), MediaType: docxMediaType, Body: body}, nil
	case KindExcel:
		body, err := exportXlsx(canonical)
		if err != nil {
			return OfficeFile{}, err
		}
		return OfficeFile{Filename: safeFilename(title, ".xlsx"), MediaType: xlsxMediaType, Body: body}, nil
	case KindPPT:
		body, err := exportPptx(canonical)
		if err != nil {
			return OfficeFile{}, err
		}
		return OfficeFile{Filename: safeFilename(title, ".pptx"), MediaType: pptxMediaType, Body: body}, nil
	case KindPDF:
		body, err := exportPDF(canonical)
		if err != nil {
			return OfficeFile{}, err
		}
		return OfficeFile{Filename: safeFilename(title, ".pdf"), MediaType: pdfMediaType, Body: body}, nil
	default:
		return OfficeFile{}, apperrors.BadRequest(400, "不支持的文档类型")
	}
}

// ImportFile 从上传的文件得到文档类型、标题和正文。
func ImportFile(filename string, body []byte) (string, string, []byte, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", "", nil, apperrors.BadRequest(400, "文件内容为空")
	}
	ext := strings.ToLower(path.Ext(path.Base(strings.ReplaceAll(filename, "\\", "/"))))
	switch ext {
	case ".json":
		return importJSON(filename, body)
	case ".txt":
		return importText(filename, body)
	case ".csv":
		return importCSV(filename, body)
	case ".docx":
		return importDocx(filename, body)
	case ".xlsx":
		return importXlsx(filename, body)
	case ".pptx":
		return importPptx(filename, body)
	case ".pdf":
		return importPDF(filename, body)
	default:
		return "", "", nil, apperrors.BadRequest(400, "请上传 docx、xlsx、pptx、pdf、txt、csv 或 json 文件")
	}
}

func importJSON(filename string, body []byte) (string, string, []byte, error) {
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return "", "", nil, apperrors.BadRequest(400, "JSON 文件无法读取")
	}
	var kind, title string
	if raw, ok := probe["kind"]; ok {
		_ = json.Unmarshal(raw, &kind)
	}
	if raw, ok := probe["title"]; ok {
		_ = json.Unmarshal(raw, &title)
	}
	var content []byte
	if raw, ok := probe["content"]; ok {
		content = bytes.TrimSpace(raw)
	}
	if kind == "" {
		switch {
		case probe["blocks"] != nil:
			kind = KindWord
			content = body
		case probe["sheets"] != nil || probe["cells"] != nil || probe["sheet_name"] != nil:
			kind = KindExcel
			content = body
		case probe["slides"] != nil:
			kind = KindPPT
			content = body
		default:
			return "", "", nil, apperrors.BadRequest(400, "无法识别这份 JSON 文件")
		}
	}
	if len(content) == 0 || string(content) == "null" {
		return "", "", nil, apperrors.BadRequest(400, "文件里没有文档内容")
	}
	if strings.TrimSpace(title) == "" {
		title = titleFromFilename(filename)
	}
	canonical, err := Canonical(kind, content)
	if err != nil {
		return "", "", nil, err
	}
	return kind, title, canonical, nil
}

func importText(filename string, body []byte) (string, string, []byte, error) {
	if !utf8.Valid(body) {
		return "", "", nil, apperrors.BadRequest(400, "请使用 UTF-8 文本")
	}
	text := string(bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF}))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	if len(lines) > maxWordBlocks {
		return "", "", nil, apperrors.BadRequest(400, "文稿段落过多，无法导入")
	}
	blocks := make([]wordBlock, 0, len(lines))
	for i, line := range lines {
		blocks = append(blocks, wordBlock{
			ID:    idAt("b", i+1),
			Type:  "paragraph",
			Align: "left",
			Runs:  []wordRun{{Text: cleanPlain(line)}},
		})
	}
	raw, err := json.Marshal(wordContent{Blocks: blocks})
	if err != nil {
		return "", "", nil, apperrors.WrapInternal(err)
	}
	canonical, err := Canonical(KindWord, raw)
	if err != nil {
		return "", "", nil, err
	}
	return KindWord, titleFromFilename(filename), canonical, nil
}

func titleFromFilename(name string) string {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	ext := strings.ToLower(path.Ext(base))
	switch ext {
	case ".docx", ".xlsx", ".pptx", ".pdf", ".txt", ".csv", ".json", ".doc", ".xls", ".ppt":
		base = strings.TrimSpace(base[:len(base)-len(ext)])
	}
	base = strings.Trim(base, ". ")
	base = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, base)
	base = strings.TrimSpace(base)
	if base == "" {
		return "未命名"
	}
	runes := []rune(base)
	if len(runes) > 200 {
		base = string(runes[:200])
	}
	return base
}

func safeFilename(title, ext string) string {
	title = strings.TrimSpace(title)
	title = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, title)
	title = strings.Trim(title, ". ")
	if title == "" {
		title = "未命名"
	}
	runes := []rune(title)
	if len(runes) > 80 {
		title = string(runes[:80])
	}
	return title + ext
}

func packZip(files map[string]string) ([]byte, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetModTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil, apperrors.WrapInternal(err)
		}
		if _, err := io.WriteString(writer, files[name]); err != nil {
			return nil, apperrors.WrapInternal(err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	return buf.Bytes(), nil
}

func readZip(raw []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, apperrors.BadRequest(400, "文件内容无法读取")
	}
	if len(reader.File) > maxZipEntries {
		return nil, apperrors.BadRequest(400, "文件过大或内容过多")
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(strings.TrimPrefix(strings.ReplaceAll(file.Name, "\\", "/"), "/"))
		if name == "." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
			return nil, apperrors.BadRequest(400, "文件内容无法读取")
		}
		rc, err := file.Open()
		if err != nil {
			return nil, apperrors.BadRequest(400, "文件内容无法读取")
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, apperrors.BadRequest(400, "文件内容无法读取")
		}
		files[name] = data
	}
	return files, nil
}

func zipKind(files map[string][]byte) string {
	switch {
	case files["word/document.xml"] != nil:
		return KindWord
	case files["xl/workbook.xml"] != nil:
		return KindExcel
	case files["ppt/presentation.xml"] != nil:
		return KindPPT
	default:
		return ""
	}
}

func walkXML(data []byte, fn func(xml.Token) error) error {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(token); err != nil {
			return err
		}
	}
}

func attr(start xml.StartElement, names ...string) string {
	for _, item := range start.Attr {
		for _, name := range names {
			if item.Name.Local == name {
				return item.Value
			}
		}
	}
	return ""
}

func attrPresent(start xml.StartElement, name string) (string, bool) {
	for _, item := range start.Attr {
		if item.Name.Local == name {
			return item.Value, true
		}
	}
	return "", false
}

func relID(start xml.StartElement) string {
	for _, item := range start.Attr {
		if item.Name.Local == "id" && strings.Contains(item.Name.Space, "relationships") {
			return item.Value
		}
	}
	return ""
}

func hasAncestor(stack []string, name string) bool {
	for _, item := range stack {
		if item == name {
			return true
		}
	}
	return false
}

func xmlEscape(value string) string {
	var buf strings.Builder
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func hexColor(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return ""
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return ""
		}
	}
	return "#" + strings.ToLower(value)
}

func cleanPlain(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
}

func cleanMultiline(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
}

func idAt(prefix string, n int) string {
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if len(digits) == 0 {
		digits = []byte{'0'}
	}
	return prefix + string(digits)
}

func readFailure(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperrors.BadRequest(400, "文件内容无法读取")
}
