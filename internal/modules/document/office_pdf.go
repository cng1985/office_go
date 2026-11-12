package document

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	apperrors "yunjian/internal/platform/errors"
)

const pdfMediaType = "application/pdf"

const (
	pdfPageW  = 595.28
	pdfPageH  = 841.89
	pdfMargin = 64.0
)

type pdfStyle struct {
	size      float64
	bold      bool
	italic    bool
	underline bool
	strike    bool
	color     string
	highlight string
}

type pdfAtom struct {
	r     rune
	gid   uint16
	adv   float64
	style pdfStyle
}

type pdfPen struct {
	pages [][]byte
	cur   bytes.Buffer
	y     float64
	dirty bool
}

func exportPDF(content []byte) ([]byte, error) {
	var doc wordContent
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	face, err := loadPDFFace()
	if err != nil {
		return nil, err
	}
	bullet := pdfBullet(face)
	needed := neededPDFRunes(doc, bullet)
	sub, err := face.subset(needed)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	for _, r := range needed {
		if r == ' ' {
			continue
		}
		if _, ok := sub.glyph[r]; !ok {
			return nil, apperrors.BadRequest(400, "有字符无法写入 PDF")
		}
	}
	pages, err := layoutPDF(doc, sub, bullet)
	if err != nil {
		return nil, err
	}
	body, err := writePDF(pages, sub)
	if err != nil {
		return nil, err
	}
	if len(body) > maxUploadBytes {
		return nil, apperrors.TooLarge(413, "导出的 PDF 超过 1MB 限制")
	}
	return body, nil
}

func pdfBullet(face *ttfFont) rune {
	for _, r := range []rune{'•', '·', '-'} {
		if face.glyphID(r) != 0 {
			return r
		}
	}
	return '-'
}

func neededPDFRunes(doc wordContent, bullet rune) []rune {
	out := []rune{' '}
	numbered := false
	for _, block := range doc.Blocks {
		if block.Type == "bullet" {
			out = append(out, bullet, ' ')
		}
		if block.Type == "numbered" {
			numbered = true
		}
		for _, run := range block.Runs {
			out = append(out, []rune(run.Text)...)
		}
	}
	if numbered {
		out = append(out, []rune("0123456789. ")...)
	}
	return out
}

func layoutPDF(doc wordContent, sub *pdfSubset, bullet rune) ([][]byte, error) {
	pen := &pdfPen{y: pdfPageH - pdfMargin}
	number := 0
	for _, block := range doc.Blocks {
		if block.Type == "break" {
			pen.finish()
			number = 0
			continue
		}
		prefix := ""
		if block.Type == "numbered" {
			number++
			prefix = strconv.Itoa(number) + ". "
		} else {
			number = 0
			if block.Type == "bullet" {
				prefix = string(bullet) + " "
			}
		}
		if err := pen.drawBlock(block, sub, prefix); err != nil {
			return nil, err
		}
	}
	if pen.dirty || len(pen.pages) == 0 {
		pen.finish()
	}
	return pen.pages, nil
}

func (pen *pdfPen) finish() {
	pen.pages = append(pen.pages, append([]byte(nil), pen.cur.Bytes()...))
	pen.cur.Reset()
	pen.y = pdfPageH - pdfMargin
	pen.dirty = false
}

