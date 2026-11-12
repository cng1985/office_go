package document

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPDFSubsetKeepsOutlines(t *testing.T) {
	face, err := loadPDFFace()
	if err != nil {
		t.Fatal(err)
	}
	sub, err := face.subset([]rune("Hello 你好"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("font %s subset %d bytes", face.path, len(sub.data))
	parsed, err := parseTTF(sub.data, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []rune{'H', 'e', 'l', 'o', '你', '好'} {
		id := parsed.glyphID(r)
		data := parsed.glyphData(id)
		if id == 0 || len(data) < 8 {
			t.Fatalf("%q glyph %d length %d", string(r), id, len(data))
		}
		if int16(binary.BigEndian.Uint16(data[:2])) < 0 {
			for _, comp := range componentIDs(data) {
				if comp <= 0 || comp >= parsed.num || len(parsed.glyphData(comp)) < 4 {
					t.Fatalf("%q component %d missing in %d glyphs", string(r), comp, parsed.num)
				}
			}
		}
		if parsed.advanceOf(id) <= 0 {
			t.Fatalf("%q advance", string(r))
		}
	}
	if binary.BigEndian.Uint32(parsed.head[12:]) != 0x5F0F3CF5 {
		t.Fatal("font magic")
	}
}

func TestPDFRoundTrip(t *testing.T) {
	raw := []byte(`{"blocks":[
		{"id":"b1","type":"heading1","align":"center","runs":[{"text":"会议纪要","bold":true,"italic":false,"underline":false}]},
		{"id":"b2","type":"paragraph","align":"left","runs":[{"text":"Hello 你好","bold":false,"italic":true,"underline":true,"strike":true,"color":"#112233","highlight":"#ffeeaa"}]},
		{"id":"b3","type":"bullet","align":"left","runs":[{"text":"第一项","bold":false,"italic":false,"underline":false}]},
		{"id":"b4","type":"numbered","align":"left","runs":[{"text":"步骤","bold":false,"italic":false,"underline":false}]},
		{"id":"b5","type":"break","align":"left","runs":[{"text":""}]},
		{"id":"b6","type":"paragraph","align":"left","runs":[{"text":"第二页","bold":false,"italic":false,"underline":false}]}
	]}`)
	canonical, err := Canonical(KindPDF, raw)
	if err != nil {
		t.Fatal(err)
	}
	file, err := ExportFile(KindPDF, "合同", canonical)
	if err != nil {
		t.Fatal(err)
	}
	if file.Filename != "合同.pdf" || file.MediaType != pdfMediaType || !bytes.HasPrefix(file.Body, []byte("%PDF")) || len(file.Body) > maxUploadBytes {
		t.Fatalf("file %s %s %d", file.Filename, file.MediaType, len(file.Body))
	}
	kind, title, got, err := ImportFile(file.Filename, file.Body)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindPDF || title != "合同" {
		t.Fatalf("kind %s title %s", kind, title)
	}
	texts := pdfBlockTexts(t, got)
	joined := strings.Join(texts, "\n")
	for _, want := range []string{"heading1:会议纪要", "paragraph:Hello 你好", "bullet:第一项", "numbered:步骤", "\f", "paragraph:第二页"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
	var doc wordContent
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Blocks[0].Align != "center" {
		t.Fatalf("heading align %s", doc.Blocks[0].Align)
	}

	wide := strings.Repeat("A", 180)
	wideRaw := []byte(`{"blocks":[{"id":"b1","type":"paragraph","align":"left","runs":[{"text":"` + wide + `","bold":false,"italic":false,"underline":false}]}]}`)
	wideFile, err := ExportFile(KindPDF, "宽", wideRaw)
	if err != nil {
		t.Fatal(err)
	}
	_, _, wideGot, err := ImportFile(wideFile.Filename, wideFile.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(wideGot), "A") != 180 {
		t.Fatalf("wrapped text kept %d A's", strings.Count(string(wideGot), "A"))
	}
}

func TestPDFLiteralAndEncodedImport(t *testing.T) {
	literal := "BT /F1 12 Tf 14 TL 72 700 Td (Hello) Tj T* (World) Tj ET"
	kind, title, got, err := ImportFile("记录.PDF", assemblePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(literal), literal),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}, ""))
	if err != nil || kind != KindPDF || title != "记录" {
		t.Fatal(err, kind, title)
	}
	texts := pdfBlockTexts(t, got)
	if strings.Join(texts, "|") != "paragraph:Hello|paragraph:World" {
		t.Fatalf("literal %v", texts)
	}

	chinese := "BT /F1 12 Tf 72 680 Td (你好) Tj ET"
	_, _, got, err = ImportFile("中文.pdf", assemblePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(chinese), chinese),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}, ""))
	if err != nil || !bytes.Contains(got, []byte("你好")) {
		t.Fatal(err, string(got))
	}

	cmap := "1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n2 beginbfchar\n<0001> <0048>\n<0002> <4F60>\nendbfchar\n"
	encoded := "0.24 0 0 -0.24 0 792 cm\nq\n3.125 0 0 3.125 40 40 cm\nBT /F1 16 Tf 1 0 0 -1 8 33 Tm <0001> Tj ET\nBT /F1 16 Tf 1 0 0 -1 8 70 Tm <0002> Tj ET\nQ\n"
	_, _, got, err = ImportFile("编码.pdf", assemblePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(encoded), encoded),
		"<< /Type /Font /Subtype /Type0 /BaseFont /Yunjian /Encoding /Identity-H /ToUnicode 6 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(cmap), cmap),
	}, ""))
	if err != nil {
		t.Fatal(err)
	}
	texts = pdfBlockTexts(t, got)
	if len(texts) < 2 || !strings.Contains(strings.Join(texts, ""), "H") || !strings.Contains(strings.Join(texts, ""), "你") {
		t.Fatalf("encoded %v", texts)
	}
}

