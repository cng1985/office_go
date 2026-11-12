package document

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxPDFObjects   = 20000
	maxPDFPages     = 300
	maxPDFFormCalls = 4096
	maxPDFRuns      = 65536
	maxPDFOps       = 2000000
)

var (
	errPDFSyntax  = errors.New("pdf syntax")
	errPDFFilter  = errors.New("pdf filter")
	errPDFEncrypt = errors.New("pdf encrypt")
	errPDFLimit   = errors.New("pdf limit")
)

const (
	objNull = iota
	objBool
	objNum
	objName
	objStr
	objArray
	objDict
	objRef
)

type pdfObj struct {
	kind      int
	num       float64
	text      string
	raw       []byte
	arr       []pdfObj
	dict      map[string]pdfObj
	ref       int
	stream    []byte
	hasStream bool
	image     []byte
}

type pdfLoc struct {
	kind int
	a, b int
}

type pdfDoc struct {
	data       []byte
	refs       map[int]pdfLoc
	cache      map[int]pdfObj
	loading    map[int]bool
	seenXRef   map[int]bool
	seenNode   map[int]bool
	root       int
	encrypted  bool
	depth      int
	formCalls  int
	runCount   int
	ops        int
	imageBytes int
}

type pdfPageText struct {
	width  float64
	lines  []pdfTextLine
	images []pdfPlacedImage
}

type pdfTextLine struct {
	text string
	size float64
	x    float64
	y    float64
}

type pdfEnc struct {
	cmap   map[uint16]string
	width  int
	simple bool
}

type pdfResources struct {
	fonts map[string]*pdfEnc
	forms map[string]int
}

type pdfMat struct{ a, b, c, d, e, f float64 }

func identMat() pdfMat { return pdfMat{1, 0, 0, 1, 0, 0} }

func (m pdfMat) mul(n pdfMat) pdfMat {
	return pdfMat{
		a: m.a*n.a + m.c*n.b,
		b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d,
		d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e,
		f: m.b*n.e + m.d*n.f + m.f,
	}
}