func (pen *pdfPen) drawBlock(block wordBlock, sub *pdfSubset, prefix string) error {
	style := pdfRunStyle(block, wordRun{})
	if len(block.Runs) > 0 {
		style = pdfRunStyle(block, block.Runs[0])
	}
	var atoms []pdfAtom
	atoms = appendPDFText(atoms, prefix, style, sub)
	for _, run := range block.Runs {
		atoms = appendPDFText(atoms, run.Text, pdfRunStyle(block, run), sub)
	}
	indent := float64(block.Indent) * 24
	if block.Type == "quote" {
		indent += 16
	}
	width := pdfPageW - pdfMargin*2 - indent
	if width < 48 {
		width = 48
	}
	lines := wrapAtoms(atoms, width)
	if len(lines) == 0 {
		lines = [][]pdfAtom{nil}
	}
	spacing := pdfSpacing(block.Spacing)
	for i, line := range lines {
		line = trimPDFAtoms(line)
		size := style.size
		for _, atom := range line {
			if atom.style.size > size {
				size = atom.style.size
			}
		}
		step := size * 1.35 * spacing
		if pen.dirty && pen.y-step < pdfMargin {
			pen.finish()
		}
		pen.paintLine(line, block.Align, i == len(lines)-1, width, indent, size)
		pen.y -= step
		pen.dirty = true
	}
	return nil
}

func (pen *pdfPen) paintLine(line []pdfAtom, align string, last bool, maxW, indent, size float64) {
	line = append([]pdfAtom(nil), line...)
	width := 0.0
	spaces := 0
	for _, atom := range line {
		width += atom.adv
		if atom.r == ' ' {
			spaces++
		}
	}
	if align == "justify" && !last && spaces > 0 && maxW-width > 0.5 {
		extra := (maxW - width) / float64(spaces)
		for i := range line {
			if line[i].r == ' ' {
				line[i].adv += extra
			}
		}
		width = maxW
	}
	x := pdfMargin + indent
	switch align {
	case "center":
		x += (maxW - width) / 2
	case "right":
		x += maxW - width
	}
	baseline := pen.y - size
	for _, span := range splitPDFSpans(line) {
		paintPDFSpan(&pen.cur, span, x, baseline)
		for _, atom := range span {
			x += atom.adv
		}
	}
}

func appendPDFText(atoms []pdfAtom, text string, style pdfStyle, sub *pdfSubset) []pdfAtom {
	for _, r := range text {
		gid, ok := sub.glyph[r]
		if !ok || gid == 0 || sub.units == 0 {
			continue
		}
		atoms = append(atoms, pdfAtom{
			r: r, gid: gid, style: style,
			adv: float64(sub.adv[r]) * style.size / float64(sub.units),
		})
	}
	return atoms
}

func pdfRunStyle(block wordBlock, run wordRun) pdfStyle {
	size := float64(run.Size)
	if size <= 0 {
		switch block.Type {
		case "heading1":
			size = 20
		case "heading2":
			size = 16
		case "heading3":
			size = 14
		default:
			size = 11
		}
	}
	color := run.Color
	if color == "" && block.Type == "heading3" {
		color = "#434343"
	}
	return pdfStyle{
		size: size, bold: run.Bold || strings.HasPrefix(block.Type, "heading"),
		italic: run.Italic, underline: run.Underline, strike: run.Strike,
		color: color, highlight: run.Highlight,
	}
}

func pdfSpacing(value string) float64 {
	switch value {
	case "1.5":
		return 1.5
	case "2":
		return 2
	default:
		return 1
	}
}

func wrapAtoms(atoms []pdfAtom, maxW float64) [][]pdfAtom {
	var lines [][]pdfAtom
	i := 0
	for i < len(atoms) {
		width := 0.0
		j := i
		breakAt := -1
		for j < len(atoms) {
			if j > i && width+atoms[j].adv > maxW {
				break
			}
			if atoms[j].r == ' ' {
				breakAt = j
			}
			width += atoms[j].adv
			j++
		}
		if j == i {
			j = i + 1
		}
		end, next := j, j
		if j < len(atoms) && breakAt > i {
			end = breakAt
			next = breakAt + 1
		}
		if end <= i {
			end = i + 1
			next = end
		}
		lines = append(lines, atoms[i:end])
		i = next
	}
	return lines
}

func trimPDFAtoms(line []pdfAtom) []pdfAtom {
	for len(line) > 0 && line[len(line)-1].r == ' ' {
		line = line[:len(line)-1]
	}
	return line
}