func TestPDFRejectsUnreadableFiles(t *testing.T) {
	if _, _, _, err := ImportFile("坏.pdf", []byte("%PDF-1.4\nnot a pdf")); err == nil || !strings.Contains(err.Error(), "无法读取") {
		t.Fatal(err)
	}
	empty := assemblePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >>",
		"<< /Length 5 >>\nstream\nBT ET\nendstream",
	}, "")
	if _, _, _, err := ImportFile("空白.pdf", empty); err == nil || !strings.Contains(err.Error(), "没有可编辑的文字") {
		t.Fatal(err)
	}
	encrypted := assemblePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
	}, "<< /Size 3 /Root 1 0 R /Encrypt 9 0 R >>")
	if _, _, _, err := ImportFile("加密.pdf", encrypted); err == nil || !strings.Contains(err.Error(), "已加密") {
		t.Fatal(err)
	}
	if _, err := Blank(KindPDF); err != nil {
		t.Fatal(err)
	}
}

func TestPDFObjectStreamAndPrev(t *testing.T) {
	if _, _, got, err := ImportFile("流.pdf", objectStreamPDF()); err != nil || !bytes.Contains(got, []byte("Hello")) {
		t.Fatal(err, string(got))
	}
	if _, _, got, err := ImportFile("链接.pdf", prevXRefPDF()); err != nil || !bytes.Contains(got, []byte("Hello")) {
		t.Fatal(err, string(got))
	}
}

func TestPDFSaveSearchAndUpload(t *testing.T) {
	db := openDB(t)
	owner := scopeOf(t, db, "pdfowner")
	resource := NewResource(NewRepository(db))
	created, err := resource.Create(owner, &DocumentCreateRequest{Kind: KindPDF, Title: "合同"})
	if err != nil || created.Data.Kind != KindPDF {
		t.Fatal(err)
	}
	body := []byte(`{"blocks":[{"id":"b1","type":"paragraph","align":"left","runs":[{"text":"Hello 你好","bold":false,"italic":false,"underline":false}]}]}`)
	if _, err := resource.Save(owner, &DocumentSaveRequest{
		ID: created.Data.ID, Revision: created.Data.Revision, Title: "合同", Content: body,
	}); err != nil {
		t.Fatal(err)
	}
	exported, err := resource.Export(owner, &DocumentExportRequest{ID: created.Data.ID})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(exported.Data.ContentBase64)
	if err != nil || !strings.HasSuffix(exported.Data.Filename, ".pdf") {
		t.Fatal(err, exported.Data.Filename)
	}
	imported, err := resource.Import(owner, &DocumentImportRequest{Filename: exported.Data.Filename, ContentBase64: exported.Data.ContentBase64})
	if err != nil || imported.Data.Kind != KindPDF || imported.Data.Title != "合同" {
		t.Fatal(err)
	}
	if !bytes.Contains(imported.Data.Content, []byte("Hello 你好")) {
		t.Fatalf("content %s", imported.Data.Content)
	}
	found, err := resource.Search(owner, &DocumentSearch{Kind: KindPDF, Page: 1, PageSize: 10})
	if err != nil || found.Total < 2 {
		t.Fatalf("search %+v %v", found, err)
	}
	_ = raw
}

func TestChromePDFIfPresent(t *testing.T) {
	data, err := os.ReadFile("/tmp/chrome.pdf")
	if err != nil {
		t.Skip()
	}
	kind, _, got, err := ImportFile("page.pdf", data)
	if err != nil || kind != KindPDF {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("第一行")) || !bytes.Contains(got, []byte("Hello")) || !bytes.Contains(got, []byte("第二行")) {
		t.Fatalf("chrome pdf %s", got)
	}
}

