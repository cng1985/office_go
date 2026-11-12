package document

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
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
	var images [][]byte
	for _, block := range content.Blocks {
		if block.Type != "image" {
			continue
		}
		raw, ok := jpegPayload(block.Image)
		if ok {
			images = append(images, raw)
		}
	}
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`)
	if len(images) > 0 {
		body.WriteString(` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"`)
	}
	body.WriteString(`><w:body>`)
	imageNo := 0
	for i, block := range content.Blocks {
		if numIDs[i] > maxNumID {
			maxNumID = numIDs[i]
		}
		if block.Type == "image" {
			if _, ok := jpegPayload(block.Image); !ok {
				continue
			}
			imageNo++
			writeWordImage(&body, block, imageNo)
			continue
		}
		writeWordParagraph(&body, block, numIDs[i])
	}
	body.WriteString(`</w:body></w:document>`)
	files := map[string]string{
		"[Content_Types].xml":          wordContentTypesXML(len(images) > 0),
		"_rels/.rels":                  wordRootRels,
		"word/document.xml":            body.String(),
		"word/_rels/document.xml.rels": wordDocumentRelsXML(len(images)),
		"word/styles.xml":              wordStyles,
		"word/numbering.xml":           wordNumberingXML(maxNumID),
	}
	for i, data := range images {
		files[fmt.Sprintf("word/media/image%d.jpeg", i+1)] = string(data)
	}
	return packZip(files)
}

func wordContentTypesXML(withJPEG bool) string {
	if !withJPEG {
		return wordContentTypes
	}
	return strings.Replace(wordContentTypes, `<Default Extension="rels"`, `<Default Extension="jpeg" ContentType="image/jpeg"/><Default Extension="rels"`, 1)
}

func wordDocumentRelsXML(imageCount int) string {
	if imageCount == 0 {
		return wordDocumentRels
	}
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	body.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	body.WriteString(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`)
	for i := 1; i <= imageCount; i++ {
		fmt.Fprintf(&body, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image%d.jpeg"/>`, i+2, i)
	}
	body.WriteString(`</Relationships>`)
	return body.String()
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

func writeWordImage(body *strings.Builder, block wordBlock, n int) {
	raw, ok := jpegPayload(block.Image)
	if !ok {
		return
	}
	cx, cy := wordImageExtent(raw)
	if block.Width >= 20 && block.Width < 100 {
		cx = cx * block.Width / 100
		cy = cy * block.Width / 100
	}
	fmt.Fprintf(body, `<w:p><w:pPr><w:jc w:val="%s"/></w:pPr>`, wordAlignOut(block.Align))
	fmt.Fprintf(body, `<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/>`, cx, cy)
	fmt.Fprintf(body, `<wp:docPr id="%d" name="Picture %d"/>`, n, n)
	fmt.Fprintf(body, `<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="%d" name="image%d.jpeg"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill>`, n, n)
	fmt.Fprintf(body, `<a:blip r:embed="rId%d"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`, n+2)
	fmt.Fprintf(body, `<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`, cx, cy)
}