func splitPDFSpans(line []pdfAtom) [][]pdfAtom {
	var spans [][]pdfAtom
	for _, atom := range line {
		if len(spans) == 0 || !samePDFStyle(spans[len(spans)-1][0].style, atom.style) {
			spans = append(spans, nil)
		}
		last := len(spans) - 1
		spans[last] = append(spans[last], atom)
	}
	return spans
}

func samePDFStyle(a, b pdfStyle) bool {
	return a.size == b.size && a.bold == b.bold && a.italic == b.italic && a.underline == b.underline && a.strike == b.strike && a.color == b.color && a.highlight == b.highlight
}

func paintPDFSpan(buf *bytes.Buffer, span []pdfAtom, x, baseline float64) {
	if len(span) == 0 {
		return
	}
	style := span[0].style
	width := 0.0
	for _, atom := range span {
		width += atom.adv
	}
	if color, ok := pdfRGB(style.highlight); ok {
		fmt.Fprintf(buf, "q %s rg %.2f %.2f %.2f %.2f re f Q\n", color, x, baseline-style.size*0.22, width, style.size*1.15)
	}
	fill := "0 0 0"
	if color, ok := pdfRGB(style.color); ok {
		fill = color
	}
	skew := 0.0
	if style.italic {
		skew = 0.25
	}
	fmt.Fprintf(buf, "BT /F1 %.2f Tf %s rg %s RG 1 0 %.2f 1 %.2f %.2f Tm\n", style.size, fill, fill, skew, x, baseline)
	if style.bold {
		fmt.Fprintf(buf, "2 Tr %.2f w\n", math.Max(0.45, style.size*0.04))
	}
	buf.WriteByte('<')
	for _, atom := range span {
		fmt.Fprintf(buf, "%04X", atom.gid)
	}
	buf.WriteString("> Tj\n")
	if style.bold {
		buf.WriteString("0 Tr\n")
	}
	buf.WriteString("ET\n")
	if style.underline {
		y := baseline - math.Max(1, style.size*0.08)
		fmt.Fprintf(buf, "%s RG %.2f w %.2f %.2f m %.2f %.2f l S\n", fill, math.Max(0.6, style.size*0.05), x, y, x+width, y)
	}
	if style.strike {
		y := baseline + style.size*0.32
		fmt.Fprintf(buf, "%s RG %.2f w %.2f %.2f m %.2f %.2f l S\n", fill, math.Max(0.6, style.size*0.04), x, y, x+width, y)
	}
}