func (m pdfMat) pt(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

type pdfRun struct {
	x, y, size float64
	text       string
}

func extractPDFPages(data []byte) ([]pdfPageText, error) {
	doc, err := openPDF(data)
	if err != nil {
		return nil, err
	}
	if doc.encrypted {
		return nil, errPDFEncrypt
	}
	if doc.root == 0 {
		return nil, errPDFSyntax
	}
	root, err := doc.load(doc.root)
	if err != nil {
		return nil, err
	}
	start := doc.root
	if root.dict["Type"].text == "Catalog" {
		pages := root.dict["Pages"]
		if pages.kind != objRef {
			return nil, errPDFSyntax
		}
		start = pages.ref
	}
	var out []pdfPageText
	if err := doc.walk(start, pdfObj{}, 595.28, 0, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func openPDF(data []byte) (*pdfDoc, error) {
	doc := &pdfDoc{
		data:     data,
		refs:     map[int]pdfLoc{},
		cache:    map[int]pdfObj{},
		loading:  map[int]bool{},
		seenXRef: map[int]bool{},
		seenNode: map[int]bool{},
	}
	off := findStartXRef(data)
	if off >= 0 {
		if err := doc.readXRef(off); err != nil {
			doc.refs = map[int]pdfLoc{}
			doc.root = 0
			doc.encrypted = false
		}
	}
	if len(doc.refs) == 0 {
		doc.refs = scanPDFObjects(data)
	}
	if doc.root == 0 && !doc.encrypted {
		doc.root = doc.findCatalog()
	}
	if len(doc.refs) == 0 || doc.root == 0 {
		if doc.encrypted {
			return nil, errPDFEncrypt
		}
		return nil, errPDFSyntax
	}
	return doc, nil
}

func findStartXRef(data []byte) int {
	at := bytes.LastIndex(data, []byte("startxref"))
	if at < 0 {
		return -1
	}
	p := pdfParser{b: data, i: at + len("startxref")}
	n, err := p.readInt()
	if err != nil || n < 0 || n >= len(data) {
		return -1
	}
	return n
}

func scanPDFObjects(data []byte) map[int]pdfLoc {
	refs := map[int]pdfLoc{}
	for i := 0; i < len(data)-7; i++ {
		if data[i] < '0' || data[i] > '9' {
			continue
		}
		if i > 0 && data[i-1] != '\n' && data[i-1] != '\r' {
			continue
		}
		p := pdfParser{b: data, i: i}
		id, err := p.readInt()
		if err != nil || id <= 0 {
			continue
		}
		if _, err := p.readInt(); err != nil || !p.consumeWord("obj") {
			continue
		}
		refs[id] = pdfLoc{kind: 1, a: i}
		if len(refs) > maxPDFObjects {
			break
		}
	}
	return refs
}

func (d *pdfDoc) findCatalog() int {
	ids := make([]int, 0, len(d.refs))
	for id, loc := range d.refs {
		if loc.kind == 1 {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	for _, id := range ids {
		obj, err := d.load(id)
		if err != nil {
			continue
		}
		if obj.dict["Type"].text == "Catalog" {
			return id
		}
	}
	return 0
}

func (d *pdfDoc) readXRef(off int) error {
	if off < 0 || off >= len(d.data) || d.seenXRef[off] {
		return nil
	}
	d.seenXRef[off] = true
	p := &pdfParser{b: d.data, i: off, resolve: d.load}
	p.skip()
	if p.consumeWord("xref") {
		trailer, err := d.parseClassicXRef(p)
		if err != nil {
			return err
		}
		d.noteTrailer(trailer)
		return d.followTrailer(trailer)
	}
	obj, err := d.parseIndirect(p)
	if err != nil {
		return err
	}
	if !obj.hasStream {
		return errPDFSyntax
	}
	d.noteTrailer(obj)
	if err := d.readXRefStream(obj); err != nil {
		return err
	}
	return d.followTrailer(obj)
}

func (d *pdfDoc) noteTrailer(obj pdfObj) {
	if d.root == 0 && obj.dict["Root"].kind == objRef {
		d.root = obj.dict["Root"].ref
	}
	if _, ok := obj.dict["Encrypt"]; ok {
		d.encrypted = true
	}
}

func (d *pdfDoc) followTrailer(obj pdfObj) error {
	if stm := obj.dict["XRefStm"]; stm.kind == objNum {
		if err := d.readXRef(int(stm.num)); err != nil {
			return err
		}
	}
	if prev := obj.dict["Prev"]; prev.kind == objNum {
		return d.readXRef(int(prev.num))
	}
	return nil
}

func (d *pdfDoc) parseClassicXRef(p *pdfParser) (pdfObj, error) {
	for {
		pos := p.i
		p.skip()
		if p.i == pos && p.i >= len(p.b) {
			return pdfObj{}, errPDFSyntax
		}
		if p.consumeWord("trailer") {
			return p.parseValue()
		}
		start, err := p.readInt()
		if err != nil {
			return pdfObj{}, err
		}
		count, err := p.readInt()
		if err != nil || count < 0 || count > maxPDFObjects {
			return pdfObj{}, errPDFSyntax
		}
		for i := 0; i < count; i++ {
			off, err := p.readInt()
			if err != nil {
				return pdfObj{}, err
			}
			if _, err := p.readInt(); err != nil {
				return pdfObj{}, err
			}
			flag := p.readWord()
			switch flag {
			case "n":
				d.put(start+i, pdfLoc{kind: 1, a: off})
			case "f":
				d.put(start+i, pdfLoc{})
			}
		}
	}
}

func (d *pdfDoc) readXRefStream(obj pdfObj) error {
	w := obj.dict["W"]
	if w.kind != objArray || len(w.arr) < 3 {
		return errPDFSyntax
	}
	widths := []int{int(w.arr[0].num), int(w.arr[1].num), int(w.arr[2].num)}
	entry := widths[0] + widths[1] + widths[2]
	if widths[0] > 4 || widths[1] > 4 || widths[2] > 4 || entry <= 0 || len(obj.stream)%entry != 0 {
		return errPDFSyntax
	}
	index := []int{0, int(obj.dict["Size"].num)}
	if raw := obj.dict["Index"]; raw.kind == objArray && len(raw.arr) >= 2 && len(raw.arr)%2 == 0 {
		index = index[:0]
		for _, item := range raw.arr {
			index = append(index, int(item.num))
		}
	}
	pos := 0
	for i := 0; i+1 < len(index); i += 2 {
		start, count := index[i], index[i+1]
		for n := 0; n < count; n++ {
			if pos+entry > len(obj.stream) {
				return errPDFSyntax
			}
			kind, rest := readField(obj.stream[pos:], widths[0])
			offset, rest := readField(rest, widths[1])
			field, _ := readField(rest, widths[2])
			pos += entry
			if widths[0] == 0 {
				kind = 1
			}
			switch kind {
			case 0:
				d.put(start+n, pdfLoc{})
			case 1:
				d.put(start+n, pdfLoc{kind: 1, a: offset})
			case 2:
				d.put(start+n, pdfLoc{kind: 2, a: offset, b: field})
			}
		}
	}
	return nil
}

func readField(data []byte, width int) (int, []byte) {
	if width <= 0 {
		return 0, data
	}
	if len(data) < width || width > 4 {
		return 0, nil
	}
	var value int
	for i := 0; i < width; i++ {
		value = value<<8 | int(data[i])
	}
	return value, data[width:]
}

func (d *pdfDoc) put(id int, loc pdfLoc) {
	if id <= 0 || id > maxPDFObjects {
		return
	}
	if _, ok := d.refs[id]; ok {
		return
	}
	d.refs[id] = loc
}

func (d *pdfDoc) load(id int) (pdfObj, error) {
	if id <= 0 {
		return pdfObj{}, errPDFSyntax
	}
	if obj, ok := d.cache[id]; ok {
		return obj, nil
	}
	if d.loading[id] || d.depth > 12 {
		return pdfObj{}, errPDFSyntax
	}
	loc, ok := d.refs[id]
	if !ok || loc.kind == 0 {
		return pdfObj{}, errPDFSyntax
	}
	d.loading[id] = true
	d.depth++
	defer func() {
		delete(d.loading, id)
		d.depth--
	}()
	var obj pdfObj
	var err error
	if loc.kind == 2 {
		obj, err = d.fromObjStm(loc.a, loc.b)
	} else {
		if loc.a < 0 || loc.a >= len(d.data) {
			return pdfObj{}, errPDFSyntax
		}
		p := &pdfParser{b: d.data, i: loc.a, resolve: d.load}
		obj, err = d.parseIndirect(p)
	}
	if err != nil {
		return pdfObj{}, err
	}
	d.cache[id] = obj
	return obj, nil
}

func (d *pdfDoc) parseIndirect(p *pdfParser) (pdfObj, error) {
	if _, err := p.readInt(); err != nil {
		return pdfObj{}, err
	}
	if _, err := p.readInt(); err != nil {
		return pdfObj{}, err
	}
	if !p.consumeWord("obj") {
		return pdfObj{}, errPDFSyntax
	}
	obj, err := p.parseValue()
	if err != nil {
		return pdfObj{}, err
	}
	// 图片单独转成 JPEG。过滤器失败时丢掉这一张，避免 DCTDecode 把整份文件判成无法读取。
	if obj.dict["Subtype"].text == "Image" {
		if raw, err := rawPDFStream(p, obj); err == nil {
			obj.image = imageFromPDFStream(obj, raw)
		}
		return obj, nil
	}
	p.skip()
	if !p.consumeWord("stream") {
		return obj, nil
	}
	if p.i < len(p.b) && p.b[p.i] == '\r' {
		p.i++
	}
	if p.i < len(p.b) && p.b[p.i] == '\n' {
		p.i++
	}
	raw, err := p.streamBytes(obj)
	if err != nil {
		return pdfObj{}, err
	}
	decoded, err := d.applyFilters(obj, raw)
	if err != nil {
		return pdfObj{}, err
	}
	obj.stream = decoded
	obj.hasStream = true
	return obj, nil
}

func rawPDFStream(p *pdfParser, obj pdfObj) ([]byte, error) {
	p.skip()
	if !p.consumeWord("stream") {
		return nil, errPDFSyntax
	}
	if p.i < len(p.b) && p.b[p.i] == '\r' {
		p.i++
	}
	if p.i < len(p.b) && p.b[p.i] == '\n' {
		p.i++
	}
	return p.streamBytes(obj)
}

func (p *pdfParser) streamBytes(obj pdfObj) ([]byte, error) {
	length := obj.dict["Length"]
	if length.kind == objRef && p.resolve != nil {
		target, err := p.resolve(length.ref)
		if err != nil {
			return nil, err
		}
		length = target
	}
	if length.kind == objNum {
		n := int(length.num)
		if n < 0 || p.i > len(p.b) || n > len(p.b)-p.i {
			return nil, errPDFSyntax
		}
		return p.b[p.i : p.i+n], nil
	}
	at := bytes.Index(p.b[p.i:], []byte("endstream"))
	if at < 0 {
		return nil, errPDFSyntax
	}
	raw := p.b[p.i : p.i+at]
	raw = bytes.TrimRight(raw, "\r\n")
	return raw, nil
}

func (d *pdfDoc) applyFilters(obj pdfObj, raw []byte) ([]byte, error) {
	if parms := obj.dict["DecodeParms"]; parms.kind == objDict {
		if pred := parms.dict["Predictor"]; pred.kind == objNum && pred.num > 1 {
			return nil, errPDFFilter
		}
	}
	names := filterNames(obj)
	data := raw
	var err error
	for _, name := range names {
		switch name {
		case "FlateDecode", "Fl":
			data, err = inflatePDF(data)
		case "ASCIIHexDecode", "AHx":
			data, err = asciiHexDecode(data)
		default:
			return nil, errPDFFilter
		}
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func filterNames(obj pdfObj) []string {
	switch obj.dict["Filter"].kind {
	case objName:
		return []string{obj.dict["Filter"].text}
	case objArray:
		var names []string
		for _, item := range obj.dict["Filter"].arr {
			if item.kind == objName {
				names = append(names, item.text)
			}
		}
		return names
	default:
		return nil
	}
}

func inflatePDF(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, errPDFSyntax
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		return nil, errPDFSyntax
	}
	return out, nil
}

func asciiHexDecode(data []byte) ([]byte, error) {
	var hex []byte
	for _, c := range data {
		if c == '>' {
			break
		}
		if isHex(c) {
			hex = append(hex, c)
		}
	}
	if len(hex)%2 == 1 {
		hex = append(hex, '0')
	}
	out := make([]byte, len(hex)/2)
	for i := 0; i < len(out); i++ {
		value, err := strconv.ParseUint(string(hex[i*2:i*2+2]), 16, 8)
		if err != nil {
			return nil, errPDFSyntax
		}
		out[i] = byte(value)
	}
	return out, nil
}

func (d *pdfDoc) fromObjStm(stmID, index int) (pdfObj, error) {
	container, err := d.load(stmID)
	if err != nil || !container.hasStream {
		return pdfObj{}, errPDFSyntax
	}
	nObj := container.dict["N"]
	first := container.dict["First"]
	if nObj.kind != objNum || first.kind != objNum {
		return pdfObj{}, errPDFSyntax
	}
	count := int(nObj.num)
	at := int(first.num)
	if count <= 0 || count > maxPDFObjects || index < 0 || index >= count || at < 0 || at > len(container.stream) {
		return pdfObj{}, errPDFSyntax
	}
	header := pdfParser{b: container.stream[:at]}
	offs := make([]int, count)
	for i := 0; i < count; i++ {
		if _, err := header.readInt(); err != nil {
			return pdfObj{}, err
		}
		offs[i], err = header.readInt()
		if err != nil {
			return pdfObj{}, err
		}
	}
	start := at + offs[index]
	end := len(container.stream)
	if index+1 < count {
		end = at + offs[index+1]
	}
	if start < 0 || end < start || end > len(container.stream) {
		return pdfObj{}, errPDFSyntax
	}
	body := pdfParser{b: container.stream[start:end], resolve: d.load}
	return body.parseValue()
}

func (d *pdfDoc) walk(id int, inherited pdfObj, width float64, depth int, out *[]pdfPageText) error {
	if depth > 32 || id <= 0 {
		return errPDFSyntax
	}
	if d.seenNode[id] {
		return errPDFSyntax
	}
	d.seenNode[id] = true
	node, err := d.load(id)
	if err != nil {
		return err
	}
	if box := mediaWidth(node); box > 0 {
		width = box
	}
	if node.dict["Resources"].kind != 0 {
		res, err := d.concrete(node.dict["Resources"])
		if err != nil {
			return err
		}
		inherited = res
	}
	if kids := node.dict["Kids"]; kids.kind != 0 {
		items, err := d.concrete(kids)
		if err != nil || items.kind != objArray {
			return errPDFSyntax
		}
		for _, kid := range items.arr {
			if kid.kind != objRef {
				continue
			}
			if len(*out) > maxPDFPages {
				return errPDFSyntax
			}
			if err := d.walk(kid.ref, inherited, width, depth+1, out); err != nil {
				return err
			}
		}
		return nil
	}
	if len(*out) >= maxPDFPages {
		return errPDFSyntax
	}
	page, err := d.readPage(node, inherited)
	if err != nil {
		return err
	}
	page.width = width
	*out = append(*out, page)
	return nil
}

func mediaWidth(obj pdfObj) float64 {
	box := obj.dict["MediaBox"]
	if box.kind != objArray || len(box.arr) < 4 {
		return 0
	}
	if box.arr[2].num > box.arr[0].num {
		return box.arr[2].num - box.arr[0].num
	}
	return 0
}

func (d *pdfDoc) concrete(obj pdfObj) (pdfObj, error) {
	if obj.kind != objRef {
		return obj, nil
	}
	return d.load(obj.ref)
}

func (d *pdfDoc) readPage(page, resources pdfObj) (pdfPageText, error) {
	res, err := d.parseResources(resources)
	if err != nil {
		return pdfPageText{}, err
	}
	streams, err := d.pageStreams(page)
	if err != nil {
		return pdfPageText{}, err
	}
	interp := &pdfInterp{doc: d, res: res, ctm: identMat(), tm: identMat(), tlm: identMat(), fontSize: 12}
	for _, stream := range streams {
		if err := interp.run(stream, res); err != nil {
			return pdfPageText{}, err
		}
	}
	return pdfPageText{lines: clusterRuns(interp.runs), images: interp.images}, nil
}

func (d *pdfDoc) pageStreams(page pdfObj) ([][]byte, error) {
	contents, ok := page.dict["Contents"]
	if !ok {
		return nil, nil
	}
	value, err := d.concrete(contents)
	if err != nil {
		return nil, err
	}
	if value.hasStream {
		return [][]byte{value.stream}, nil
	}
	if value.kind != objArray {
		return nil, nil
	}
	var out [][]byte
	for _, item := range value.arr {
		stm, err := d.concrete(item)
		if err != nil {
			return nil, err
		}
		if stm.hasStream {
			out = append(out, stm.stream)
		}
	}
	return out, nil
}

func (d *pdfDoc) parseResources(obj pdfObj) (pdfResources, error) {
	res := pdfResources{fonts: map[string]*pdfEnc{}, forms: map[string]int{}}
	if obj.kind == 0 {
		return res, nil
	}
	value, err := d.concrete(obj)
	if err != nil {
		return res, err
	}
	if fonts := value.dict["Font"]; fonts.kind != 0 {
		fontDict, err := d.concrete(fonts)
		if err != nil {
			return res, err
		}
		for name, item := range fontDict.dict {
			if item.kind != objRef {
				continue
			}
			enc, err := d.fontEnc(item.ref)
			if err != nil {
				return res, err
			}
			res.fonts[name] = enc
		}
	}
	if xobj := value.dict["XObject"]; xobj.kind != 0 {
		dict, err := d.concrete(xobj)
		if err != nil {
			return res, err
		}
		for name, item := range dict.dict {
			if item.kind == objRef {
				res.forms[name] = item.ref
			}
		}
	}
	return res, nil
}

func (d *pdfDoc) fontEnc(id int) (*pdfEnc, error) {
	obj, err := d.load(id)
	if err != nil {
		return nil, err
	}
	enc := &pdfEnc{simple: true, width: 1, cmap: map[uint16]string{}}
	if obj.dict["Subtype"].text == "Type0" || obj.dict["Encoding"].text == "Identity-H" || obj.dict["Encoding"].text == "Identity-V" {
		enc.simple = false
		enc.width = 2
	}
	to := obj.dict["ToUnicode"]
	if to.kind != objRef {
		return enc, nil
	}
	stm, err := d.load(to.ref)
	if err != nil {
		return nil, err
	}
	cmap, width, err := parseToUnicode(stm.stream)
	if err != nil {
		return nil, err
	}
	enc.cmap = cmap
	enc.simple = false
	if width > 0 {
		enc.width = width
	}
	return enc, nil
}

func (e *pdfEnc) decode(raw []byte) string {
	if e == nil || e.simple || len(e.cmap) == 0 {
		if e != nil && !e.simple {
			return ""
		}
		return decodePlain(raw)
	}
	width := e.width
	if width != 2 {
		width = 1
	}
	var b strings.Builder
	for i := 0; i+width <= len(raw); i += width {
		var code uint16
		if width == 1 {
			code = uint16(raw[i])
		} else {
			code = uint16(raw[i])<<8 | uint16(raw[i+1])
		}
		b.WriteString(e.cmap[code])
	}
	return b.String()
}

func decodePlain(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF {
		return utf16BEString(raw[2:])
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	var b strings.Builder
	for _, c := range raw {
		b.WriteRune(rune(c))
	}
	return b.String()
}

func utf16BEString(raw []byte) string {
	if len(raw)%2 != 0 {
		return ""
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = uint16(raw[i*2])<<8 | uint16(raw[i*2+1])
	}
	return string(utf16.Decode(units))
}

type pdfTok struct {
	kind int
	num  float64
	op   string
	raw  []byte
	arr  []pdfTok
}

type pdfParser struct {
	b       []byte
	i       int
	resolve func(int) (pdfObj, error)
	depth   int
}

func (p *pdfParser) parseValue() (pdfObj, error) {
	if p.depth > 32 {
		return pdfObj{}, errPDFSyntax
	}
	p.depth++
	defer func() { p.depth-- }()
	p.skip()
	if p.i >= len(p.b) {
		return pdfObj{}, errPDFSyntax
	}
	switch p.b[p.i] {
	case '<':
		if p.i+1 < len(p.b) && p.b[p.i+1] == '<' {
			return p.parseDict()
		}
		raw, err := p.parseHex()
		return pdfObj{kind: objStr, raw: raw}, err
	case '(':
		raw, err := p.parseLiteral()
		return pdfObj{kind: objStr, raw: raw}, err
	case '[':
		return p.parseArray()
	case '/':
		name, err := p.parseName()
		return pdfObj{kind: objName, text: name}, err
	case '+', '-', '.', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return p.parseNumberOrRef()
	default:
		word := p.readWord()
		switch word {
		case "true":
			return pdfObj{kind: objBool, num: 1}, nil
		case "false":
			return pdfObj{kind: objBool}, nil
		case "null":
			return pdfObj{kind: objNull}, nil
		default:
			return pdfObj{}, errPDFSyntax
		}
	}
}

func (p *pdfParser) parseDict() (pdfObj, error) {
	if p.i+1 >= len(p.b) || p.b[p.i] != '<' || p.b[p.i+1] != '<' {
		return pdfObj{}, errPDFSyntax
	}
	p.i += 2
	dict := map[string]pdfObj{}
	for {
		p.skip()
		if p.i+1 < len(p.b) && p.b[p.i] == '>' && p.b[p.i+1] == '>' {
			p.i += 2
			return pdfObj{kind: objDict, dict: dict}, nil
		}
		if p.i >= len(p.b) || p.b[p.i] != '/' {
			return pdfObj{}, errPDFSyntax
		}
		key, err := p.parseName()
		if err != nil {
			return pdfObj{}, err
		}
		value, err := p.parseValue()
		if err != nil {
			return pdfObj{}, err
		}
		if len(dict) > 10000 {
			return pdfObj{}, errPDFSyntax
		}
		dict[key] = value
	}
}

func (p *pdfParser) parseArray() (pdfObj, error) {
	p.i++
	var arr []pdfObj
	for {
		p.skip()
		if p.i < len(p.b) && p.b[p.i] == ']' {
			p.i++
			return pdfObj{kind: objArray, arr: arr}, nil
		}
		if len(arr) > 200000 {
			return pdfObj{}, errPDFSyntax
		}
		value, err := p.parseValue()
		if err != nil {
			return pdfObj{}, err
		}
		arr = append(arr, value)
	}
}

func (p *pdfParser) parseName() (string, error) {
	if p.i >= len(p.b) || p.b[p.i] != '/' {
		return "", errPDFSyntax
	}
	p.i++
	var buf []byte
	for p.i < len(p.b) && !isPDFDelim(p.b[p.i]) {
		if p.b[p.i] == '#' && p.i+2 < len(p.b) && isHex(p.b[p.i+1]) && isHex(p.b[p.i+2]) {
			value, err := strconv.ParseUint(string(p.b[p.i+1:p.i+3]), 16, 8)
			if err != nil {
				return "", errPDFSyntax
			}
			buf = append(buf, byte(value))
			p.i += 3
			continue
		}
		buf = append(buf, p.b[p.i])
		p.i++
	}
	return string(buf), nil
}

func (p *pdfParser) parseNumberOrRef() (pdfObj, error) {
	num, raw, err := p.readNumber()
	if err != nil {
		return pdfObj{}, err
	}
	if strings.Contains(raw, ".") || num < 0 {
		return pdfObj{kind: objNum, num: num}, nil
	}
	save := p.i
	p.skip()
	if p.i < len(p.b) && (p.b[p.i] == '+' || p.b[p.i] == '-' || isDigit(p.b[p.i])) {
		gen, raw2, err := p.readNumber()
		if err == nil && !strings.Contains(raw2, ".") && gen >= 0 {
			p.skip()
			if p.consumeWord("R") {
				return pdfObj{kind: objRef, ref: int(num)}, nil
			}
		}
	}
	p.i = save
	return pdfObj{kind: objNum, num: num}, nil
}

func (p *pdfParser) parseLiteral() ([]byte, error) {
	if p.i >= len(p.b) || p.b[p.i] != '(' {
		return nil, errPDFSyntax
	}
	p.i++
	var out []byte
	depth := 1
	for p.i < len(p.b) && depth > 0 {
		c := p.b[p.i]
		p.i++
		switch c {
		case '\\':
			if p.i >= len(p.b) {
				return nil, errPDFSyntax
			}
			esc := p.b[p.i]
			p.i++
			switch esc {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '(', ')', '\\':
				out = append(out, esc)
			case '\n':
			case '\r':
				if p.i < len(p.b) && p.b[p.i] == '\n' {
					p.i++
				}
			default:
				if esc >= '0' && esc <= '7' {
					value := int(esc - '0')
					for n := 0; n < 2 && p.i < len(p.b) && p.b[p.i] >= '0' && p.b[p.i] <= '7'; n++ {
						value = value*8 + int(p.b[p.i]-'0')
						p.i++
					}
					out = append(out, byte(value))
				} else {
					out = append(out, esc)
				}
			}
		case '(':
			depth++
			out = append(out, c)
		case ')':
			depth--
			if depth > 0 {
				out = append(out, c)
			}
		default:
			out = append(out, c)
		}
	}
	if depth != 0 {
		return nil, errPDFSyntax
	}
	return out, nil
}

func (p *pdfParser) parseHex() ([]byte, error) {
	if p.i >= len(p.b) || p.b[p.i] != '<' {
		return nil, errPDFSyntax
	}
	p.i++
	var hex []byte
	for p.i < len(p.b) && p.b[p.i] != '>' {
		if isHex(p.b[p.i]) {
			hex = append(hex, p.b[p.i])
		}
		p.i++
	}
	if p.i >= len(p.b) || p.b[p.i] != '>' {
		return nil, errPDFSyntax
	}
	p.i++
	if len(hex)%2 == 1 {
		hex = append(hex, '0')
	}
	out := make([]byte, len(hex)/2)
	for i := range out {
		value, err := strconv.ParseUint(string(hex[i*2:i*2+2]), 16, 8)
		if err != nil {
			return nil, errPDFSyntax
		}
		out[i] = byte(value)
	}
	return out, nil
}

func (p *pdfParser) readNumber() (float64, string, error) {
	p.skip()
	start := p.i
	if p.i < len(p.b) && (p.b[p.i] == '+' || p.b[p.i] == '-') {
		p.i++
	}
	saw := false
	for p.i < len(p.b) && isDigit(p.b[p.i]) {
		p.i++
		saw = true
	}
	if p.i < len(p.b) && p.b[p.i] == '.' {
		p.i++
		for p.i < len(p.b) && isDigit(p.b[p.i]) {
			p.i++
			saw = true
		}
	}
	if !saw {
		p.i = start
		return 0, "", errPDFSyntax
	}
	raw := string(p.b[start:p.i])
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, "", errPDFSyntax
	}
	return value, raw, nil
}

func (p *pdfParser) readInt() (int, error) {
	p.skip()
	start := p.i
	if p.i < len(p.b) && (p.b[p.i] == '+' || p.b[p.i] == '-') {
		p.i++
	}
	if p.i >= len(p.b) || !isDigit(p.b[p.i]) {
		p.i = start
		return 0, errPDFSyntax
	}
	for p.i < len(p.b) && isDigit(p.b[p.i]) {
		p.i++
	}
	if p.i < len(p.b) && p.b[p.i] == '.' {
		p.i = start
		return 0, errPDFSyntax
	}
	value, err := strconv.Atoi(string(p.b[start:p.i]))
	if err != nil {
		return 0, errPDFSyntax
	}
	return value, nil
}

func (p *pdfParser) readWord() string {
	p.skip()
	start := p.i
	for p.i < len(p.b) && !isPDFDelim(p.b[p.i]) {
		p.i++
	}
	return string(p.b[start:p.i])
}

func (p *pdfParser) consumeWord(want string) bool {
	p.skip()
	end := p.i + len(want)
	if end > len(p.b) || string(p.b[p.i:end]) != want {
		return false
	}
	if end < len(p.b) && !isPDFDelim(p.b[end]) {
		return false
	}
	p.i = end
	return true
}

func (p *pdfParser) skip() {
	for p.i < len(p.b) {
		c := p.b[p.i]
		if c == '%' {
			for p.i < len(p.b) && p.b[p.i] != '\n' && p.b[p.i] != '\r' {
				p.i++
			}
			continue
		}
		if isPDFSpace(c) {
			p.i++
			continue
		}
		return
	}
}

func isPDFSpace(c byte) bool {
	return c == 0 || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func isPDFDelim(c byte) bool {
	if isPDFSpace(c) {
		return true
	}
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	default:
		return false
	}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func parseToUnicode(data []byte) (map[uint16]string, int, error) {
	p := &pdfParser{b: data}
	var toks []pdfTok
	for {
		p.skip()
		if p.i >= len(p.b) {
			break
		}
		tok, err := p.contentTok()
		if err != nil {
			return nil, 0, err
		}
		toks = append(toks, tok)
		if len(toks) > 200000 {
			return nil, 0, errPDFSyntax
		}
	}
	cmap := map[uint16]string{}
	width := 0
	count := 0
	for i := 0; i < len(toks); i++ {
		if toks[i].kind != 'o' {
			continue
		}
		switch toks[i].op {
		case "begincodespacerange":
			if i+1 < len(toks) && toks[i+1].kind == 'h' && len(toks[i+1].raw) > width {
				width = len(toks[i+1].raw)
			}
		case "beginbfchar":
			j := i + 1
			for j < len(toks) && !(toks[j].kind == 'o' && toks[j].op == "endbfchar") {
				if j+1 < len(toks) && toks[j].kind == 'h' && toks[j+1].kind == 'h' {
					if err := putCmap(cmap, &count, &width, toks[j].raw, destText(toks[j+1].raw)); err != nil {
						return nil, 0, err
					}
					j += 2
					continue
				}
				j++
			}
			i = j
		case "beginbfrange":
			j := i + 1
			for j < len(toks) && !(toks[j].kind == 'o' && toks[j].op == "endbfrange") {
				if j+2 < len(toks) && toks[j].kind == 'h' && toks[j+1].kind == 'h' && toks[j+2].kind == 'h' {
					if err := putRange(cmap, &count, &width, toks[j].raw, toks[j+1].raw, destText(toks[j+2].raw), nil); err != nil {
						return nil, 0, err
					}
					j += 3
					continue
				}
				if j+2 < len(toks) && toks[j].kind == 'h' && toks[j+1].kind == 'h' && toks[j+2].kind == 'a' {
					if err := putRange(cmap, &count, &width, toks[j].raw, toks[j+1].raw, "", toks[j+2].arr); err != nil {
						return nil, 0, err
					}
					j += 3
					continue
				}
				j++
			}
			i = j
		}
	}
	if width > 2 {
		return nil, 0, errPDFFilter
	}
	return cmap, width, nil
}

func putCmap(cmap map[uint16]string, count, width *int, src []byte, text string) error {
	if len(src) == 0 || text == "" {
		return nil
	}
	if len(src) > *width {
		*width = len(src)
	}
	if *count > 100000 {
		return errPDFSyntax
	}
	code := codeOf(src)
	if _, ok := cmap[code]; !ok {
		cmap[code] = text
		*count++
	}
	return nil
}

func putRange(cmap map[uint16]string, count, width *int, src1, src2 []byte, text string, items []pdfTok) error {
	start, end := codeOf(src1), codeOf(src2)
	if len(src1) > *width {
		*width = len(src1)
	}
	if end < start || int(end-start) > 4096 {
		return errPDFSyntax
	}
	src := src1
	if len(src) == 0 {
		src = []byte{0}
	}
	for code := start; ; code++ {
		var dest string
		if items != nil {
			idx := int(code - start)
			if idx < len(items) && items[idx].kind == 'h' {
				dest = destText(items[idx].raw)
			}
		} else {
			dest = shiftText(text, int(code-start))
		}
		if dest != "" {
			if *count > 100000 {
				return errPDFSyntax
			}
			if _, ok := cmap[code]; !ok {
				cmap[code] = dest
				*count++
			}
		}
		if code == end {
			break
		}
	}
	if len(src) > *width {
		*width = len(src)
	}
	return nil
}

func codeOf(src []byte) uint16 {
	if len(src) >= 2 {
		return uint16(src[0])<<8 | uint16(src[1])
	}
	if len(src) == 1 {
		return uint16(src[0])
	}
	return 0
}

func destText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw)%2 == 0 {
		return utf16BEString(raw)
	}
	return string(raw)
}

func shiftText(text string, delta int) string {
	if delta == 0 {
		return text
	}
	rs := []rune(text)
	if len(rs) == 0 {
		return ""
	}
	rs[len(rs)-1] += rune(delta)
	return string(rs)
}

func (p *pdfParser) contentTok() (pdfTok, error) {
	p.skip()
	if p.i >= len(p.b) {
		return pdfTok{}, errPDFSyntax
	}
	switch p.b[p.i] {
	case '(':
		raw, err := p.parseLiteral()
		return pdfTok{kind: 's', raw: raw}, err
	case '<':
		if p.i+1 < len(p.b) && p.b[p.i+1] == '<' {
			if _, err := p.parseValue(); err != nil {
				return pdfTok{}, err
			}
			return pdfTok{kind: 'd'}, nil
		}
		raw, err := p.parseHex()
		return pdfTok{kind: 'h', raw: raw}, err
	case '[':
		return p.contentArray()
	case '/':
		name, err := p.parseName()
		return pdfTok{kind: 'm', op: name}, err
	case '+', '-', '.', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		num, _, err := p.readNumber()
		return pdfTok{kind: 'n', num: num}, err
	default:
		word := p.readWord()
		if word == "BI" {
			return pdfTok{kind: 'o', op: word}, p.skipInlineImage()
		}
		if word == "" {
			return pdfTok{}, errPDFSyntax
		}
		return pdfTok{kind: 'o', op: word}, nil
	}
}

func (p *pdfParser) contentArray() (pdfTok, error) {
	p.i++
	var arr []pdfTok
	for {
		p.skip()
		if p.i < len(p.b) && p.b[p.i] == ']' {
			p.i++
			return pdfTok{kind: 'a', arr: arr}, nil
		}
		if len(arr) > 200000 {
			return pdfTok{}, errPDFSyntax
		}
		tok, err := p.contentTok()
		if err != nil {
			return pdfTok{}, err
		}
		arr = append(arr, tok)
	}
}

func (p *pdfParser) skipInlineImage() error {
	for p.i+1 < len(p.b) {
		if p.b[p.i] == 'E' && p.b[p.i+1] == 'I' && (p.i == 0 || isPDFDelim(p.b[p.i-1])) && (p.i+2 >= len(p.b) || isPDFDelim(p.b[p.i+2])) {
			p.i += 2
			return nil
		}
		p.i++
	}
	return errPDFSyntax
}

type pdfInterp struct {
	doc      *pdfDoc
	res      pdfResources
	ctm      pdfMat
	tm       pdfMat
	tlm      pdfMat
	gs       []pdfMat
	font     *pdfEnc
	fontSize float64
	leading  float64
	runs     []pdfRun
	images   []pdfPlacedImage
	depth    int
	seenForm map[int]bool
}

func (in *pdfInterp) run(data []byte, res pdfResources) error {
	savedRes := in.res
	in.res = res
	defer func() { in.res = savedRes }()
	p := pdfParser{b: data}
	var stack []pdfTok
	for {
		p.skip()
		if p.i >= len(p.b) {
			return nil
		}
		in.doc.ops++
		if in.doc.ops > maxPDFOps {
			return errPDFLimit
		}
		tok, err := p.contentTok()
		if err != nil {
			return err
		}
		if tok.kind != 'o' {
			if len(stack) > 10000 {
				return errPDFSyntax
			}
			stack = append(stack, tok)
			continue
		}
		if err := in.exec(tok.op, &stack); err != nil {
			return err
		}
	}
}

func (in *pdfInterp) exec(op string, stack *[]pdfTok) error {
	switch op {
	case "q":
		if len(in.gs) < 64 {
			in.gs = append(in.gs, in.ctm)
		}
	case "Q":
		if len(in.gs) > 0 {
			in.ctm = in.gs[len(in.gs)-1]
			in.gs = in.gs[:len(in.gs)-1]
		}
	case "cm":
		a, b, c, d, e, f, ok := pop6(stack)
		if ok {
			in.ctm = in.ctm.mul(pdfMat{a, b, c, d, e, f})
		}
	case "BT":
		in.tm, in.tlm = identMat(), identMat()
	case "ET":
	case "Tm":
		a, b, c, d, e, f, ok := pop6(stack)
		if ok {
			in.tm = pdfMat{a, b, c, d, e, f}
			in.tlm = in.tm
		}
	case "Td":
		tx, ty, ok := pop2(stack)
		if ok {
			in.translate(tx, ty)
		}
	case "TD":
		tx, ty, ok := pop2(stack)
		if ok {
			in.leading = -ty
			in.translate(tx, ty)
		}
	case "T*":
		in.translate(0, -in.leading)
	case "TL":
		if v, ok := popNum(stack); ok {
			in.leading = v
		}
	case "Tf":
		size, ok1 := popNum(stack)
		name, ok2 := popName(stack)
		if ok1 && ok2 {
			in.fontSize = size
			if in.fontSize <= 0 {
				in.fontSize = 12
			}
			in.font = in.res.fonts[name]
		}
	case "Tj":
		if raw, ok := popRaw(stack); ok {
			return in.show(raw)
		}
	case "TJ":
		if arr, ok := popArr(stack); ok {
			return in.showArray(arr)
		}
	case "'":
		if raw, ok := popRaw(stack); ok {
			in.translate(0, -in.leading)
			return in.show(raw)
		}
	case "\"":
		raw, ok1 := popRaw(stack)
		_, ok2 := popNum(stack)
		_, ok3 := popNum(stack)
		if ok1 && ok2 && ok3 {
			in.translate(0, -in.leading)
			return in.show(raw)
		}
	case "Do":
		name, ok := popName(stack)
		if ok {
			return in.doForm(name)
		}
	default:
		*stack = nil
	}
	return nil
}

func (in *pdfInterp) translate(tx, ty float64) {
	in.tlm = in.tlm.mul(pdfMat{1, 0, 0, 1, tx, ty})
	in.tm = in.tlm
}

func (in *pdfInterp) show(raw []byte) error {
	var text string
	if in.font != nil {
		text = in.font.decode(raw)
	} else {
		text = decodePlain(raw)
	}
	if text == "" {
		return nil
	}
	x, y := in.ctm.mul(in.tm).pt(0, 0)
	return in.addRun(pdfRun{x: x, y: y, size: in.visualSize(), text: text})
}

func (in *pdfInterp) showArray(arr []pdfTok) error {
	var b strings.Builder
	for _, tok := range arr {
		if tok.kind == 'n' && tok.num <= -200 {
			b.WriteByte(' ')
		}
		if tok.kind == 'h' || tok.kind == 's' {
			if in.font != nil {
				b.WriteString(in.font.decode(tok.raw))
			} else {
				b.WriteString(decodePlain(tok.raw))
			}
		}
	}
	if b.Len() == 0 {
		return nil
	}
	x, y := in.ctm.mul(in.tm).pt(0, 0)
	return in.addRun(pdfRun{x: x, y: y, size: in.visualSize(), text: b.String()})
}

func (in *pdfInterp) addRun(run pdfRun) error {
	in.doc.runCount++
	if in.doc.runCount > maxPDFRuns {
		return errPDFLimit
	}
	in.runs = append(in.runs, run)
	return nil
}

func (in *pdfInterp) visualSize() float64 {
	size := in.fontSize
	if size <= 0 {
		size = 12
	}
	x0, y0 := in.ctm.mul(in.tm).pt(0, 0)
	x1, y1 := in.ctm.mul(in.tm).pt(0, size)
	got := math.Hypot(x1-x0, y1-y0)
	if got < 0.5 {
		return size
	}
	return got
}

func (in *pdfInterp) doForm(name string) error {
	id := in.res.forms[name]
	if id == 0 || in.depth >= 4 {
		return nil
	}
	obj, err := in.doc.load(id)
	if err != nil {
		if errors.Is(err, errPDFFilter) {
			return nil
		}
		return err
	}
	if obj.dict["Subtype"].text == "Image" {
		return in.placeImage(obj)
	}
	if !obj.hasStream {
		return nil
	}
	if obj.dict["Subtype"].text != "Form" && obj.dict["BBox"].kind != objArray {
		return nil
	}
	if in.seenForm[id] {
		return nil
	}
	if in.doc.formCalls >= maxPDFFormCalls {
		return errPDFLimit
	}
	in.doc.formCalls++
	if in.seenForm == nil {
		in.seenForm = map[int]bool{}
	}
	in.seenForm[id] = true
	defer delete(in.seenForm, id)
	res := in.res
	if obj.dict["Resources"].kind != 0 {
		res, err = in.doc.parseResources(obj.dict["Resources"])
		if err != nil {
			return err
		}
	}
	matrix := identMat()
	if obj.dict["Matrix"].kind == objArray {
		matrix = matOf(obj.dict["Matrix"].arr)
	}
	saved := *in
	in.depth++
	in.ctm = saved.ctm.mul(matrix)
	in.tm, in.tlm = identMat(), identMat()
	in.gs = nil
	err = in.run(obj.stream, res)
	runs := in.runs
	images := in.images
	*in = saved
	in.runs = runs
	in.images = images
	return err
}

func matOf(arr []pdfObj) pdfMat {
	if len(arr) < 6 {
		return identMat()
	}
	return pdfMat{arr[0].num, arr[1].num, arr[2].num, arr[3].num, arr[4].num, arr[5].num}
}

func popNum(stack *[]pdfTok) (float64, bool) {
	s := *stack
	if len(s) == 0 || s[len(s)-1].kind != 'n' {
		return 0, false
	}
	*stack = s[:len(s)-1]
	return s[len(s)-1].num, true
}

func popName(stack *[]pdfTok) (string, bool) {
	s := *stack
	if len(s) == 0 || s[len(s)-1].kind != 'm' {
		return "", false
	}
	*stack = s[:len(s)-1]
	return s[len(s)-1].op, true
}

func popRaw(stack *[]pdfTok) ([]byte, bool) {
	s := *stack
	if len(s) == 0 {
		return nil, false
	}
	kind := s[len(s)-1].kind
	if kind != 'h' && kind != 's' {
		return nil, false
	}
	*stack = s[:len(s)-1]
	return s[len(s)-1].raw, true
}

func popArr(stack *[]pdfTok) ([]pdfTok, bool) {
	s := *stack
	if len(s) == 0 || s[len(s)-1].kind != 'a' {
		return nil, false
	}
	*stack = s[:len(s)-1]
	return s[len(s)-1].arr, true
}

func pop2(stack *[]pdfTok) (float64, float64, bool) {
	y, ok1 := popNum(stack)
	x, ok2 := popNum(stack)
	return x, y, ok1 && ok2
}

func pop6(stack *[]pdfTok) (float64, float64, float64, float64, float64, float64, bool) {
	f, ok1 := popNum(stack)
	e, ok2 := popNum(stack)
	d, ok3 := popNum(stack)
	c, ok4 := popNum(stack)
	b, ok5 := popNum(stack)
	a, ok6 := popNum(stack)
	return a, b, c, d, e, f, ok1 && ok2 && ok3 && ok4 && ok5 && ok6
}

func clusterRuns(runs []pdfRun) []pdfTextLine {
	if len(runs) == 0 {
		return nil
	}
	ordered := append([]pdfRun(nil), runs...)
	sort.SliceStable(ordered, func(i, j int) bool { return runBefore(ordered[i], ordered[j]) })
	var lines []pdfTextLine
	var buf strings.Builder
	var lineY, lineSize, lineX, endX float64
	have := false
	flush := func() {
		text := strings.TrimSpace(cleanExtracted(buf.String()))
		buf.Reset()
		have = false
		if text == "" {
			return
		}
		lines = append(lines, pdfTextLine{text: text, size: lineSize, x: lineX, y: lineY})
	}
	for _, run := range ordered {
		if !have {
			buf.WriteString(run.text)
			lineY, lineSize, lineX = run.y, run.size, run.x
			endX = run.x + estWidth(run.text, run.size)
			have = true
			continue
		}
		if math.Abs(run.y-lineY) > math.Max(2, math.Max(lineSize, run.size)*0.45) {
			flush()
			buf.WriteString(run.text)
			lineY, lineSize, lineX = run.y, run.size, run.x
			endX = run.x + estWidth(run.text, run.size)
			have = true
			continue
		}
		gap := run.x - endX
		if gap > math.Max(lineSize, run.size)*0.55 && !strings.HasSuffix(buf.String(), " ") && !strings.HasPrefix(run.text, " ") {
			buf.WriteByte(' ')
		}
		if run.size > lineSize {
			lineSize = run.size
		}
		if run.x < lineX {
			lineX = run.x
		}
		buf.WriteString(run.text)
		if next := run.x + estWidth(run.text, run.size); next > endX {
			endX = next
		}
	}
	if have {
		flush()
	}
	return lines
}

func runBefore(a, b pdfRun) bool {
	ay := math.Round(a.y)
	by := math.Round(b.y)
	if ay != by {
		return ay > by
	}
	return a.x < b.x
}

func estWidth(text string, size float64) float64 {
	width := 0.0
	for _, r := range text {
		switch {
		case r == ' ':
			width += size * 0.33
		case r > 0x2E80:
			width += size
		default:
			width += size * 0.5
		}
	}
	return width
}

func cleanExtracted(text string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, text)
}