func pdfBlockTexts(t *testing.T, content []byte) []string {
	t.Helper()
	var doc wordContent
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(doc.Blocks))
	for _, block := range doc.Blocks {
		if block.Type == "break" {
			out = append(out, "\f")
			continue
		}
		text := ""
		if len(block.Runs) > 0 {
			text = block.Runs[0].Text
		}
		out = append(out, block.Type+":"+text)
	}
	return out
}

func TestPDFSkipsImagesAndBoundsForms(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
	content := "BT (Hello) Tj ET\n/Im0 Do"
	image := fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n%s\nendstream", len(jpeg), jpeg)
	page := "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /XObject << /Im0 5 0 R >> >> >>"
	body := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
	if _, _, got, err := ImportFile("图.pdf", assemblePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		page,
		body,
		image,
	}, "")); err != nil || !bytes.Contains(got, []byte("Hello")) {
		t.Fatal(err, string(got))
	}

	form := "BT (Hello) Tj ET\n/Fm Do"
	pageText := "BT (Page) Tj ET\n/Fm Do"
	if _, _, got, err := ImportFile("循环.pdf", formPDF(pageText, form)); err != nil || strings.Count(string(got), "Hello") != 1 || !bytes.Contains(got, []byte("Page")) {
		t.Fatal(err, string(got))
	}

	if _, _, got, err := ImportFile("表单.pdf", formPDF("/Fm Do\n/Fm Do", "BT (Hi) Tj ET")); err != nil || !bytes.Contains(got, []byte("Hi")) {
		t.Fatal(err, string(got))
	}
	var many strings.Builder
	for i := 0; i < maxPDFFormCalls+1; i++ {
		many.WriteString("/Fm Do\n")
	}
	if _, _, _, err := ImportFile("过多.pdf", formPDF(many.String(), "BT (Hi) Tj ET")); err == nil || !strings.Contains(err.Error(), "过于复杂") {
		t.Fatal(err)
	}
}

func formPDF(content, form string) []byte {
	return assemblePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Count 1 /Kids [3 0 R] >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /XObject << /Fm 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 100 100] /Length %d >>\nstream\n%s\nendstream", len(form), form),
	}, "")
}

func assemblePDF(objects []string, trailer string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objects))
	for i, obj := range objects {
		offs[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offs {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	if trailer == "" {
		trailer = fmt.Sprintf("<< /Size %d /Root 1 0 R >>", len(objects)+1)
	}
	fmt.Fprintf(&buf, "trailer\n%s\nstartxref\n%d\n%%%%EOF\n", trailer, xref)
	return buf.Bytes()
}

func objectStreamPDF() []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	write := func(id int, body string) int {
		off := buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
		return off
	}
	content := "BT /F1 12 Tf 72 700 Td (Hello) Tj ET"
	header := "5 0 "
	font := "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	stream := header + font
	off1 := write(1, "<< /Type /Catalog /Pages 2 0 R >>")
	off2 := write(2, "<< /Type /Pages /Count 1 /Kids [3 0 R] >>")
	off3 := write(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")
	off4 := write(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	off6 := write(6, fmt.Sprintf("<< /Type /ObjStm /N 1 /First %d /Length %d >>\nstream\n%s\nendstream", len(header), len(stream), stream))
	xrefAt := buf.Len()
	entries := make([]byte, 0, 48)
	put := func(kind, a, b int) {
		entries = append(entries, byte(kind), byte(a>>24), byte(a>>16), byte(a>>8), byte(a), byte(b))
	}
	put(0, 0, 255)
	put(1, off1, 0)
	put(1, off2, 0)
	put(1, off3, 0)
	put(1, off4, 0)
	put(2, 6, 0)
	put(1, off6, 0)
	put(1, xrefAt, 0)
	fmt.Fprintf(&buf, "7 0 obj\n<< /Type /XRef /Size 8 /W [1 4 1] /Index [0 8] /Root 1 0 R /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(entries), entries)
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return buf.Bytes()
}

func prevXRefPDF() []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offs := map[int]int{}
	write := func(id int, body string) {
		offs[id] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}
	content := "BT /F1 12 Tf 72 700 Td (Hello) Tj ET"
	write(1, "<< /Type /Catalog /Pages 2 0 R >>")
	write(2, "<< /Type /Pages /Count 1 /Kids [3 0 R] >>")
	write(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")
	write(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	write(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	old := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 6\n0000000000 65535 f \n")
	for id := 1; id <= 5; id++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offs[id])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size 6 /Root 1 0 R >>\n")
	start := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 4\n0000000000 65535 f \n%010d 00000 n \n%010d 00000 n \n%010d 00000 n \n5 1\n%010d 00000 n \n", offs[1], offs[2], offs[3], offs[5])
	fmt.Fprintf(&buf, "trailer\n<< /Size 6 /Root 1 0 R /Prev %d >>\nstartxref\n%d\n%%%%EOF\n", old, start)
	return buf.Bytes()
}
