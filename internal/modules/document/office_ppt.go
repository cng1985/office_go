package document

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	apperrors "yunjian/internal/platform/errors"
)

const pptRootRels = officeXMLHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`

const pptMaster = officeXMLHeader + `
<p:sldMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>
  <p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>
  <p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>
</p:sldMaster>`

const pptMasterRels = officeXMLHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="../theme/theme1.xml"/>
</Relationships>`

const pptLayout = officeXMLHeader + `
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="blank" preserve="1">
  <p:cSld name="空白"><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr></p:spTree></p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`

const pptLayoutRels = officeXMLHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/>
</Relationships>`

const pptSlideRels = officeXMLHeader + `
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`

const pptTheme = officeXMLHeader + `
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="云笺">
  <a:themeElements>
    <a:clrScheme name="云笺">
      <a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>
      <a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="1F4E79"/></a:dk2>
      <a:lt2><a:srgbClr val="EEECE1"/></a:lt2>
      <a:accent1><a:srgbClr val="1A73E8"/></a:accent1>
      <a:accent2><a:srgbClr val="188038"/></a:accent2>
      <a:accent3><a:srgbClr val="FBBC04"/></a:accent3>
      <a:accent4><a:srgbClr val="E37400"/></a:accent4>
      <a:accent5><a:srgbClr val="A142F4"/></a:accent5>
      <a:accent6><a:srgbClr val="12B5CB"/></a:accent6>
      <a:hlink><a:srgbClr val="1A73E8"/></a:hlink>
      <a:folHlink><a:srgbClr val="7B1FA2"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="云笺">
      <a:majorFont><a:latin typeface="Arial"/><a:ea typeface="微软雅黑"/><a:cs typeface=""/></a:majorFont>
      <a:minorFont><a:latin typeface="Arial"/><a:ea typeface="微软雅黑"/><a:cs typeface=""/></a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="云笺">
      <a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>
      <a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>
      <a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>
      <a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>
    </a:fmtScheme>
  </a:themeElements>
  <a:objectDefaults/>
  <a:extraClrSchemeLst/>
</a:theme>`

var slidePartPattern = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

func exportPptx(canonical []byte) ([]byte, error) {
	var content pptContent
	if err := json.Unmarshal(canonical, &content); err != nil {
		return nil, apperrors.WrapInternal(err)
	}
	files := map[string]string{
		"_rels/.rels":                                  pptRootRels,
		"ppt/slideMasters/slideMaster1.xml":            pptMaster,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": pptMasterRels,
		"ppt/slideLayouts/slideLayout1.xml":            pptLayout,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": pptLayoutRels,
		"ppt/theme/theme1.xml":                         pptTheme,
	}
	var rels, slideIDs, overrides strings.Builder
	rels.WriteString(officeXMLHeader)
	rels.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	rels.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	imageCount := 0
	for i, slide := range content.Slides {
		xml, images := slideXML(slide)
		files[fmt.Sprintf("ppt/slides/slide%d.xml", i+1)] = xml
		files[fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1)] = pptSlideRelsXML(i+1, len(images))
		for n, data := range images {
			files[fmt.Sprintf("ppt/media/s%di%d.jpeg", i+1, n+1)] = string(data)
			imageCount++
		}
		rid := fmt.Sprintf("rId%d", i+2)
		fmt.Fprintf(&rels, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, rid, i+1)
		fmt.Fprintf(&slideIDs, `<p:sldId id="%d" r:id="%s"/>`, 256+i, rid)
		fmt.Fprintf(&overrides, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i+1)
	}
	rels.WriteString(`</Relationships>`)
	files["ppt/_rels/presentation.xml.rels"] = rels.String()
	files["ppt/presentation.xml"] = officeXMLHeader + fmt.Sprintf(`
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
  <p:sldIdLst>%s</p:sldIdLst>
  <p:sldSz cx="12192000" cy="6858000" type="screen16x9"/>
  <p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`, slideIDs.String())
	jpegDefault := ""
	if imageCount > 0 {
		jpegDefault = `<Default Extension="jpeg" ContentType="image/jpeg"/>`
	}
	files["[Content_Types].xml"] = officeXMLHeader + `
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>` + jpegDefault + `
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
  <Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>
  <Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` + overrides.String() + `</Types>`
	return packZip(files)
}

func pptSlideRelsXML(slide, imageCount int) string {
	if imageCount == 0 {
		return pptSlideRels
	}
	var body strings.Builder
	body.WriteString(officeXMLHeader)
	body.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	body.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>`)
	for i := 1; i <= imageCount; i++ {
		fmt.Fprintf(&body, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/s%di%d.jpeg"/>`, i+1, slide, i)
	}
	body.WriteString(`</Relationships>`)
	return body.String()
}

