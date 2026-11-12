package document

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	apperrors "yunjian/internal/platform/errors"
)

const wordContentTypes = officeXMLHeader + `
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
</Types>`

const wordDocumentRels = officeXMLHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
</Relationships>`

const wordStyles = officeXMLHeader + `
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>
  <w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="64"/><w:szCs w:val="64"/></w:rPr></w:style>
  <w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:b/><w:sz w:val="48"/><w:szCs w:val="48"/></w:rPr></w:style>
  <w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:outlineLvl w:val="2"/></w:pPr><w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr></w:style>
  <w:style w:type="paragraph" w:styleId="ListBullet"><w:name w:val="List Bullet"/><w:basedOn w:val="Normal"/><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr></w:style>
  <w:style w:type="paragraph" w:styleId="ListNumber"><w:name w:val="List Number"/><w:basedOn w:val="Normal"/><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr></w:style>
  <w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:rPr><w:i/></w:rPr></w:style>
</w:styles>`

const wordNumberingAbstract = `
  <w:abstractNum w:abstractNumId="0"><w:multiLevelType w:val="hybridMultilevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>
  <w:abstractNum w:abstractNumId="1"><w:multiLevelType w:val="hybridMultilevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl></w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`

const wordRootRels = officeXMLHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

func exportDocx(canonical []byte) ([]byte, error) {
	var content wordContent
	if err := json.Unmarshal(canonical, &content); err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	numIDs := listNumIDs(content.Blocks)
	maxNumID := 2
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for i, block := range content.Blocks {
		if numIDs[i] > maxNumID {
			maxNumID = numIDs[i]
		}
		writeWordParagraph(&body, block, numIDs[i])
	}
	body.WriteString(`</w:body></w:document>`)
	return packZip(map[string]string{
		"[Content_Types].xml":          wordContentTypes,
		"_rels/.rels":                  wordRootRels,
		"word/document.xml":            body.String(),
		"word/_rels/document.xml.rels": wordDocumentRels,
		"word/styles.xml":              wordStyles,
		"word/numbering.xml":           wordNumberingXML(maxNumID),
	})
}

func listNumIDs(blocks []wordBlock) []int {
	ids := make([]int, len(blocks))
	next := 2
	current := 0
	for i, block := range blocks {
		switch block.Type {
		case "bullet":
			ids[i] = 1
			current = 0
		case "numbered":
			if current == 0 {
				current = next
				next++
			}
			ids[i] = current
		default:
			current = 0
		}
	}
	return ids
}

func wordNumberingXML(maxNumID int) string {
	if maxNumID < 2 {
		maxNumID = 2
	}
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	body.WriteString(wordNumberingAbstract)
	for id := 2; id <= maxNumID; id++ {
		fmt.Fprintf(&body, `<w:num w:numId="%d"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="0"><w:startOverride w:val="1"/></w:lvlOverride></w:num>`, id)
	}
	body.WriteString(`</w:numbering>`)
	return body.String()
}

func writeWordParagraph(body *strings.Builder, block wordBlock, numID int) {
	body.WriteString("<w:p><w:pPr>")
	if style := wordStyleOut(block.Type); style != "" {
		fmt.Fprintf(body, `<w:pStyle w:val="%s"/>`, style)
	}
	if numID > 0 {
		fmt.Fprintf(body, `<w:numPr><w:ilvl w:val="0"/><w:numId w:val="%d"/></w:numPr>`, numID)
	}
	fmt.Fprintf(body, `<w:jc w:val="%s"/>`, wordAlignOut(block.Align))
	if block.Indent > 0 {
		fmt.Fprintf(body, `<w:ind w:left="%d"/>`, block.Indent*420)
	}
	body.WriteString("</w:pPr>")
	if block.Type == "break" && wordTextEmpty(block.Runs) {
		body.WriteString(`<w:r><w:br w:type="page"/></w:r>`)
	} else {
		for _, run := range block.Runs {
			writeWordRun(body, run)
		}
		if block.Type == "break" {
			body.WriteString(`<w:r><w:br w:type="page"/></w:r>`)
		}
	}
	body.WriteString("</w:p>")
}