func pdfRGB(hex string) (string, bool) {
	if len(hex) != 7 || hex[0] != '#' {
		return "", false
	}
	r, err1 := strconv.ParseUint(hex[1:3], 16, 8)
	g, err2 := strconv.ParseUint(hex[3:5], 16, 8)
	b, err3 := strconv.ParseUint(hex[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return "", false
	}
	return fmt.Sprintf("%.3f %.3f %.3f", float64(r)/255, float64(g)/255, float64(b)/255), true
}

func writePDF(pages [][]byte, sub *pdfSubset) ([]byte, error) {
	fontBytes, err := compressPDF(sub.data)
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	cmapBytes, err := compressPDF(toUnicodeCMap(sub))
	if err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	n := len(pages)
	if n == 0 {
		n = 1
		pages = [][]byte{nil}
	}
	total := 7 + n*2
	objs := make([][]byte, total)
	kids := make([]string, n)
	for i := range pages {
		contentID := 8 + i*2
		pageID := contentID + 1
		kids[i] = fmt.Sprintf("%d 0 R", pageID)
		stream, err := compressPDF(pages[i])
		if err != nil {
			return nil, apperrors.WrapInternal(err)
		}
		objs[contentID-1] = pdfStream("/Filter /FlateDecode", stream)
		objs[pageID-1] = []byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Contents %d 0 R /Resources << /Font << /F1 7 0 R >> >> >>\n", pdfPageW, pdfPageH, contentID))
	}
	objs[0] = []byte("<< /Type /Catalog /Pages 2 0 R >>\n")
	objs[1] = []byte(fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>\n", n, strings.Join(kids, " ")))
	objs[2] = pdfStream(fmt.Sprintf("/Filter /FlateDecode /Length1 %d", len(sub.data)), fontBytes)
	objs[3] = []byte(fmt.Sprintf("<< /Type /FontDescriptor /FontName /Yunjian /Flags 4 /Ascent %d /Descent %d /CapHeight %d /ItalicAngle 0 /StemV 80 /FontBBox [%d %d %d %d] /FontFile2 3 0 R >>\n", sub.ascent, sub.descent, sub.ascent, sub.xMin, sub.yMin, sub.xMax, sub.yMax))
	objs[4] = []byte(fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Yunjian /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor 4 0 R /CIDToGIDMap /Identity /DW 1000 %s>>\n", pdfWidths(sub)))
	objs[5] = pdfStream("/Filter /FlateDecode", cmapBytes)
	objs[6] = []byte("<< /Type /Font /Subtype /Type0 /BaseFont /Yunjian /Encoding /Identity-H /DescendantFonts [5 0 R] /ToUnicode 6 0 R >>\n")
	return finishPDF(objs), nil
}

func pdfWidths(sub *pdfSubset) string {
	if len(sub.widths) <= 1 {
		return ""
	}
	parts := make([]string, 0, len(sub.widths)-1)
	for _, width := range sub.widths[1:] {
		parts = append(parts, strconv.Itoa(width))
	}
	return "/W [1 [" + strings.Join(parts, " ") + "]] "
}

func pdfStream(dict string, data []byte) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<< %s /Length %d >>\nstream\n", dict, len(data))
	buf.Write(data)
	buf.WriteString("\nendstream\n")
	return buf.Bytes()
}

func finishPDF(objs [][]byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offs := make([]int, len(objs))
	for i, obj := range objs {
		offs[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n", i+1)
		buf.Write(obj)
		if len(obj) == 0 || obj[len(obj)-1] != '\n' {
			buf.WriteByte('\n')
		}
		buf.WriteString("endobj\n")
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offs {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return buf.Bytes()
}

func compressPDF(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := zlib.NewWriterLevel(&buf, zlib.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toUnicodeCMap(sub *pdfSubset) []byte {
	type pair struct {
		gid uint16
		r   rune
	}
	pairs := make([]pair, 0, len(sub.glyph))
	for r, gid := range sub.glyph {
		if gid > 0 {
			pairs = append(pairs, pair{gid, r})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].gid < pairs[j].gid })
	type span struct {
		start uint16
		end   uint16
		base  rune
	}
	var spans []span
	for _, item := range pairs {
		if item.r > 0xFFFF {
			spans = append(spans, span{item.gid, item.gid, item.r})
			continue
		}
		if len(spans) > 0 {
			last := &spans[len(spans)-1]
			if last.base <= 0xFFFF && item.gid == last.end+1 && item.r == last.base+rune(last.end-last.start)+1 {
				last.end = item.gid
				continue
			}
		}
		spans = append(spans, span{item.gid, item.gid, item.r})
	}
	var body strings.Builder
	body.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	body.WriteString("/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	body.WriteString("/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n")
	body.WriteString("1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for at := 0; at < len(spans); {
		end := at + 100
		if end > len(spans) {
			end = len(spans)
		}
		fmt.Fprintf(&body, "%d beginbfrange\n", end-at)
		for _, item := range spans[at:end] {
			fmt.Fprintf(&body, "<%04X> <%04X> <%s>\n", item.start, item.end, pdfRuneHex(item.base))
		}
		body.WriteString("endbfrange\n")
		at = end
	}
	body.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return []byte(body.String())
}

func pdfRuneHex(r rune) string {
	if r <= 0xFFFF {
		return fmt.Sprintf("%04X", r)
	}
	r -= 0x10000
	hi := 0xD800 + (r >> 10)
	lo := 0xDC00 + (r & 0x3FF)
	return fmt.Sprintf("%04X%04X", hi, lo)
}

func importPDF(filename string, body []byte) (string, string, []byte, error) {
	if len(body) > maxUploadBytes {
		return "", "", nil, apperrors.TooLarge(413, "文件超过 1MB 限制")
	}
	head := body
	if len(head) > 1024 {
		head = head[:1024]
	}
	if !bytes.Contains(head, []byte("%PDF")) {
		return "", "", nil, apperrors.BadRequest(400, "无法读取这份 PDF")
	}
	pages, err := extractPDFPages(body)
	if err != nil {
		return "", "", nil, pdfReadError(err)
	}
	blocks, err := pagesToBlocks(pages)
	if err != nil {
		return "", "", nil, err
	}
	raw, err := json.Marshal(wordContent{Blocks: blocks})
	if err != nil {
		return "", "", nil, apperrors.WrapInternal(err)
	}
	canonical, err := Canonical(KindPDF, raw)
	if err != nil {
		return "", "", nil, err
	}
	return KindPDF, titleFromFilename(filename), canonical, nil
}

func pdfReadError(err error) error {
	switch {
	case errors.Is(err, errPDFEncrypt):
		return apperrors.BadRequest(400, "这份 PDF 已加密，无法导入")
	case errors.Is(err, errPDFFilter):
		return apperrors.BadRequest(400, "这份 PDF 的文字暂时无法读取")
	case errors.Is(err, errPDFLimit):
		return apperrors.BadRequest(400, "这份 PDF 过于复杂，无法导入")
	default:
		return apperrors.BadRequest(400, "无法读取这份 PDF")
	}
}

func pagesToBlocks(pages []pdfPageText) ([]wordBlock, error) {
	var blocks []wordBlock
	started := false
	for _, page := range pages {
		if len(page.lines) == 0 {
			continue
		}
		if started {
			blocks = append(blocks, wordBlock{Type: "break", Align: "left", Runs: []wordRun{{}}})
		}
		started = true
		for _, line := range page.lines {
			block, ok := blockFromPDFLine(line, page.width)
			if !ok {
				continue
			}
			blocks = append(blocks, block)
		}
	}
	if len(blocks) == 0 {
		return nil, apperrors.BadRequest(400, "这份 PDF 没有可编辑的文字")
	}
	if len(blocks) > maxWordBlocks {
		return nil, apperrors.BadRequest(400, "PDF 段落过多，无法导入")
	}
	for i := range blocks {
		blocks[i].ID = idAt("b", i+1)
	}
	return blocks, nil
}

func blockFromPDFLine(line pdfTextLine, width float64) (wordBlock, bool) {
	text := cleanPlain(line.text)
	typ := "paragraph"
	if kind, rest := splitPDFMarker(text); kind != "" {
		typ = kind
		text = cleanPlain(rest)
	} else if line.size >= 18 {
		typ = "heading1"
	} else if line.size >= 15 {
		typ = "heading2"
	} else if line.size >= 13 {
		typ = "heading3"
	}
	if text == "" {
		return wordBlock{}, false
	}
	align := "left"
	if strings.HasPrefix(typ, "heading") && width > 0 {
		switch {
		case line.x > width*0.62:
			align = "right"
		case line.x > width*0.22:
			align = "center"
		}
	}
	return wordBlock{Type: typ, Align: align, Runs: []wordRun{{Text: text}}}, true
}

func splitPDFMarker(text string) (string, string) {
	for _, prefix := range []string{"• ", "· ", "- "} {
		if strings.HasPrefix(text, prefix) {
			return "bullet", strings.TrimPrefix(text, prefix)
		}
	}
	i := 0
	for i < len(text) && i < 3 && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(text) && text[i] == '.' && text[i+1] == ' ' {
		return "numbered", text[i+2:]
	}
	return "", text
}