func slideXML(slide pptSlide) (string, [][]byte) {
	var body strings.Builder
	var images [][]byte
	body.WriteString(officeXMLHeader)
	body.WriteString(`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld>`)
	fmt.Fprintf(&body, `<p:bg><p:bgPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:effectLst/></p:bgPr></p:bg>`, strings.ToUpper(strings.TrimPrefix(slide.Background, "#")))
	body.WriteString(`<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	next := 2
	for _, element := range slide.Elements {
		if element.Kind == "image" {
			raw, ok := jpegPayload(element.Image)
			if !ok {
				continue
			}
			images = append(images, raw)
			writePPTPicture(&body, element, next, len(images)+1)
			next++
			continue
		}
		name := "云笺文本"
		if element.Kind == "shape" {
			name = "云笺形状"
		}
		writePPTShape(&body, element, next, name)
		next++
	}
	if strings.TrimSpace(slide.Notes) != "" {
		writePPTShape(&body, pptElement{
			Kind: "text", X: 40, Y: 800, W: 400, H: 40,
			Text: slide.Notes, FontSize: 12, Color: "#202124", Align: "left",
		}, next, "云笺备注")
	}
	body.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
	return body.String(), images
}

func writePPTPicture(body *strings.Builder, element pptElement, id, rel int) {
	x, y, w, h := containedPicture(element)
	fmt.Fprintf(body, `<p:pic><p:nvPicPr><p:cNvPr id="%d" name="云笺图片"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>`, id)
	fmt.Fprintf(body, `<p:blipFill><a:blip r:embed="rId%d"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>`, rel)
	fmt.Fprintf(body, `<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
		pxToEmu(x), pxToEmu(y), pxToEmu(w), pxToEmu(h))
}

func containedPicture(element pptElement) (x, y, w, h float64) {
	x, y, w, h = element.X, element.Y, element.W, element.H
	raw, ok := jpegPayload(element.Image)
	if !ok || w <= 0 || h <= 0 {
		return x, y, w, h
	}
	iw, ih, _, ok := jpegInfo(raw)
	if !ok || iw < 1 || ih < 1 {
		return x, y, w, h
	}
	scale := math.Min(w/float64(iw), h/float64(ih))
	dw := float64(iw) * scale
	dh := float64(ih) * scale
	return x + (w-dw)/2, y + (h-dh)/2, dw, dh
}

