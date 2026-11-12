package document

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
)

func TestPPTImportShowsBackgroundAndFilledPictures(t *testing.T) {
	photo := solidJPEG(t, color.RGBA{R: 220, G: 30, B: 40, A: 255})
	logo := solidPNG(t, color.RGBA{R: 20, G: 90, B: 200, A: 255})
	files := map[string]string{
		"ppt/presentation.xml": officeXMLHeader + `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldSz cx="9144000" cy="6858000"/><p:sldIdLst><p:sldId id="256" r:id="rId1"/><p:sldId id="257" r:id="rId2"/><p:sldId id="258" r:id="rId3"/><p:sldId id="259" r:id="rId4"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>` +
			`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide3.xml"/>` +
			`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide4.xml"/>` +
			`</Relationships>`,
		"ppt/slides/slide1.xml": pptSlideXML(
			`<p:bg><p:bgPr><a:blipFill><a:blip r:embed="rId2"/></a:blipFill><a:effectLst/></p:bgPr></p:bg>`,
			`<p:sp><p:nvSpPr><p:cNvPr id="2" name="标题"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="635000" y="508000"/><a:ext cx="7620000" cy="762000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>封面</a:t></a:r></a:p></p:txBody></p:sp>`,
		),
		"ppt/slides/_rels/slide1.xml.rels": pptRels("rId2", "../media/photo.jpg", ""),
		"ppt/slides/slide2.xml": pptSlideXML("",
			`<p:sp><p:nvSpPr><p:cNvPr id="3" name="圆角"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="1270000" y="2540000"/><a:ext cx="2540000" cy="1270000"/></a:xfrm><a:prstGeom prst="roundRect"><a:avLst/></a:prstGeom><a:blipFill><a:blip r:embed="rId2"/></a:blipFill></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>说明</a:t></a:r></a:p></p:txBody></p:sp>`+
				`<p:grpSp><p:nvGrpSpPr><p:cNvPr id="4" name="组"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1270000" cy="1270000"/><a:chOff x="0" y="0"/><a:chExt cx="2540000" cy="2540000"/></a:xfrm></p:grpSpPr>`+
				`<p:pic><p:nvPicPr><p:cNvPr id="5" name="组内图片"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"/></p:blipFill><p:spPr><a:xfrm><a:off x="254000" y="0"/><a:ext cx="1270000" cy="1270000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic></p:grpSp>`,
		),
		"ppt/slides/_rels/slide2.xml.rels": pptRels("rId2", "../media/photo.jpg", ""),
		"ppt/slides/slide3.xml": pptSlideXML("",
			`<p:pic><p:nvPicPr><p:cNvPr id="6" name="占位图片"/><p:cNvPicPr/><p:nvPr><p:ph type="pic" idx="1"/></p:nvPr></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"/></p:blipFill><p:spPr/></p:pic>`,
		),
		"ppt/slides/_rels/slide3.xml.rels": pptRels("rId2", "../media/photo.jpg", "../slideLayouts/slideLayout1.xml"),
		"ppt/slideLayouts/slideLayout1.xml": officeXMLHeader + `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
			`<p:sp><p:nvSpPr><p:cNvPr id="2" name="图片占位"/><p:cNvSpPr/><p:nvPr><p:ph type="pic" idx="1"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="2540000" y="1270000"/><a:ext cx="5080000" cy="2540000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:sp>` +
			`</p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slides/slide4.xml":                        pptSlideXML("", `<p:sp><p:nvSpPr><p:cNvPr id="2" name="正文"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="635000" y="635000"/><a:ext cx="2540000" cy="635000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>只有文字</a:t></a:r></a:p></p:txBody></p:sp>`),
		"ppt/slides/_rels/slide4.xml.rels":             pptRels("", "", "../slideLayouts/slideLayout2.xml"),
		"ppt/slideLayouts/slideLayout2.xml":            officeXMLHeader + `<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout2.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml": officeXMLHeader + `<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:bg><p:bgPr><a:blipFill><a:blip r:embed="rId2"/></a:blipFill></p:bgPr></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
			`<p:pic><p:nvPicPr><p:cNvPr id="8" name="标志"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId3"/></p:blipFill><p:spPr><a:xfrm><a:off x="7620000" y="508000"/><a:ext cx="1016000" cy="508000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>` +
			`</p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/photo.jpg"/>` +
			`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/logo.png"/>` +
			`</Relationships>`,
		"ppt/media/photo.jpg": string(photo),
		"ppt/media/logo.png":  string(logo),
	}
	body, err := packZip(files)
	if err != nil {
		t.Fatal(err)
	}
	kind, _, content, err := ImportFile("图片.pptx", body)
	if err != nil || kind != KindPPT {
		t.Fatal(err)
	}
	var deck pptContent
	if err := json.Unmarshal(content, &deck); err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 4 {
		t.Fatalf("slides = %d", len(deck.Slides))
	}

	cover := deck.Slides[0]
	if len(cover.Elements) < 2 || cover.Elements[0].Kind != "image" || cover.Elements[0].X != 0 || cover.Elements[0].Y != 0 || cover.Elements[0].W != 720 || cover.Elements[0].H != 540 {
		t.Fatalf("background = %+v", cover.Elements)
	}
	if !strings.Contains(cover.Elements[1].Text, "封面") || !jpegNear(t, cover.Elements[0].Image, 220, 30, 40) {
		t.Fatalf("cover = %+v", cover.Elements[1])
	}

	filled := deck.Slides[1]
	if len(filled.Elements) != 3 {
		t.Fatalf("filled count = %+v", filled.Elements)
	}
	if filled.Elements[0].Kind != "image" || filled.Elements[0].X != 100 || filled.Elements[0].Y != 200 || filled.Elements[0].W != 200 || filled.Elements[0].H != 100 {
		t.Fatalf("shape picture = %+v", filled.Elements[0])
	}
	if filled.Elements[1].Kind != "text" || !strings.Contains(filled.Elements[1].Text, "说明") {
		t.Fatalf("shape text = %+v", filled.Elements[1])
	}
	if filled.Elements[2].Kind != "image" || filled.Elements[2].X != 10 || filled.Elements[2].Y != 0 || filled.Elements[2].W != 50 || filled.Elements[2].H != 50 {
		t.Fatalf("grouped picture = %+v", filled.Elements[2])
	}

	placed := deck.Slides[2]
	if len(placed.Elements) != 3 || placed.Elements[2].Kind != "image" || placed.Elements[2].X != 200 || placed.Elements[2].Y != 100 || placed.Elements[2].W != 400 || placed.Elements[2].H != 200 {
		t.Fatalf("placeholder picture = %+v", placed.Elements)
	}

	inherited := deck.Slides[3]
	if inherited.Background != "#ffffff" || len(inherited.Elements) != 3 {
		t.Fatalf("inherited = bg %s %+v", inherited.Background, inherited.Elements)
	}
	if inherited.Elements[0].Kind != "image" || inherited.Elements[0].W != 720 || inherited.Elements[0].H != 540 || !jpegNear(t, inherited.Elements[0].Image, 220, 30, 40) {
		t.Fatalf("master background = %+v", inherited.Elements[0])
	}
	if inherited.Elements[1].Kind != "image" || inherited.Elements[1].X != 600 || inherited.Elements[1].Y != 40 || inherited.Elements[1].W != 80 || inherited.Elements[1].H != 40 || !jpegNear(t, inherited.Elements[1].Image, 20, 90, 200) {
		t.Fatalf("logo = %+v", inherited.Elements[1])
	}
	if !strings.Contains(inherited.Elements[2].Text, "只有文字") {
		t.Fatalf("slide text = %+v", inherited.Elements[2])
	}
}

