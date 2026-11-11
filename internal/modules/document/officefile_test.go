package document

import (
	"bytes"
	"encoding/base64"
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
					{"row":1,"col":1,"input":"你好\n世界","bold":true,"italic":true,"underline":true,"merge_rows":2,"merge_cols":2},
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