func writePPTShape(body *strings.Builder, element pptElement, id int, name string) {
	prst := "rect"
	if element.Kind == "shape" {
		switch element.Shape {
		case "ellipse":
			prst = "ellipse"
		case "round":
			prst = "roundRect"
		}
	}
	fill := `<a:noFill/>`
	if element.Kind == "shape" && element.Fill != "" {
		fill = fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, strings.ToUpper(strings.TrimPrefix(element.Fill, "#")))
	}
	fmt.Fprintf(body, `<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="%s"><a:avLst/></a:prstGeom>%s</p:spPr><p:txBody><a:bodyPr wrap="square"/><a:lstStyle/>`,
		id, xmlEscape(name), pxToEmu(element.X), pxToEmu(element.Y), pxToEmu(element.W), pxToEmu(element.H), prst, fill)
	lines := strings.Split(element.Text, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	align := pptAlignOut(element.Align)
	size := int(math.Round(element.FontSize * 100))
	for _, line := range lines {
		fmt.Fprintf(body, `<a:p><a:pPr algn="%s"/><a:r><a:rPr lang="zh-CN" sz="%d"`, align, size)
		if element.Bold {
			body.WriteString(` b="1"`)
		}
		if element.Italic {
			body.WriteString(` i="1"`)
		}
		if element.Underline {
			body.WriteString(` u="sng"`)
		}
		body.WriteString(`>`)
		fmt.Fprintf(body, `<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, strings.ToUpper(strings.TrimPrefix(element.Color, "#")))
		if element.Font != "" {
			latin, east := pptFontOut(element.Font)
			fmt.Fprintf(body, `<a:latin typeface="%s"/><a:ea typeface="%s"/>`, xmlEscape(latin), xmlEscape(east))
		}
		body.WriteString(`</a:rPr><a:t xml:space="preserve">`)
		body.WriteString(xmlEscape(line))
		body.WriteString(`</a:t></a:r></a:p>`)
	}
	body.WriteString(`</p:txBody></p:sp>`)
}

func pptAlignOut(align string) string {
	switch align {
	case "center":
		return "ctr"
	case "right":
		return "r"
	default:
		return "l"
	}
}

func pptFontOut(font string) (string, string) {
	switch font {
	case "serif":
		return "Times New Roman", "宋体"
	case "mono":
		return "Consolas", "Consolas"
	default:
		return "Arial", "微软雅黑"
	}
}

func pxToEmu(value float64) int64 {
	return int64(math.Round(value * 12700))
}

func importPptx(filename string, body []byte) (string, string, []byte, error) {
	files, err := readZip(body)
	if err != nil {
		return "", "", nil, err
	}
	if zipKind(files) != KindPPT {
		return "", "", nil, apperrors.BadRequest(400, "这个文件不是有效的演示文稿")
	}
	parts, err := slideParts(files)
	if err != nil {
		return "", "", nil, err
	}
	if len(parts) == 0 {
		return "", "", nil, apperrors.BadRequest(400, "演示文稿没有幻灯片")
	}
	if len(parts) > 30 {
		return "", "", nil, apperrors.BadRequest(400, "幻灯片过多，无法导入")
	}
	slides := make([]pptSlide, 0, len(parts))
	for i, part := range parts {
		relsPath := path.Join(path.Dir(part.name), "_rels", path.Base(part.name)+".rels")
		background, elements, notes, err := parseSlide(part.data, imagePartsIn(files, files[relsPath], path.Dir(part.name)))
		if err != nil {
			return "", "", nil, readFailure(err)
		}
		if len(elements) > 40 {
			return "", "", nil, apperrors.BadRequest(400, "幻灯片元素过多，无法导入")
		}
		slides = append(slides, pptSlide{
			ID:         idAt("s", i+1),
			Background: background,
			Elements:   elements,
			Notes:      notes,
		})
	}
	raw, err := json.Marshal(pptContent{Slides: slides})
	if err != nil {
		return "", "", nil, apperrors.WrapInternal(err)
	}
	canonical, err := Canonical(KindPPT, raw)
	if err != nil {
		return "", "", nil, err
	}
	return KindPPT, titleFromFilename(filename), canonical, nil
}

type slideSource struct {
	name string
	data []byte
}

func slideParts(files map[string][]byte) ([]slideSource, error) {
	rels := parseRels(files["ppt/_rels/presentation.xml.rels"])
	var ids []string
	if err := walkXML(files["ppt/presentation.xml"], func(token xml.Token) error {
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "sldId" {
			return nil
		}
		if id := relID(start); id != "" {
			ids = append(ids, id)
		}
		return nil
	}); err != nil {
		return nil, apperrors.BadRequest(400, "演示文稿无法读取")
	}
	parts := make([]slideSource, 0, len(ids))
	for _, id := range ids {
		target := strings.TrimPrefix(rels[id], "/")
		if target != "" && !strings.HasPrefix(target, "ppt/") {
			target = pathJoin("ppt", target)
		}
		if files[target] != nil {
			parts = append(parts, slideSource{name: target, data: files[target]})
		}
	}
	if len(parts) > 0 {
		return parts, nil
	}
	type numbered struct {
		n    int
		name string
		data []byte
	}
	found := make([]numbered, 0)
	for name, data := range files {
		match := slidePartPattern.FindStringSubmatch(name)
		if match == nil {
			continue
		}
		n, _ := strconv.Atoi(match[1])
		found = append(found, numbered{n: n, name: name, data: data})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].n < found[j].n })
	for _, item := range found {
		parts = append(parts, slideSource{name: item.name, data: item.data})
	}
	return parts, nil
}

type shapeDraft struct {
	name       string
	prst       string
	fill       string
	color      string
	align      string
	font       string
	text       strings.Builder
	bold       bool
	italic     bool
	underline  bool
	size       float64
	x, y, w, h float64
	hasPos     bool
	picture    bool
	image      string
}

func parseSlide(data []byte, images map[string]string) (string, []pptElement, string, error) {
	background := "#ffffff"
	var notes strings.Builder
	var elements []pptElement
	var stack []string
	var shapes []*shapeDraft
	inBackground := 0
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
			stack = append(stack, node.Name.Local)
			switch node.Name.Local {
			case "bg":
				inBackground++
			case "sp":
				shapes = append(shapes, &shapeDraft{color: "#202124", align: "left", size: 18})
			case "pic":
				shapes = append(shapes, &shapeDraft{color: "#202124", align: "left", size: 18, picture: true})
			case "blip":
				if shape := topShape(shapes); shape != nil && shape.picture {
					if data := images[attr(node, "embed")]; data != "" {
						shape.image = data
					}
				}
			case "cNvPr":
				if shape := topShape(shapes); shape != nil && !insideGroup(stack) {
					shape.name = attr(node, "name")
				}
			case "off":
				if shape := topShape(shapes); shape != nil && parentIs(stack, "xfrm") && !insideGroup(stack) {
					shape.x = emuToPx(attr(node, "x"))
					shape.y = emuToPx(attr(node, "y"))
					shape.hasPos = true
				}
			case "ext":
				if shape := topShape(shapes); shape != nil && parentIs(stack, "xfrm") && !insideGroup(stack) {
					shape.w = emuToPx(attr(node, "cx"))
					shape.h = emuToPx(attr(node, "cy"))
				}
			case "prstGeom":
				if shape := topShape(shapes); shape != nil {
					shape.prst = attr(node, "prst")
				}
			case "srgbClr":
				color := hexColor(attr(node, "val"))
				if color == "" {
					break
				}
				if inBackground > 0 && len(shapes) == 0 {
					background = color
				} else if shape := topShape(shapes); shape != nil {
					switch {
					case hasAncestor(stack, "rPr"):
						shape.color = color
					case hasAncestor(stack, "spPr") && !hasAncestor(stack, "ln") && !hasAncestor(stack, "txBody"):
						shape.fill = color
					}
				}
			case "pPr":
				if shape := topShape(shapes); shape != nil && hasAncestor(stack, "txBody") {
					if align := pptAlignIn(attr(node, "algn")); align != "" {
						shape.align = align
					}
				}
			case "rPr":
				if shape := topShape(shapes); shape != nil {
					if value, ok := attrPresent(node, "sz"); ok {
						if n, err := strconv.Atoi(value); err == nil {
							shape.size = float64(n) / 100
						}
					}
					if value, ok := attrPresent(node, "b"); ok {
						shape.bold = drawOn(value)
					}
					if value, ok := attrPresent(node, "i"); ok {
						shape.italic = drawOn(value)
					}
					if value, ok := attrPresent(node, "u"); ok {
						shape.underline = drawOn(value)
					}
				}
			case "latin":
				if shape := topShape(shapes); shape != nil && hasAncestor(stack, "rPr") {
					if font := pptFontIn(attr(node, "typeface")); font != "" {
						shape.font = font
					}
				}
			case "t":
				readingText = true
			}
		case xml.EndElement:
			if node.Name.Local == "Fallback" {
				if fallback > 0 {
					fallback--
				}
				return nil
			}
			if fallback > 0 {
				return nil
			}
			switch node.Name.Local {
			case "bg":
				if inBackground > 0 {
					inBackground--
				}
			case "t":
				readingText = false
			case "p":
				if shape := topShape(shapes); shape != nil && hasAncestor(stack, "txBody") {
					if shape.text.Len() == 0 || !strings.HasSuffix(shape.text.String(), "\n") {
						shape.text.WriteByte('\n')
					}
				}
			case "sp", "pic":
				if len(shapes) > 0 {
					shape := shapes[len(shapes)-1]
					shapes = shapes[:len(shapes)-1]
					appendShape(shape, &elements, &notes)
				}
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if fallback > 0 {
				break
			}
			if readingText {
				if shape := topShape(shapes); shape != nil {
					shape.text.Write(node)
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", nil, "", err
	}
	if !colorPattern.MatchString(background) {
		background = "#ffffff"
	}
	return background, elements, strings.TrimSpace(notes.String()), nil
}

func appendShape(shape *shapeDraft, elements *[]pptElement, notes *strings.Builder) {
	if shape.picture {
		appendPicture(shape, elements)
		return
	}
	text := strings.TrimRight(cleanMultiline(shape.text.String()), "\n")
	if shape.name == "云笺备注" {
		if strings.TrimSpace(text) == "" {
			return
		}
		if notes.Len() > 0 {
			notes.WriteByte('\n')
		}
		notes.WriteString(text)
		return
	}
	kind := "text"
	switch {
	case shape.name == "云笺形状", shape.prst == "ellipse", shape.prst == "roundRect":
		kind = "shape"
	case shape.prst == "rect" && shape.fill != "":
		kind = "shape"
	}
	shapeName := ""
	fill := ""
	if kind == "shape" {
		switch shape.prst {
		case "ellipse":
			shapeName = "ellipse"
		case "roundRect":
			shapeName = "round"
		default:
			shapeName = "rect"
		}
		fill = shape.fill
	}
	if kind == "text" && strings.TrimSpace(text) == "" {
		return
	}
	if !shape.hasPos {
		shape.x = 68
		shape.y = float64(40 + len(*elements)*72)
		if shape.y > 420 {
			shape.y = 420
		}
		shape.w = 824
		shape.h = 72
	}
	if shape.w <= 0 {
		shape.w = 200
	}
	if shape.h <= 0 {
		shape.h = 48
	}
	element := pptElement{
		ID:        idAt("e", len(*elements)+1),
		Kind:      kind,
		X:         clampFloat(roundPx(shape.x), -200, 1100),
		Y:         clampFloat(roundPx(shape.y), -200, 700),
		W:         clampFloat(roundPx(shape.w), 24, 960),
		H:         clampFloat(roundPx(shape.h), 24, 540),
		Text:      text,
		FontSize:  clampFloat(roundPx(shape.size), 12, 96),
		Color:     shape.color,
		Bold:      shape.bold,
		Align:     shape.align,
		Italic:    shape.italic,
		Underline: shape.underline,
		Fill:      fill,
		Shape:     shapeName,
		Font:      shape.font,
	}
	if !colorPattern.MatchString(element.Color) {
		element.Color = "#202124"
	}
	if element.Fill != "" && !colorPattern.MatchString(element.Fill) {
		element.Fill = ""
	}
	switch element.Align {
	case "left", "center", "right":
	default:
		element.Align = "left"
	}
	*elements = append(*elements, element)
}

func appendPicture(shape *shapeDraft, elements *[]pptElement) {
	if shape.image == "" {
		return
	}
	if !shape.hasPos {
		shape.x = 180
		shape.y = 120
		shape.w = 480
		shape.h = 280
	}
	if shape.w <= 0 {
		shape.w = 320
	}
	if shape.h <= 0 {
		shape.h = 200
	}
	*elements = append(*elements, pptElement{
		ID:       idAt("e", len(*elements)+1),
		Kind:     "image",
		X:        clampFloat(roundPx(shape.x), -200, 1100),
		Y:        clampFloat(roundPx(shape.y), -200, 700),
		W:        clampFloat(roundPx(shape.w), 24, 960),
		H:        clampFloat(roundPx(shape.h), 24, 540),
		Text:     "",
		FontSize: 18,
		Color:    "#202124",
		Align:    "left",
		Image:    shape.image,
	})
}

func topShape(shapes []*shapeDraft) *shapeDraft {
	if len(shapes) == 0 {
		return nil
	}
	return shapes[len(shapes)-1]
}

func parentIs(stack []string, name string) bool {
	if len(stack) < 2 {
		return false
	}
	return stack[len(stack)-2] == name
}

func insideGroup(stack []string) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i] {
		case "sp":
			return false
		case "grpSp":
			return true
		}
	}
	return false
}

func emuToPx(value string) float64 {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return float64(n) / 12700
}

func roundPx(value float64) float64 {
	return math.Round(value*100) / 100
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func pptAlignIn(value string) string {
	switch strings.ToLower(value) {
	case "ctr", "center":
		return "center"
	case "r", "right":
		return "right"
	case "l", "left", "":
		return "left"
	default:
		return "left"
	}
}

func pptFontIn(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "consol"):
		return "mono"
	case strings.Contains(lower, "times"), strings.Contains(lower, "song"), strings.Contains(name, "宋"):
		return "serif"
	case strings.Contains(lower, "arial"), strings.Contains(lower, "calibri"), strings.Contains(name, "黑"):
		return "sans"
	default:
		return ""
	}
}

func drawOn(value string) bool {
	switch strings.ToLower(value) {
	case "1", "true", "on", "sng":
		return true
	default:
		return false
	}
}