func TestPPTSolidBackgroundDoesNotInheritMasterPicture(t *testing.T) {
	photo := solidJPEG(t, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	files := map[string]string{
		"ppt/presentation.xml":                         officeXMLHeader + `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slides/slide1.xml":                        pptSlideXML(`<p:bg><p:bgPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></p:bgPr></p:bg>`, `<p:sp><p:nvSpPr><p:cNvPr id="2" name="标题"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="635000" y="635000"/><a:ext cx="2540000" cy="635000"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>红底</a:t></a:r></a:p></p:txBody></p:sp>`),
		"ppt/slides/_rels/slide1.xml.rels":             pptRels("", "", "../slideLayouts/slideLayout1.xml"),
		"ppt/slideLayouts/slideLayout1.xml":            officeXMLHeader + `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            officeXMLHeader + `<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:bg><p:bgPr><a:blipFill><a:blip r:embed="rId2"/></a:blipFill></p:bgPr></p:bg><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": officeXMLHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/photo.jpg"/></Relationships>`,
		"ppt/media/photo.jpg":                          string(photo),
	}
	body, err := packZip(files)
	if err != nil {
		t.Fatal(err)
	}
	_, _, content, err := ImportFile("红底.pptx", body)
	if err != nil {
		t.Fatal(err)
	}
	var deck pptContent
	if err := json.Unmarshal(content, &deck); err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 1 || deck.Slides[0].Background != "#ff0000" {
		t.Fatalf("background = %+v", deck.Slides)
	}
	for _, element := range deck.Slides[0].Elements {
		if element.Kind == "image" {
			t.Fatalf("solid slide inherited a picture: %+v", element)
		}
	}
}

func TestPPTImportDecodesBMP(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 12, G: 140, B: 70, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	body, err := packZip(map[string]string{
		"ppt/presentation.xml":             officeXMLHeader + `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
		"ppt/slides/slide1.xml":            pptSlideXML("", `<p:pic><p:nvPicPr><p:cNvPr id="2" name="BMP"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"/></p:blipFill><p:spPr><a:xfrm><a:off x="1270000" y="1270000"/><a:ext cx="2540000" cy="2540000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`),
		"ppt/slides/_rels/slide1.xml.rels": pptRels("rId2", "../media/image1.bmp", ""),
		"ppt/media/image1.bmp":             buf.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, content, err := ImportFile("位图.pptx", body)
	if err != nil {
		t.Fatal(err)
	}
	var deck pptContent
	if err := json.Unmarshal(content, &deck); err != nil {
		t.Fatal(err)
	}
	if len(deck.Slides) != 1 || len(deck.Slides[0].Elements) != 1 || deck.Slides[0].Elements[0].Kind != "image" || !jpegNear(t, deck.Slides[0].Elements[0].Image, 12, 140, 70) {
		t.Fatalf("bmp = %+v", deck.Slides)
	}
}

func pptSlideXML(background, shapes string) string {
	return officeXMLHeader + `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld>` +
		background + `<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
		shapes + `</p:spTree></p:cSld></p:sld>`
}

func pptRels(imageID, imageTarget, layoutTarget string) string {
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	if layoutTarget != "" {
		body.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="` + layoutTarget + `"/>`)
	}
	if imageID != "" {
		body.WriteString(`<Relationship Id="` + imageID + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="` + imageTarget + `"/>`)
	}
	body.WriteString(`</Relationships>`)
	return body.String()
}

func jpegNear(t *testing.T, dataURL string, red, green, blue uint8) bool {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, jpegDataPrefix))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	pixel := color.RGBAModel.Convert(img.At(img.Bounds().Dx()/2, img.Bounds().Dy()/2)).(color.RGBA)
	return absInt(int(pixel.R)-int(red)) < 30 && absInt(int(pixel.G)-int(green)) < 30 && absInt(int(pixel.B)-int(blue)) < 30
}
