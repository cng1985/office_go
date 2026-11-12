package document

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestOfficeRoundTrip(t *testing.T) {
	cases := []struct {
		kind  string
		title string
		raw   string
	}{
		{
			kind:  KindWord,
			title: "纪要",
			raw: `{"blocks":[
				{"id":"b1","type":"heading1","align":"center","runs":[{"text":"标题","bold":true,"italic":false,"underline":false,"size":24}]},
				{"id":"b2","type":"paragraph","align":"justify","indent":1,"runs":[{"text":"甲","bold":false,"italic":true,"underline":true,"strike":true,"color":"#112233","highlight":"#ffeeaa","size":18,"font":"serif"}]},
				{"id":"b3","type":"bullet","align":"left","runs":[{"text":"要点","bold":false,"italic":false,"underline":false}]},
				{"id":"b4","type":"numbered","align":"right","runs":[{"text":"步骤","bold":false,"italic":false,"underline":false}]},
				{"id":"b5","type":"quote","align":"left","runs":[{"text":"引用","bold":false,"italic":false,"underline":false,"font":"mono"}]},
				{"id":"b6","type":"break","align":"left","runs":[{"text":"","bold":false,"italic":false,"underline":false}]}
			]}`,
		},
		{
			kind:  KindExcel,
			title: "预算",
			raw: `{"active":1,"sheets":[
				{"name":"工作表1","row_count":50,"col_count":26,"cells":[
					{"row":1,"col":1,"input":"你好\n世界","bold":true,"italic":true,"underline":true,"format":"text","merge_rows":2,"merge_cols":2},
					{"row":2,"col":3,"input":"=SUM(A1)"}
				]},
				{"name":"明细","row_count":50,"col_count":26,"cells":[]}
			]}`,
		},
		{
			kind:  KindPPT,
			title: "发布会",
			raw: `{"slides":[
				{"id":"s1","background":"#fffdf8","notes":"第一行\n第二行","elements":[
					{"id":"e1","kind":"text","x":68,"y":40,"w":824,"h":72,"text":"标题\n副标题","font_size":32,"color":"#202124","bold":true,"align":"center","font":"serif"},
					{"id":"e2","kind":"shape","shape":"ellipse","x":80,"y":160,"w":240,"h":140,"text":"形状","font_size":18,"color":"#112233","bold":false,"align":"left","fill":"#aabbcc","italic":true}
				]},
				{"id":"s2","background":"#111111","elements":[]}
			]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			canonical, err := Canonical(tc.kind, []byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			file, err := ExportFile(tc.kind, tc.title, canonical)
			if err != nil {
				t.Fatal(err)
			}
			if file.Filename != tc.title+extOf(tc.kind) || len(file.Body) < 4 || string(file.Body[:2]) != "PK" {
				t.Fatalf("file = %+v", file.Filename)
			}
			kind, title, got, err := ImportFile("../../"+file.Filename, file.Body)
			if err != nil {
				t.Fatal(err)
			}
			if kind != tc.kind || title != tc.title || string(got) != string(canonical) {
				t.Fatalf("imported kind=%s title=%s\nwant %s\ngot  %s", kind, title, canonical, got)
			}
		})
	}
}

func TestPlainAndJSONImport(t *testing.T) {
	kind, title, content, err := ImportFile("记录.TXT", []byte("第一行\r\n\r\n第二行\r\n"))
	if err != nil || kind != KindWord || title != "记录" || !bytes.Contains(content, []byte("第一行")) || !bytes.Contains(content, []byte("第二行")) {
		t.Fatalf("txt = %s %s %s %v", kind, title, content, err)
	}
	kind, title, content, err = ImportFile("库存.csv", []byte("名称,数量\n苹果,2\n"))
	if err != nil || kind != KindExcel || title != "库存" || !bytes.Contains(content, []byte("苹果")) || !bytes.Contains(content, []byte(`"row":2`)) {
		t.Fatalf("csv = %s %s %s %v", kind, title, content, err)
	}
	kind, _, content, err = ImportFile("欧洲.csv", []byte("名称;数量\n苹果;2\n"))
	if err != nil || !bytes.Contains(content, []byte("苹果")) || !bytes.Contains(content, []byte(`"col":2`)) {
		t.Fatalf("semicolon = %s %v", content, err)
	}
	envelope := `{"kind":"word","title":"信封","content":{"blocks":[{"id":"b1","type":"paragraph","align":"left","runs":[{"text":"正文","bold":false,"italic":false,"underline":false}]}]}}`
	kind, title, content, err = ImportFile("忽略.json", []byte(envelope))
	if err != nil || kind != KindWord || title != "信封" || !bytes.Contains(content, []byte("正文")) {
		t.Fatalf("json = %s %s %s %v", kind, title, content, err)
	}
	if _, _, _, err = ImportFile("旧版.doc", []byte("abc")); err == nil {
		t.Fatal("legacy office files should be rejected")
	}
	if _, _, _, err = ImportFile("空.txt", []byte("   ")); err == nil {
		t.Fatal("blank file should be rejected")
	}
}

func TestSharedStringWorkbook(t *testing.T) {
	body, err := packZip(map[string]string{
		"_rels/.rels":                officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            officeXMLHeader + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="水果" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       officeXMLHeader + `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>苹果</t></si></sst>`,
		"xl/worksheets/sheet1.xml":   officeXMLHeader + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="B2" t="s"><v>0</v></c></row></sheetData></worksheet>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	kind, title, content, err := ImportFile("水果.xlsx", body)
	if err != nil || kind != KindExcel || title != "水果" || !bytes.Contains(content, []byte("苹果")) || !bytes.Contains(content, []byte(`"row":2`)) || !bytes.Contains(content, []byte(`"col":2`)) {
		t.Fatalf("shared = %s %s %s %v", kind, title, content, err)
	}
}

func TestImportExportThroughResource(t *testing.T) {
	db := openDB(t)
	owner := scopeOf(t, db, "uploader")
	other := scopeOf(t, db, "outsider")
	resource := NewResource(NewRepository(db))
	imported, err := resource.Import(owner, &DocumentImportRequest{
		Filename:      `..\会议纪要.txt`,
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("第一行\n第二行\n")),
	})
	if err != nil || imported.Data.Kind != KindWord || imported.Data.Title != "会议纪要" {
		t.Fatalf("import = %+v err=%v", imported.Data, err)
	}
	versions, err := resource.VersionSearch(owner, &DocumentVersionSearch{DocumentID: imported.Data.ID})
	if err != nil || versions.Total != 1 || versions.Data[0].Note != "上传文件" {
		t.Fatalf("versions = %+v err=%v", versions, err)
	}
	if _, err := resource.Export(other, &DocumentExportRequest{ID: imported.Data.ID}); err == nil {
		t.Fatal("other user should not export the file")
	}
	exported, err := resource.Export(owner, &DocumentExportRequest{ID: imported.Data.ID})
	if err != nil || !strings.HasSuffix(exported.Data.Filename, ".docx") {
		t.Fatalf("export = %+v err=%v", exported.Data.Filename, err)
	}
	raw, err := base64.StdEncoding.DecodeString(exported.Data.ContentBase64)
	if err != nil {
		t.Fatal(err)
	}
	kind, title, content, err := ImportFile(exported.Data.Filename, raw)
	if err != nil || kind != KindWord || title != "会议纪要" || !bytes.Contains(content, []byte("第一行")) || !bytes.Contains(content, []byte("第二行")) {
		t.Fatalf("reimport = %s %s %s %v", kind, title, content, err)
	}
	if _, err := resource.Import(owner, &DocumentImportRequest{Filename: "病毒.exe", ContentBase64: base64.StdEncoding.EncodeToString([]byte("abc"))}); err == nil {
		t.Fatal("unknown extension should be rejected")
	}
}

func TestNumericCellsAndWordStyles(t *testing.T) {
	canonical, err := Canonical(KindExcel, []byte(`{"sheets":[{"name":"工作表1","row_count":50,"col_count":26,"cells":[
		{"row":1,"col":1,"input":"10"},
		{"row":2,"col":1,"input":"20"},
		{"row":3,"col":1,"input":"=SUM(A1:A2)"},
		{"row":4,"col":1,"input":"001","format":"text"},
		{"row":5,"col":1,"input":"001"}
	]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	file, err := ExportFile(KindExcel, "合计", canonical)
	if err != nil {
		t.Fatal(err)
	}
	files, err := readZip(file.Body)
	if err != nil {
		t.Fatal(err)
	}
	sheet := string(files["xl/worksheets/sheet1.xml"])
	for _, want := range []string{`<c r="A1"><v>10</v></c>`, `<c r="A2"><v>20</v></c>`, `<f>SUM(A1:A2)</f>`, `>001</t>`} {
		if !strings.Contains(sheet, want) {
			t.Fatalf("sheet missing %s\n%s", want, sheet)
		}
	}
	if strings.Contains(sheet, `r="A1" t="inlineStr"`) || !strings.Contains(sheet, `<c r="A5"><v>1</v></c>`) {
		t.Fatalf("numeric cells = %s", sheet)
	}
	_, _, got, err := ImportFile(file.Filename, file.Body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"input":"10"`, `"input":"20"`, `"input":"=SUM(A1:A2)"`, `"input":"001"`, `"input":"1"`} {
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("imported %s missing %s", got, want)
		}
	}
	if bytes.Count(got, []byte(`"format":"text"`)) != 1 {
		t.Fatalf("text format was not preserved: %s", got)
	}

	word, err := Canonical(KindWord, []byte(`{"blocks":[
		{"id":"b1","type":"heading1","align":"left","runs":[{"text":"标题","bold":false,"italic":false,"underline":false}]},
		{"id":"b2","type":"bullet","align":"left","runs":[{"text":"要点","bold":false,"italic":false,"underline":false}]},
		{"id":"b3","type":"numbered","align":"left","runs":[{"text":"步骤","bold":false,"italic":false,"underline":false}]}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExportFile(KindWord, "纪要", word)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := readZip(doc.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(parts["word/styles.xml"], []byte(`w:styleId="Heading1"`)) || !bytes.Contains(parts["word/styles.xml"], []byte(`w:styleId="ListBullet"`)) {
		t.Fatal("missing paragraph styles")
	}
	if !bytes.Contains(parts["word/numbering.xml"], []byte(`w:numFmt w:val="bullet"`)) || !bytes.Contains(parts["word/numbering.xml"], []byte(`w:numFmt w:val="decimal"`)) {
		t.Fatal("missing list numbering")
	}
	if !bytes.Contains(parts["word/_rels/document.xml.rels"], []byte("numbering.xml")) || !bytes.Contains(parts["word/_rels/document.xml.rels"], []byte("styles.xml")) {
		t.Fatal("document relationships do not include styles")
	}
	documentXML := string(parts["word/document.xml"])
	if !strings.Contains(documentXML, `<w:numId w:val="1"/>`) || !strings.Contains(documentXML, `<w:numId w:val="2"/>`) {
		t.Fatalf("paragraph numbering = %s", documentXML)
	}
}

func TestSharedFormulaImport(t *testing.T) {
	body, err := packZip(map[string]string{
		"_rels/.rels":                officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            officeXMLHeader + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="公式" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": officeXMLHeader + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
			`<row r="3"><c r="A3"><f t="shared" si="0"/><v>99</v></c><c r="C3"><f t="shared" si="1"/><v>5</v></c></row>` +
			`<row r="2"><c r="A2"><f t="shared" ref="A2:A3" si="0">A1+1</f><v>2</v></c><c r="B2"><f t="shared" ref="B2:C3" si="1">$A2+B1</f><v>4</v></c></row>` +
			`</sheetData></worksheet>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, content, err := ImportFile("公式.xlsx", body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"input":"=A1+1"`, `"input":"=A2+1"`, `"input":"=$A2+B1"`, `"input":"=$A3+C2"`} {
		if !bytes.Contains(content, []byte(want)) {
			t.Fatalf("shared formulas = %s missing %s", content, want)
		}
	}
	if bytes.Contains(content, []byte(`"99"`)) || bytes.Contains(content, []byte(`"input":"5"`)) {
		t.Fatalf("cached shared values leaked: %s", content)
	}
	broken, err := packZip(map[string]string{
		"xl/workbook.xml":            officeXMLHeader + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="公式" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   officeXMLHeader + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><f t="shared" si="4"/><v>1</v></c></row></sheetData></worksheet>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = ImportFile("坏公式.xlsx", broken); err == nil {
		t.Fatal("missing shared formula anchor should fail")
	}
}

func TestTextNumbersStayText(t *testing.T) {
	body, err := packZip(map[string]string{
		"_rels/.rels":                officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            officeXMLHeader + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="编号" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       officeXMLHeader + `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>001</t></si><si><t>10</t></si></sst>`,
		"xl/worksheets/sheet1.xml": officeXMLHeader + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
			`<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>` +
			`<row r="2"><c r="A2" t="inlineStr"><is><t>001</t></is></c><c r="A3"><v>10</v></c></row>` +
			`</sheetData></worksheet>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, content, err := ImportFile("编号.xlsx", body)
	if err != nil {
		t.Fatal(err)
	}
	var book excelContent
	if err := json.Unmarshal(content, &book); err != nil {
		t.Fatal(err)
	}
	if len(book.Sheets) != 1 {
		t.Fatalf("sheets = %d", len(book.Sheets))
	}
	got := map[[2]int]excelCell{}
	for _, cell := range book.Sheets[0].Cells {
		got[[2]int{cell.Row, cell.Col}] = cell
	}
	checks := []struct {
		row, col int
		input    string
		format   string
	}{
		{1, 1, "001", "text"},
		{1, 2, "10", "text"},
		{2, 1, "001", "text"},
		{3, 1, "10", ""},
	}
	for _, check := range checks {
		cell, ok := got[[2]int{check.row, check.col}]
		if !ok || cell.Input != check.input || cell.Format != check.format {
			t.Fatalf("cell %d,%d = %+v", check.row, check.col, cell)
		}
	}
	file, err := ExportFile(KindExcel, "编号", content)
	if err != nil {
		t.Fatal(err)
	}
	files, err := readZip(file.Body)
	if err != nil {
		t.Fatal(err)
	}
	sheet := string(files["xl/worksheets/sheet1.xml"])
	for _, want := range []string{
		`<c r="A1" t="inlineStr"><is><t xml:space="preserve">001</t></is></c>`,
		`<c r="B1" t="inlineStr"><is><t xml:space="preserve">10</t></is></c>`,
		`<c r="A2" t="inlineStr"><is><t xml:space="preserve">001</t></is></c>`,
		`<c r="A3"><v>10</v></c>`,
	} {
		if !strings.Contains(sheet, want) {
			t.Fatalf("sheet missing %s\n%s", want, sheet)
		}
	}
	if strings.Contains(sheet, `<v>1</v>`) {
		t.Fatalf("leading zero was dropped: %s", sheet)
	}
}

func TestWholeRowAndColumnSharedFormulas(t *testing.T) {
	body, err := packZip(map[string]string{
		"_rels/.rels":                officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            officeXMLHeader + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="公式" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": officeXMLHeader + `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
			`<row r="1"><c r="A1"><f t="shared" ref="A1:B2" si="0">SUM(A:A)+SUM(1:1)+SUM($A:$A)+SUM($1:2)</f><v>0</v></c><c r="B1"><f t="shared" si="0"/><v>0</v></c></row>` +
			`<row r="2"><c r="A2"><f t="shared" si="0"/><v>0</v></c><c r="B2"><f t="shared" si="0"/><v>0</v></c></row>` +
			`</sheetData></worksheet>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, content, err := ImportFile("整列.xlsx", body)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"input":"=SUM(A:A)+SUM(1:1)+SUM($A:$A)+SUM($1:2)"`,
		`"input":"=SUM(B:B)+SUM(1:1)+SUM($A:$A)+SUM($1:2)"`,
		`"input":"=SUM(A:A)+SUM(2:2)+SUM($A:$A)+SUM($1:3)"`,
		`"input":"=SUM(B:B)+SUM(2:2)+SUM($A:$A)+SUM($1:3)"`,
	} {
		if !bytes.Contains(content, []byte(want)) {
			t.Fatalf("shared ranges = %s missing %s", content, want)
		}
	}
}

func TestSeparateNumberedListsRestart(t *testing.T) {
	word, err := Canonical(KindWord, []byte(`{"blocks":[
		{"id":"b1","type":"numbered","align":"left","runs":[{"text":"甲","bold":false,"italic":false,"underline":false}]},
		{"id":"b2","type":"bullet","align":"left","runs":[{"text":"点","bold":false,"italic":false,"underline":false}]},
		{"id":"b3","type":"numbered","align":"left","runs":[{"text":"乙","bold":false,"italic":false,"underline":false}]},
		{"id":"b4","type":"numbered","align":"left","runs":[{"text":"丙","bold":false,"italic":false,"underline":false}]},
		{"id":"b5","type":"paragraph","align":"left","runs":[{"text":"间隔","bold":false,"italic":false,"underline":false}]},
		{"id":"b6","type":"numbered","align":"left","runs":[{"text":"丁","bold":false,"italic":false,"underline":false}]}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExportFile(KindWord, "清单", word)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := readZip(doc.Body)
	if err != nil {
		t.Fatal(err)
	}
	documentXML := string(parts["word/document.xml"])
	if strings.Count(documentXML, `<w:numId w:val="1"/>`) != 1 ||
		strings.Count(documentXML, `<w:numId w:val="2"/>`) != 1 ||
		strings.Count(documentXML, `<w:numId w:val="3"/>`) != 2 ||
		strings.Count(documentXML, `<w:numId w:val="4"/>`) != 1 {
		t.Fatalf("list instances = %s", documentXML)
	}
	numbering := string(parts["word/numbering.xml"])
	if strings.Count(numbering, `<w:startOverride w:val="1"/>`) != 3 {
		t.Fatalf("numbering = %s", numbering)
	}
}

func TestTallExcelImports(t *testing.T) {
	kind, _, content, err := ImportFile("宽表.xlsx", excelGridFile(t, maxExcelRows, maxExcelCols))
	if err != nil || kind != KindExcel || !bytes.Contains(content, []byte("末格")) || !bytes.Contains(content, []byte(fmt.Sprintf(`"row":%d`, maxExcelRows))) {
		t.Fatalf("import wide = %s %v", content, err)
	}
	if _, _, _, err = ImportFile("过高.xlsx", excelGridFile(t, maxExcelRows+1, 1)); err == nil || !strings.Contains(err.Error(), "表格超过") {
		t.Fatal(err)
	}
	if _, _, _, err = ImportFile("过宽.xlsx", excelGridFile(t, 1, maxExcelCols+1)); err == nil || !strings.Contains(err.Error(), "表格超过") {
		t.Fatal(err)
	}
	body := bytes.Repeat([]byte("甲,乙\n"), 121)
	kind, _, content, err = ImportFile("名单.csv", body)
	if err != nil || kind != KindExcel || !bytes.Contains(content, []byte(`"row":121`)) {
		t.Fatalf("csv = %s %v", content, err)
	}
}

func excelGridFile(t *testing.T, rows, cols int) []byte {
	t.Helper()
	var sheet strings.Builder
	sheet.WriteString(officeXMLHeader)
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for _, row := range []int{1, rows} {
		if row < 1 {
			continue
		}
		fmt.Fprintf(&sheet, `<row r="%d"><c r="%s%d" t="inlineStr"><is><t>末格</t></is></c></row>`, row, colName(cols), row)
		if rows == 1 {
			break
		}
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	raw, err := packZip(map[string]string{
		"xl/workbook.xml":          officeXMLHeader + `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"></workbook>`,
		"xl/worksheets/sheet1.xml": sheet.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestLongWordDocumentImports(t *testing.T) {
	file := wordParagraphs(t, 401)
	kind, _, content, err := ImportFile("长文.docx", file)
	if err != nil || kind != KindWord || !bytes.Contains(content, []byte("段401")) {
		t.Fatalf("import 401 = %s %v", content, err)
	}
	if _, _, _, err = ImportFile("过长.docx", wordParagraphs(t, maxWordBlocks+1)); err == nil || !strings.Contains(err.Error(), "文稿段落过多") {
		t.Fatal(err)
	}
}

func TestWordImagesImportAndRoundTrip(t *testing.T) {
	pngBytes := solidPNG(t, color.RGBA{R: 220, G: 30, B: 30, A: 255})
	jpegBytes := solidJPEG(t, color.RGBA{R: 20, G: 80, B: 200, A: 255})
	document := officeXMLHeader + `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><w:body>` +
		`<w:p><w:r><w:t>前文</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:t>图前</w:t></w:r><w:r><w:drawing><a:blip r:embed="rId1"/><a:p><a:r><a:t>形状里</a:t></a:r></a:p></w:drawing></w:r><w:r><w:t>图后</w:t></w:r></w:p>` +
		`<w:p><w:r><mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><a:blip r:embed="rId2"/></w:drawing></mc:Choice><mc:Fallback><w:pict><v:imagedata r:id="rId2"/></w:pict></mc:Fallback></mc:AlternateContent></w:r></w:p>` +
		`<w:p><w:r><w:t>结尾</w:t></w:r></w:p>` +
		`</w:body></w:document>`
	rels := officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image2.jpg"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>` +
		`</Relationships>`
	header := officeXMLHeader + `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><w:p><w:r><w:drawing><a:blip r:embed="rId1"/></w:drawing></w:r></w:p></w:hdr>`
	headerRels := officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`
	body, err := packZip(map[string]string{
		"word/document.xml":            document,
		"word/_rels/document.xml.rels": rels,
		"word/header1.xml":             header,
		"word/_rels/header1.xml.rels":  headerRels,
		"word/media/image1.png":        string(pngBytes),
		"word/media/image2.jpg":        string(jpegBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	kind, _, content, err := ImportFile("带图.docx", body)
	if err != nil || kind != KindWord {
		t.Fatal(err)
	}
	var doc wordContent
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(doc.Blocks))
	images := 0
	for _, block := range doc.Blocks {
		if block.Type == "image" {
			images++
			if !strings.HasPrefix(block.Image, jpegDataPrefix) {
				t.Fatalf("image block = %#v", block)
			}
			if images == 2 && block.Align != "center" {
				t.Fatalf("image align = %s", block.Align)
			}
			got = append(got, "image")
			continue
		}
		got = append(got, plainWord(block))
	}
	want := []string{"image", "前文", "图前", "image", "图后", "image", "结尾"}
	if strings.Join(got, "|") != strings.Join(want, "|") || images != 3 || bytes.Contains(content, []byte("形状里")) {
		t.Fatalf("blocks = %s", strings.Join(got, "|"))
	}
	exported, err := ExportFile(KindWord, "带图", content)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := readZip(exported.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(parts["word/document.xml"], []byte(`r:embed="rId3"`)) || len(parts["word/media/image1.jpeg"]) == 0 || len(parts["word/media/image3.jpeg"]) == 0 {
		t.Fatalf("exported images missing: %s", parts["word/document.xml"])
	}
	_, _, again, err := ImportFile("带图.docx", exported.Body)
	if err != nil || bytes.Count(again, []byte(`"type":"image"`)) != 3 || !bytes.Contains(again, []byte("前文")) || !bytes.Contains(again, []byte("图后")) {
		t.Fatalf("round trip = %s %v", again, err)
	}

	missing, err := packZip(map[string]string{
		"word/document.xml":            officeXMLHeader + `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body><w:p><w:r><w:t>还有文字</w:t></w:r><w:r><w:drawing><a:blip r:embed="rId9"/></w:drawing></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/gone.png"/></Relationships>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, textOnly, err := ImportFile("缺图.docx", missing)
	if err != nil || !bytes.Contains(textOnly, []byte("还有文字")) || bytes.Contains(textOnly, []byte(`"type":"image"`)) {
		t.Fatalf("missing media = %s %v", textOnly, err)
	}
}

func solidPNG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func solidJPEG(t *testing.T, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func plainWord(block wordBlock) string {
	var b strings.Builder
	for _, run := range block.Runs {
		b.WriteString(run.Text)
	}
	return b.String()
}

func TestWordImageKeepsTextOrderAndTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 6; y < 10; y++ {
		for x := 6; x < 10; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 12, G: 18, B: 24, A: 255})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	document := officeXMLHeader + `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><w:body><w:p><w:r><w:t>before</w:t><w:drawing><a:blip r:embed="rId1"/></w:drawing><w:t>after</w:t></w:r></w:p></w:body></w:document>`
	rels := officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/mark.png"/></Relationships>`
	body, err := packZip(map[string]string{
		"word/document.xml":            document,
		"word/_rels/document.xml.rels": rels,
		"word/media/mark.png":          pngBuf.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, content, err := ImportFile("透明.docx", body)
	if err != nil {
		t.Fatal(err)
	}
	var doc wordContent
	if err := json.Unmarshal(content, &doc); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(doc.Blocks))
	var imageData string
	for _, block := range doc.Blocks {
		if block.Type == "image" {
			got = append(got, "image")
			imageData = block.Image
			continue
		}
		got = append(got, plainWord(block))
	}
	if strings.Join(got, "|") != "before|image|after" {
		t.Fatalf("order = %s", strings.Join(got, "|"))
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(imageData, jpegDataPrefix))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	corner := color.RGBAModel.Convert(decoded.At(0, 0)).(color.RGBA)
	ink := color.RGBAModel.Convert(decoded.At(8, 8)).(color.RGBA)
	if corner.R < 240 || corner.G < 240 || corner.B < 240 {
		t.Fatalf("transparent pixel = %#v", corner)
	}
	if ink.R > 80 || ink.G > 80 || ink.B > 80 {
		t.Fatalf("ink pixel = %#v", ink)
	}
}

func TestWordFallbackIsNotImported(t *testing.T) {
	document := officeXMLHeader + `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><w:body>` +
		`<w:p><w:r><w:t>可见</w:t></w:r></w:p>` +
		`<mc:AlternateContent><mc:Choice Requires="wps"><w:p><w:r><w:t>现代</w:t></w:r></w:p></mc:Choice>` +
		`<mc:Fallback><w:p><w:r><w:t>旧版</w:t></w:r></w:p></mc:Fallback></mc:AlternateContent>` +
		`</w:body></w:document>`
	body, err := packZip(map[string]string{"word/document.xml": document})
	if err != nil {
		t.Fatal(err)
	}
	_, _, content, err := ImportFile("兼容.docx", body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("可见")) || !bytes.Contains(content, []byte("现代")) || bytes.Contains(content, []byte("旧版")) {
		t.Fatalf("fallback leaked: %s", content)
	}
}

func wordParagraphs(t *testing.T, count int) []byte {
	t.Helper()
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&body, `<w:p><w:r><w:t>段%d</w:t></w:r></w:p>`, i)
	}
	body.WriteString(`</w:body></w:document>`)
	raw, err := packZip(map[string]string{"word/document.xml": body.String()})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func extOf(kind string) string {
	switch kind {
	case KindWord:
		return ".docx"
	case KindExcel:
		return ".xlsx"
	default:
		return ".pptx"
	}
}

func TestZipBombIsRejected(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writer, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	chunk := bytes.Repeat([]byte{0}, 1<<20)
	for i := 0; i < 9; i++ {
		if _, err := writer.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() >= 1<<20 {
		t.Fatalf("fixture did not compress: %d", buf.Len())
	}
	if _, err := readZip(buf.Bytes()); err == nil || !strings.Contains(err.Error(), "文件过大或内容过多") {
		t.Fatal(err)
	}
	var packed bytes.Buffer
	z, err := zlib.NewWriterLevel(&packed, zlib.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := z.Write(bytes.Repeat([]byte{0}, 9<<20)); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := inflatePDF(packed.Bytes()); !errors.Is(err, errPDFLimit) {
		t.Fatal(err)
	}
}

func TestLargeUploadIsNotRejectedForSize(t *testing.T) {
	raw := bytes.Repeat([]byte("A"), (1<<20)+128)
	got, err := decodeUpload(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("decoded %d bytes", len(got))
	}
	body := largeDocx(t, "大文件正文")
	if len(body) <= 1<<20 {
		t.Fatalf("fixture = %d", len(body))
	}
	kind, title, content, err := ImportFile("大文件.docx", body)
	if err != nil || kind != KindWord || title != "大文件" || !bytes.Contains(content, []byte("大文件正文")) {
		t.Fatalf("large docx = %s %s %s %v", kind, title, content, err)
	}
}

func largeDocx(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body></w:document>`,
		"word/pad.bin":      strings.Repeat("x", 1<<20+128),
	}
	for name, content := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