func imageWidthPercent(dataURL string, cx int) int {
	if cx <= 0 {
		return 0
	}
	raw, ok := jpegPayload(dataURL)
	if !ok {
		return 0
	}
	natural, _ := wordImageExtent(raw)
	if natural <= 0 {
		return 0
	}
	percent := (cx*100 + natural/2) / natural
	if percent >= 95 {
		return 0
	}
	if percent < 20 {
		return 20
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func wordImageExtent(data []byte) (int, int) {
	width, height, _, ok := jpegInfo(data)
	if !ok {
		return 5486400, 3200400
	}
	const emuPerPixel = 9525
	const maxWidth = 5943600
	cx := width * emuPerPixel
	cy := height * emuPerPixel
	if cx > maxWidth && cx > 0 {
		cy = cy * maxWidth / cx
		cx = maxWidth
	}
	if cx < emuPerPixel {
		cx = emuPerPixel
	}
	if cy < emuPerPixel {
		cy = emuPerPixel
	}
	return cx, cy
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
	blocks, err := parseWordDocument(files["word/document.xml"], wordImageParts(files))
	if err != nil {
		return "", "", nil, readFailure(err)
	}
	blocks = append(wordPartImages(files, "/header"), blocks...)
	blocks = append(blocks, wordPartImages(files, "/footer")...)
	for i := range blocks {
		blocks[i].ID = idAt("b", i+1)
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

type wordImageAt struct {
	at    int
	data  string
	width int
}

type wordDraft struct {
	kind      string
	align     string
	indent    int
	runs      []wordRun
	pageBreak bool
	image     string
	width     int
	images    []wordImageAt
}

func wordImageParts(files map[string][]byte) map[string]string {
	return imagePartsFromRels(files, files["word/_rels/document.xml.rels"])
}

func imagePartsFromRels(files map[string][]byte, rels []byte) map[string]string {
	return imagePartsIn(files, rels, "word")
}

func wordPartImages(files map[string][]byte, relType string) []wordBlock {
	var blocks []wordBlock
	_ = walkXML(files["word/_rels/document.xml.rels"], func(token xml.Token) error {
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" || !strings.HasSuffix(attr(start, "Type"), relType) {
			return nil
		}
		part := wordPartPath(attr(start, "Target"))
		if part == "" {
			return nil
		}
		relsPath := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
		parsed, err := parseWordDocument(files[part], imagePartsFromRels(files, files[relsPath]))
		if err != nil {
			return nil
		}
		for _, block := range parsed {
			if block.Type == "image" {
				blocks = append(blocks, block)
			}
		}
		return nil
	})
	return blocks
}

func wordPartPath(target string) string {
	target = strings.ReplaceAll(target, "\\", "/")
	if decoded, err := url.PathUnescape(target); err == nil {
		target = decoded
	}
	var name string
	if strings.HasPrefix(target, "/") {
		name = strings.TrimPrefix(path.Clean(target), "/")
	} else {
		name = path.Clean(path.Join("word", target))
	}
	if name == "." || strings.Contains(name, "..") {
		return ""
	}
	return name
}

func imagePartsIn(files map[string][]byte, rels []byte, base string) map[string]string {
	if len(rels) == 0 {
		return nil
	}
	images := map[string]string{}
	_ = walkXML(rels, func(token xml.Token) error {
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" {
			return nil
		}
		id := attr(start, "Id")
		if data, ok := wordImageURL(files, base, attr(start, "Target"), attr(start, "Type"), attr(start, "TargetMode")); ok {
			images[id] = data
		}
		return nil
	})
	return images
}

func wordImageURL(files map[string][]byte, base, target, typ, mode string) (string, bool) {
	if !strings.HasSuffix(typ, "/image") || strings.EqualFold(mode, "External") {
		return "", false
	}
	target = strings.ReplaceAll(target, "\\", "/")
	if cut := strings.IndexAny(target, "?#"); cut >= 0 {
		target = target[:cut]
	}
	if decoded, err := url.PathUnescape(target); err == nil {
		target = decoded
	}
	var name string
	if strings.HasPrefix(target, "/") {
		name = strings.TrimPrefix(path.Clean(target), "/")
	} else {
		name = path.Clean(path.Join(base, target))
	}
	if name == "." || strings.Contains(name, "..") {
		return "", false
	}
	raw := files[name]
	if raw == nil {
		want := strings.ToLower(name)
		for key, value := range files {
			if strings.ToLower(key) == want {
				raw = value
				break
			}
		}
	}
	return officeImageDataURL(raw)
}

func parseWordDocument(data []byte, images map[string]string) ([]wordBlock, error) {
	var drafts []wordDraft
	var current *wordDraft
	var run *wordRun
	var text strings.Builder
	inRunProps := 0
	readingText := false
	fallback := 0
	pendingExtent := 0
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
				if !wordMain(node.Name) {
					break
				}
				if current != nil {
					drafts = append(drafts, *current)
				}
				current = &wordDraft{kind: "paragraph", align: "left"}
				pendingExtent = 0
			case "extent", "ext":
				if n, err := strconv.Atoi(attr(node, "cx")); err == nil && n > 0 {
					if current != nil && len(current.images) > 0 && current.images[len(current.images)-1].width == 0 {
						last := &current.images[len(current.images)-1]
						last.width = imageWidthPercent(last.data, n)
					} else {
						pendingExtent = n
					}
				}
			case "blip", "imagedata":
				if current == nil {
					current = &wordDraft{kind: "paragraph", align: "left"}
				}
				ref := attr(node, "embed")
				if ref == "" {
					ref = attr(node, "id")
				}
				if data := images[ref]; data != "" {
					if run != nil && text.Len() > 0 {
						flushed := *run
						flushed.Text = text.String()
						current.runs = append(current.runs, flushed)
						text.Reset()
					}
					item := wordImageAt{at: len(current.runs), data: data}
					if pendingExtent > 0 {
						item.width = imageWidthPercent(data, pendingExtent)
						pendingExtent = 0
					}
					current.images = append(current.images, item)
				}
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
				if !wordMain(node.Name) {
					break
				}
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
				if !wordMain(node.Name) {
					break
				}
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
				if wordMain(node.Name) {
					readingText = false
				}
			case "r":
				if !wordMain(node.Name) {
					break
				}
				if current != nil && run != nil {
					run.Text = text.String()
					current.runs = append(current.runs, *run)
				}
				run = nil
				text.Reset()
			case "p":
				if !wordMain(node.Name) {
					break
				}
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
	if current != nil {
		drafts = append(drafts, *current)
	}
	blocks := expandWordDrafts(flattenWordImages(drafts))
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

func flattenWordImages(drafts []wordDraft) []wordDraft {
	out := make([]wordDraft, 0, len(drafts))
	for _, draft := range drafts {
		out = append(out, splitDraftOnImages(draft)...)
	}
	return out
}

func splitDraftOnImages(draft wordDraft) []wordDraft {
	if len(draft.images) == 0 {
		return []wordDraft{draft}
	}
	pageBreak := draft.pageBreak
	draft.pageBreak = false
	var out []wordDraft
	var runs []wordRun
	next := 0
	hasText := false
	flushRuns := func() {
		if len(runs) == 0 {
			return
		}
		piece := draft
		piece.runs = append([]wordRun(nil), runs...)
		piece.images = nil
		out = append(out, piece)
		runs = nil
	}
	for i, run := range draft.runs {
		for next < len(draft.images) && draft.images[next].at <= i {
			flushRuns()
			out = append(out, wordDraft{kind: "paragraph", align: draft.align, indent: draft.indent, image: draft.images[next].data, width: draft.images[next].width})
			next++
		}
		if run.Text == "" {
			continue
		}
		hasText = true
		runs = append(runs, run)
	}
	for next < len(draft.images) {
		flushRuns()
		out = append(out, wordDraft{kind: "paragraph", align: draft.align, indent: draft.indent, image: draft.images[next].data, width: draft.images[next].width})
		next++
	}
	flushRuns()
	if pageBreak && !hasText {
		out = append(out, wordDraft{kind: draft.kind, align: draft.align, indent: draft.indent, pageBreak: true})
	}
	if len(out) == 0 {
		draft.pageBreak = pageBreak
		draft.images = nil
		return []wordDraft{draft}
	}
	return out
}

func expandWordDrafts(drafts []wordDraft) []wordBlock {
	blocks := make([]wordBlock, 0, len(drafts))
	for _, draft := range drafts {
		if draft.image != "" {
			blocks = append(blocks, wordBlock{
				ID:     idAt("b", len(blocks)+1),
				Type:   "image",
				Align:  draft.align,
				Indent: draft.indent,
				Runs:   []wordRun{{}},
				Image:  draft.image,
				Width:  draft.width,
			})
			continue
		}
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

func wordMain(name xml.Name) bool {
	return name.Space == "" || strings.Contains(name.Space, "wordprocessingml")
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