func writeWordRun(body *strings.Builder, run wordRun) {
	body.WriteString("<w:r><w:rPr>")
	if run.Bold {
		body.WriteString("<w:b/>")
	}
	if run.Italic {
		body.WriteString("<w:i/>")
	}
	if run.Underline {
		body.WriteString(`<w:u w:val="single"/>`)
	}
	if run.Strike {
		body.WriteString("<w:strike/>")
	}
	if run.Size > 0 {
		fmt.Fprintf(body, `<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, run.Size*2, run.Size*2)
	}
	if color := strings.TrimPrefix(run.Color, "#"); color != "" {
		fmt.Fprintf(body, `<w:color w:val="%s"/>`, strings.ToUpper(color))
	}
	if fill := strings.TrimPrefix(run.Highlight, "#"); fill != "" {
		fmt.Fprintf(body, `<w:shd w:val="clear" w:fill="%s"/>`, strings.ToUpper(fill))
	}
	if run.Font != "" {
		ascii, east := wordFontOut(run.Font)
		fmt.Fprintf(body, `<w:rFonts w:ascii="%s" w:hAnsi="%s" w:eastAsia="%s"/>`, xmlEscape(ascii), xmlEscape(ascii), xmlEscape(east))
	}
	body.WriteString(`</w:rPr><w:t xml:space="preserve">`)
	body.WriteString(xmlEscape(run.Text))
	body.WriteString("</w:t></w:r>")
}

func wordTextEmpty(runs []wordRun) bool {
	for _, run := range runs {
		if strings.TrimSpace(run.Text) != "" {
			return false
		}
	}
	return true
}

func wordStyleOut(kind string) string {
	switch kind {
	case "heading1":
		return "Heading1"
	case "heading2":
		return "Heading2"
	case "heading3":
		return "Heading3"
	case "bullet":
		return "ListBullet"
	case "numbered":
		return "ListNumber"
	case "quote":
		return "Quote"
	default:
		return ""
	}
}

func wordAlignOut(align string) string {
	switch align {
	case "center":
		return "center"
	case "right":
		return "right"
	case "justify":
		return "both"
	default:
		return "left"
	}
}

func wordFontOut(font string) (string, string) {
	switch font {
	case "serif":
		return "Times New Roman", "宋体"
	case "mono":
		return "Consolas", "Consolas"
	default:
		return "Arial", "微软雅黑"
	}
}

func importDocx(filename string, body []byte) (string, string, []byte, error) {
	files, err := readZip(body)
	if err != nil {
		return "", "", nil, err
	}
	if zipKind(files) != KindWord {
		return "", "", nil, apperrors.BadRequest(400, "这个文件不是有效的文稿")
	}
	blocks, err := parseWordDocument(files["word/document.xml"])
	if err != nil {
		return "", "", nil, readFailure(err)
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

type wordDraft struct {
	kind      string
	align     string
	indent    int
	runs      []wordRun
	pageBreak bool
}

func parseWordDocument(data []byte) ([]wordBlock, error) {
	var drafts []wordDraft
	var current *wordDraft
	var run *wordRun
	var text strings.Builder
	inRunProps := 0
	readingText := false
	fallback := 0
	err := walkXML(data, func(token xml.Token) error {
		switch node := token.(type) {
		case xml.StartElement:
			if node.Name.Local == "Fallback" {
				fallback++
				return nil
			}
			if fallback > 0 {
				return nil
			}
			switch node.Name.Local {
			case "p":
				current = &wordDraft{kind: "paragraph", align: "left"}
			case "pStyle":
				if current != nil {
					current.kind = wordStyleIn(attr(node, "val"))
				}
			case "jc":
				if current != nil {
					current.align = wordAlignIn(attr(node, "val"))
				}
			case "ind":
				if current != nil {
					current.indent = wordIndent(attr(node, "left"))
				}
			case "r":
				run = &wordRun{}
				text.Reset()
			case "rPr":
				inRunProps++
			case "b":
				if run != nil && inRunProps > 0 {
					run.Bold = flagOn(attr(node, "val"), true)
				}
			case "i":
				if run != nil && inRunProps > 0 {
					run.Italic = flagOn(attr(node, "val"), true)
				}
			case "strike":
				if run != nil && inRunProps > 0 {
					run.Strike = flagOn(attr(node, "val"), true)
				}
			case "u":
				if run != nil && inRunProps > 0 {
					value := strings.ToLower(attr(node, "val"))
					run.Underline = value != "none" && value != "0" && value != "false" && value != "off"
				}
			case "sz":
				if run != nil && inRunProps > 0 {
					if size, err := strconv.Atoi(attr(node, "val")); err == nil {
						size /= 2
						if size >= 8 && size <= 72 {
							run.Size = size
						}
					}
				}
			case "color":
				if run != nil && inRunProps > 0 {
					if color := hexColor(attr(node, "val")); color != "" {
						run.Color = color
					}
				}
			case "shd":
				if run != nil && inRunProps > 0 {
					if color := hexColor(attr(node, "fill")); color != "" {
						run.Highlight = color
					}
				}
			case "rFonts":
				if run != nil && inRunProps > 0 {
					run.Font = wordFontIn(attr(node, "ascii"), attr(node, "eastAsia"))
				}
			case "br":
				if current != nil && strings.EqualFold(attr(node, "type"), "page") {
					current.pageBreak = true
				} else if run != nil {
					text.WriteByte('\n')
				}
			case "tab":
				if run != nil {
					text.WriteByte(' ')
				}
			case "t":
				readingText = true
			}
		case xml.EndElement:
			if node.Name.Local == "Fallback" && fallback > 0 {
				fallback--
				return nil
			}
			if fallback > 0 {
				return nil
			}
			switch node.Name.Local {
			case "rPr":
				if inRunProps > 0 {
					inRunProps--
				}
			case "t":
				readingText = false
			case "r":
				if current != nil && run != nil {
					run.Text = text.String()
					current.runs = append(current.runs, *run)
				}
				run = nil
				text.Reset()
			case "p":
				if current != nil {
					drafts = append(drafts, *current)
				}
				current = nil
			}
		case xml.CharData:
			if fallback > 0 {
				return nil
			}
			if readingText && run != nil {
				text.Write(node)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	blocks := expandWordDrafts(drafts)
	if len(blocks) == 0 {
		blocks = []wordBlock{{
			ID: "b1", Type: "paragraph", Align: "left",
			Runs: []wordRun{{}},
		}}
	}
	if len(blocks) > maxWordBlocks {
		return nil, apperrors.BadRequest(400, "文稿段落过多，无法导入")
	}
	return blocks, nil
}

func expandWordDrafts(drafts []wordDraft) []wordBlock {
	blocks := make([]wordBlock, 0, len(drafts))
	for _, draft := range drafts {
		lines := splitWordLines(draft.runs)
		if len(lines) == 0 {
			lines = [][]wordRun{{{}}}
		}
		for i, runs := range lines {
			kind := draft.kind
			if draft.pageBreak && wordTextEmpty(runs) {
				kind = "break"
			} else if i > 0 && (kind == "heading1" || kind == "heading2" || kind == "heading3") {
				kind = "paragraph"
			}
			if len(runs) == 0 {
				runs = []wordRun{{}}
			}
			blocks = append(blocks, wordBlock{
				ID:     idAt("b", len(blocks)+1),
				Type:   kind,
				Align:  draft.align,
				Indent: draft.indent,
				Runs:   runs,
			})
		}
	}
	return blocks
}

func splitWordLines(runs []wordRun) [][]wordRun {
	lines := [][]wordRun{{}}
	for _, run := range runs {
		parts := strings.Split(strings.ReplaceAll(run.Text, "\r\n", "\n"), "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, []wordRun{})
			}
			next := run
			next.Text = cleanPlain(part)
			lines[len(lines)-1] = append(lines[len(lines)-1], next)
		}
	}
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil
	}
	return lines
}

func wordStyleIn(style string) string {
	switch strings.ToLower(strings.ReplaceAll(style, " ", "")) {
	case "heading1", "title":
		return "heading1"
	case "heading2":
		return "heading2"
	case "heading3":
		return "heading3"
	case "listbullet":
		return "bullet"
	case "listnumber":
		return "numbered"
	case "quote", "intensequote":
		return "quote"
	default:
		return "paragraph"
	}
}

func wordAlignIn(value string) string {
	switch strings.ToLower(value) {
	case "center":
		return "center"
	case "right", "end":
		return "right"
	case "both", "justify", "distribute":
		return "justify"
	default:
		return "left"
	}
}

func wordIndent(left string) int {
	n, err := strconv.Atoi(strings.TrimSpace(left))
	if err != nil || n <= 0 {
		return 0
	}
	level := n / 420
	if level > 8 {
		return 8
	}
	return level
}

func wordFontIn(ascii, east string) string {
	latin := strings.ToLower(ascii)
	switch {
	case strings.Contains(latin, "consol"):
		return "mono"
	case strings.Contains(latin, "times"), strings.Contains(latin, "song"), strings.Contains(east, "宋"):
		return "serif"
	case strings.Contains(latin, "arial"), strings.Contains(latin, "calibri"), strings.Contains(east, "黑"):
		return "sans"
	default:
		return ""
	}
}

func flagOn(value string, missingOn bool) bool {
	if strings.TrimSpace(value) == "" {
		return missingOn
	}
	switch strings.ToLower(value) {
	case "0", "false", "off", "none":
		return false
	default:
		return true
	}
}
