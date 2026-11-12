package document

import (
	"encoding/binary"
	"os"
	"sort"
	"sync"

	apperrors "yunjian/internal/platform/errors"
)

var errPDFFont = apperrors.BadRequest(400, "服务器缺少中文字体，无法导出 PDF")

// pdfSubset 是按本次正文裁过的 TrueType，字形编号从 1 开始，0 是 .notdef。
type pdfSubset struct {
	data    []byte
	units   int
	ascent  int
	descent int
	xMin    int
	yMin    int
	xMax    int
	yMax    int
	glyph   map[rune]uint16
	adv     map[rune]int
	widths  []int
}

type ttfFont struct {
	path     string
	num      int
	units    int
	ascent   int
	descent  int
	xMin     int
	yMin     int
	xMax     int
	yMax     int
	nMetrics int
	loca     []int
	glyf     []byte
	hmtx     []byte
	head     []byte
	hhea     []byte
	maxp     []byte
	os2      []byte
	fpgm     []byte
	cvt      []byte
	prep     []byte
	seg      []ttfSeg4
	garr     []byte
	groups   []ttfGroup
}

type ttfSeg4 struct {
	start  uint16
	end    uint16
	delta  int16
	offset uint16
	index  int
}

type ttfGroup struct {
	start int
	end   int
	glyph int
}

var (
	pdfFaceOnce sync.Once
	pdfFace     *ttfFont
	pdfFaceErr  error
)

func loadPDFFace() (*ttfFont, error) {
	pdfFaceOnce.Do(func() {
		pdfFace, pdfFaceErr = choosePDFFace()
	})
	return pdfFace, pdfFaceErr
}

func choosePDFFace() (*ttfFont, error) {
	paths := []string{
		os.Getenv("YUNJIAN_PDF_FONT"),
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/System/Library/Fonts/PingFang.ttc",
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/croscore/Arimo-Regular.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	}
	var best *ttfFont
	bestScore := 0
	for _, path := range paths {
		if path == "" {
			continue
		}
		face, err := openTTF(path)
		if err != nil {
			continue
		}
		score := face.score()
		if score > bestScore {
			best = face
			bestScore = score
		}
	}
	if best == nil || bestScore < 2 {
		return nil, errPDFFont
	}
	return best, nil
}

func openTTF(path string) (*ttfFont, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || len(data) > 20<<20 {
		return nil, errPDFFont
	}
	bases := []int{0}
	if string(data[:4]) == "ttcf" {
		count := int(binary.BigEndian.Uint32(data[8:12]))
		if count < 1 || 12+4*count > len(data) {
			return nil, errPDFFont
		}
		bases = bases[:0]
		if count > 4 {
			count = 4
		}
		for i := 0; i < count; i++ {
			bases = append(bases, int(binary.BigEndian.Uint32(data[12+4*i:])))
		}
	}
	var best *ttfFont
	bestScore := -1
	for _, base := range bases {
		face, err := parseTTF(data, base)
		if err != nil {
			continue
		}
		face.path = path
		score := face.score()
		if score > bestScore {
			best = face
			bestScore = score
		}
	}
	if best == nil {
		return nil, errPDFFont
	}
	return best, nil
}

func parseTTF(data []byte, base int) (*ttfFont, error) {
	if base < 0 || base+12 > len(data) {
		return nil, errPDFFont
	}
	version := binary.BigEndian.Uint32(data[base:])
	if version != 0x00010000 && version != 0x74727565 {
		return nil, errPDFFont
	}
	count := int(binary.BigEndian.Uint16(data[base+4:]))
	if count <= 0 || base+12+count*16 > len(data) {
		return nil, errPDFFont
	}
	tables := map[string][]byte{}
	for i := 0; i < count; i++ {
		rec := data[base+12+i*16:]
		tag := string(rec[:4])
		off := int(binary.BigEndian.Uint32(rec[8:]))
		size := int(binary.BigEndian.Uint32(rec[12:]))
		if off < 0 || size < 0 || off+size > len(data) {
			return nil, errPDFFont
		}
		tables[tag] = data[off : off+size]
	}
	head := tables["head"]
	maxp := tables["maxp"]
	loca := tables["loca"]
	glyf := tables["glyf"]
	hhea := tables["hhea"]
	hmtx := tables["hmtx"]
	cmap := tables["cmap"]
	if len(head) < 54 || len(maxp) < 6 || len(hhea) < 36 || len(loca) < 4 || len(cmap) < 4 {
		return nil, errPDFFont
	}
	num := int(binary.BigEndian.Uint16(maxp[4:]))
	format := int(int16(binary.BigEndian.Uint16(head[50:])))
	locaCount := len(loca) / 4
	if format == 0 {
		locaCount = len(loca) / 2
	}
	if num+1 > locaCount {
		num = locaCount - 1
	}
	if num <= 0 {
		return nil, errPDFFont
	}
	face := &ttfFont{
		num:      num,
		units:    int(binary.BigEndian.Uint16(head[18:])),
		xMin:     int(int16(binary.BigEndian.Uint16(head[36:]))),
		yMin:     int(int16(binary.BigEndian.Uint16(head[38:]))),
		xMax:     int(int16(binary.BigEndian.Uint16(head[40:]))),
		yMax:     int(int16(binary.BigEndian.Uint16(head[42:]))),
		ascent:   int(int16(binary.BigEndian.Uint16(hhea[4:]))),
		descent:  int(int16(binary.BigEndian.Uint16(hhea[6:]))),
		nMetrics: int(binary.BigEndian.Uint16(hhea[34:])),
		glyf:     glyf,
		hmtx:     hmtx,
		head:     head,
		hhea:     hhea,
		maxp:     maxp,
		os2:      append([]byte{}, tables["OS/2"]...),
		fpgm:     append([]byte{}, tables["fpgm"]...),
		cvt:      append([]byte{}, tables["cvt "]...),
		prep:     append([]byte{}, tables["prep"]...),
	}
	if face.units == 0 {
		face.units = 2048
	}
	if face.nMetrics < 1 || face.nMetrics > num {
		face.nMetrics = num
	}
	face.loca = make([]int, num+1)
	for i := 0; i <= num; i++ {
		if format == 0 {
			face.loca[i] = int(binary.BigEndian.Uint16(loca[i*2:])) * 2
		} else {
			face.loca[i] = int(binary.BigEndian.Uint32(loca[i*4:]))
		}
	}
	face.parseCmap(cmap)
	return face, nil
}

func (f *ttfFont) parseCmap(cmap []byte) {
	if len(cmap) < 4 {
		return
	}
	count := int(binary.BigEndian.Uint16(cmap[2:]))
	var seg []ttfSeg4
	var garr []byte
	var groups []ttfGroup
	for i := 0; i < count; i++ {
		if 4+i*8+8 > len(cmap) {
			return
		}
		platform := binary.BigEndian.Uint16(cmap[4+i*8:])
		off := int(binary.BigEndian.Uint32(cmap[8+i*8:]))
		if off < 0 || off+2 > len(cmap) || (platform != 0 && platform != 3) {
			continue
		}
		format := binary.BigEndian.Uint16(cmap[off:])
		if format == 4 && seg == nil {
			seg, garr = parseCmap4(cmap[off:])
		}
		if format == 12 && groups == nil {
			groups = parseCmap12(cmap[off:])
		}
	}
	f.seg = seg
	f.garr = garr
	f.groups = groups
}

func parseCmap4(data []byte) ([]ttfSeg4, []byte) {
	if len(data) < 16 {
		return nil, nil
	}
	length := int(binary.BigEndian.Uint16(data[2:]))
	if length > len(data) {
		length = len(data)
	}
	segCount := int(binary.BigEndian.Uint16(data[6:])) / 2
	if segCount <= 0 || 16+8*segCount > length {
		return nil, nil
	}
	segs := make([]ttfSeg4, segCount)
	for i := 0; i < segCount; i++ {
		segs[i] = ttfSeg4{
			end:    binary.BigEndian.Uint16(data[14+2*i:]),
			start:  binary.BigEndian.Uint16(data[16+2*segCount+2*i:]),
			delta:  int16(binary.BigEndian.Uint16(data[16+4*segCount+2*i:])),
			offset: binary.BigEndian.Uint16(data[16+6*segCount+2*i:]),
			index:  i,
		}
	}
	arrayAt := 16 + 8*segCount
	return segs, append([]byte{}, data[arrayAt:length]...)
}

func parseCmap12(data []byte) []ttfGroup {
	if len(data) < 16 {
		return nil
	}
	count := int(binary.BigEndian.Uint32(data[12:]))
	if count < 0 || 16+count*12 > len(data) {
		return nil
	}
	groups := make([]ttfGroup, 0, count)
	for i := 0; i < count; i++ {
		at := 16 + i*12
		groups = append(groups, ttfGroup{
			start: int(binary.BigEndian.Uint32(data[at:])),
			end:   int(binary.BigEndian.Uint32(data[at+4:])),
			glyph: int(binary.BigEndian.Uint32(data[at+8:])),
		})
	}
	return groups
}

func (f *ttfFont) glyphID(r rune) int {
	if r < 0 {
		return 0
	}
	if id, ok := f.lookup12(int(r)); ok && id > 0 && id < f.num {
		return id
	}
	if id, ok := f.lookup4(r); ok && id > 0 && id < f.num {
		return id
	}
	return 0
}

func (f *ttfFont) lookup12(code int) (int, bool) {
	i := sort.Search(len(f.groups), func(i int) bool { return f.groups[i].end >= code })
	if i >= len(f.groups) || code < f.groups[i].start {
		return 0, false
	}
	return f.groups[i].glyph + code - f.groups[i].start, true
}

func (f *ttfFont) lookup4(r rune) (int, bool) {
	if r > 0xFFFF || len(f.seg) == 0 {
		return 0, false
	}
	code := uint16(r)
	i := sort.Search(len(f.seg), func(i int) bool { return f.seg[i].end >= code })
	if i >= len(f.seg) || code < f.seg[i].start {
		return 0, false
	}
	seg := f.seg[i]
	if seg.offset == 0 {
		return int((int(code) + int(seg.delta)) & 0xFFFF), true
	}
	pos := seg.index + int(seg.offset)/2 + int(code-seg.start) - len(f.seg)
	if pos < 0 || pos*2+2 > len(f.garr) {
		return 0, false
	}
	return int(binary.BigEndian.Uint16(f.garr[pos*2:])), true
}

func (f *ttfFont) glyphData(id int) []byte {
	if id < 0 || id+1 >= len(f.loca) {
		return nil
	}
	start, end := f.loca[id], f.loca[id+1]
	if start < 0 || end < start || end > len(f.glyf) {
		return nil
	}
	return f.glyf[start:end]
}

func (f *ttfFont) advanceOf(id int) int {
	if len(f.hmtx) < 4 {
		return f.units
	}
	slot := id
	if slot >= f.nMetrics {
		slot = f.nMetrics - 1
	}
	if slot < 0 || slot*4+2 > len(f.hmtx) {
		return f.units
	}
	return int(binary.BigEndian.Uint16(f.hmtx[slot*4:]))
}

func (f *ttfFont) lsbOf(id int) int {
	if id >= 0 && id < f.nMetrics && id*4+4 <= len(f.hmtx) {
		return int(int16(binary.BigEndian.Uint16(f.hmtx[id*4+2:])))
	}
	off := f.nMetrics*4 + (id-f.nMetrics)*2
	if id >= f.nMetrics && off >= 0 && off+2 <= len(f.hmtx) {
		return int(int16(binary.BigEndian.Uint16(f.hmtx[off:])))
	}
	return 0
}

func (f *ttfFont) score() int {
	latin := f.covered('A') && f.covered('H')
	cjk := f.covered('你')
	switch {
	case latin && cjk:
		return 10
	case cjk:
		return 4
	case latin:
		return 2
	default:
		return 0
	}
}

func (f *ttfFont) covered(r rune) bool {
	return len(f.glyphData(f.glyphID(r))) > 8
}

func (f *ttfFont) subset(chars []rune) (*pdfSubset, error) {
	seen := map[rune]bool{}
	runes := make([]rune, 0, len(chars)+1)
	add := func(r rune) {
		if r < 0 || seen[r] || f.glyphID(r) == 0 {
			return
		}
		seen[r] = true
		runes = append(runes, r)
	}
	add(' ')
	for _, r := range chars {
		add(r)
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })

	idOf := map[int]int{0: 0}
	oldIDs := []int{0}
	queue := []int{0}
	include := func(old int) {
		if old < 0 || old >= f.num {
			return
		}
		if _, ok := idOf[old]; ok {
			return
		}
		idOf[old] = len(oldIDs)
		oldIDs = append(oldIDs, old)
		queue = append(queue, old)
	}
	for _, r := range runes {
		include(f.glyphID(r))
	}
	for i := 0; i < len(queue); i++ {
		for _, comp := range componentIDs(f.glyphData(queue[i])) {
			include(comp)
		}
	}

	glyf := make([]byte, 0, 4096)
	loca := make([]int, len(oldIDs)+1)
	hmtx := make([]byte, len(oldIDs)*4)
	widths := make([]int, len(oldIDs))
	maxAdv := 0
	for i, old := range oldIDs {
		loca[i] = len(glyf)
		data := append([]byte{}, f.glyphData(old)...)
		data = remapComposite(data, idOf)
		if len(data)%2 == 1 {
			data = append(data, 0)
		}
		glyf = append(glyf, data...)
		adv := f.advanceOf(old)
		if adv == 0 {
			adv = f.units / 3
		}
		if adv > maxAdv {
			maxAdv = adv
		}
		widths[i] = adv * 1000 / f.units
		binary.BigEndian.PutUint16(hmtx[i*4:], uint16(adv))
		binary.BigEndian.PutUint16(hmtx[i*4+2:], uint16(int16(f.lsbOf(old))))
	}
	loca[len(oldIDs)] = len(glyf)

	glyph := make(map[rune]uint16, len(runes))
	advances := make(map[rune]int, len(runes))
	for _, r := range runes {
		glyph[r] = uint16(idOf[f.glyphID(r)])
		advances[r] = f.advanceOf(f.glyphID(r))
		if advances[r] == 0 {
			advances[r] = f.units / 3
		}
	}
	tables := map[string][]byte{
		"cmap": buildCmap(runes, glyph),
		"glyf": glyf,
		"head": subsetHead(f.head, len(oldIDs)),
		"hhea": subsetHhea(f.hhea, len(oldIDs), maxAdv),
		"hmtx": hmtx,
		"loca": subsetLoca(loca),
		"maxp": subsetMaxp(f.maxp, len(oldIDs)),
		"name": subsetName(),
		"post": subsetPost(),
	}
	if len(f.os2) >= 64 && len(f.os2) <= 128 {
		os2 := append([]byte{}, f.os2...)
		binary.BigEndian.PutUint16(os2[8:], 0)
		tables["OS/2"] = os2
	}
	if len(f.fpgm) > 0 && len(f.fpgm) < 64<<10 {
		tables["fpgm"] = f.fpgm
	}
	if len(f.cvt) > 0 && len(f.cvt) < 64<<10 {
		tables["cvt "] = f.cvt
	}
	if len(f.prep) > 0 && len(f.prep) < 64<<10 {
		tables["prep"] = f.prep
	}
	return &pdfSubset{
		data:    assembleTTF(tables),
		units:   f.units,
		ascent:  f.ascent,
		descent: f.descent,
		xMin:    f.xMin,
		yMin:    f.yMin,
		xMax:    f.xMax,
		yMax:    f.yMax,
		glyph:   glyph,
		adv:     advances,
		widths:  widths,
	}, nil
}

func componentIDs(data []byte) []int {
	if len(data) < 10 || int16(binary.BigEndian.Uint16(data[:2])) >= 0 {
		return nil
	}
	var ids []int
	i := 10
	for i+4 <= len(data) {
		flags := binary.BigEndian.Uint16(data[i:])
		ids = append(ids, int(binary.BigEndian.Uint16(data[i+2:])))
		i += 4
		if flags&1 != 0 {
			i += 4
		} else {
			i += 2
		}
		switch {
		case flags&8 != 0:
			i += 2
		case flags&64 != 0:
			i += 4
		case flags&128 != 0:
			i += 8
		}
		if flags&32 == 0 {
			break
		}
	}
	return ids
}

func remapComposite(data []byte, idOf map[int]int) []byte {
	if len(data) < 10 || int16(binary.BigEndian.Uint16(data[:2])) >= 0 {
		return data
	}
	i := 10
	for i+4 <= len(data) {
		flags := binary.BigEndian.Uint16(data[i:])
		old := int(binary.BigEndian.Uint16(data[i+2:]))
		neu := uint16(0)
		if id, ok := idOf[old]; ok && id >= 0 && id <= 0xFFFF {
			neu = uint16(id)
		}
		binary.BigEndian.PutUint16(data[i+2:], neu)
		i += 4
		if flags&1 != 0 {
			i += 4
		} else {
			i += 2
		}
		switch {
		case flags&8 != 0:
			i += 2
		case flags&64 != 0:
			i += 4
		case flags&128 != 0:
			i += 8
		}
		if flags&32 == 0 {
			break
		}
	}
	return data
}

func subsetHead(src []byte, glyphs int) []byte {
	head := append([]byte{}, src[:54]...)
	binary.BigEndian.PutUint32(head[8:], 0)
	binary.BigEndian.PutUint16(head[50:], 1)
	_ = glyphs
	return head
}

func subsetHhea(src []byte, glyphs, maxAdv int) []byte {
	hhea := append([]byte{}, src[:36]...)
	if maxAdv > 0xFFFF {
		maxAdv = 0xFFFF
	}
	binary.BigEndian.PutUint16(hhea[10:], uint16(maxAdv))
	binary.BigEndian.PutUint16(hhea[34:], uint16(glyphs))
	return hhea
}

func subsetLoca(loca []int) []byte {
	out := make([]byte, len(loca)*4)
	for i, off := range loca {
		binary.BigEndian.PutUint32(out[i*4:], uint32(off))
	}
	return out
}

func subsetMaxp(src []byte, glyphs int) []byte {
	maxp := make([]byte, 32)
	if len(src) >= 32 && binary.BigEndian.Uint32(src) == 0x00010000 {
		copy(maxp, src[:32])
	} else {
		binary.BigEndian.PutUint32(maxp, 0x00010000)
		binary.BigEndian.PutUint16(maxp[14:], 2)
	}
	binary.BigEndian.PutUint16(maxp[4:], uint16(glyphs))
	return maxp
}

func subsetName() []byte {
	text := utf16BE("Yunjian")
	ids := []int{1, 4, 6}
	out := make([]byte, 6+12*len(ids)+len(text))
	binary.BigEndian.PutUint16(out[2:], uint16(len(ids)))
	binary.BigEndian.PutUint16(out[4:], uint16(6+12*len(ids)))
	for i, id := range ids {
		at := 6 + i*12
		binary.BigEndian.PutUint16(out[at:], 3)
		binary.BigEndian.PutUint16(out[at+2:], 1)
		binary.BigEndian.PutUint16(out[at+4:], 0x0409)
		binary.BigEndian.PutUint16(out[at+6:], uint16(id))
		binary.BigEndian.PutUint16(out[at+8:], uint16(len(text)))
	}
	copy(out[6+12*len(ids):], text)
	return out
}

func utf16BE(value string) []byte {
	out := make([]byte, 0, len(value)*2)
	for _, r := range value {
		if r > 0xFFFF {
			r -= 0x10000
			hi := uint16(0xD800 + (r >> 10))
			lo := uint16(0xDC00 + (r & 0x3FF))
			out = append(out, byte(hi>>8), byte(hi), byte(lo>>8), byte(lo))
			continue
		}
		out = append(out, byte(r>>8), byte(r))
	}
	return out
}

func subsetPost() []byte {
	out := make([]byte, 32)
	binary.BigEndian.PutUint32(out, 0x00030000)
	underline := int16(-100)
	binary.BigEndian.PutUint16(out[8:], uint16(underline))
	binary.BigEndian.PutUint16(out[10:], 50)
	return out
}

func buildCmap(runes []rune, glyph map[rune]uint16) []byte {
	bmp := make([]rune, 0, len(runes))
	for _, r := range runes {
		if r <= 0xFFFF {
			bmp = append(bmp, r)
		}
	}
	format4 := buildFormat4(bmp, glyph)
	format12 := buildFormat12(runes, glyph)
	count := 3
	header := 4 + count*8
	out := make([]byte, header+len(format4)+len(format12))
	binary.BigEndian.PutUint16(out[2:], uint16(count))
	// platform 0 encoding 4 and platform 3 encoding 10 share the full repertoire.
	writeCmapRec(out, 0, 0, 4, header+len(format4))
	writeCmapRec(out, 1, 3, 1, header)
	writeCmapRec(out, 2, 3, 10, header+len(format4))
	copy(out[header:], format4)
	copy(out[header+len(format4):], format12)
	return out
}

func writeCmapRec(out []byte, index, platform, encoding, offset int) {
	at := 4 + index*8
	binary.BigEndian.PutUint16(out[at:], uint16(platform))
	binary.BigEndian.PutUint16(out[at+2:], uint16(encoding))
	binary.BigEndian.PutUint32(out[at+4:], uint32(offset))
}

func buildFormat4(runes []rune, glyph map[rune]uint16) []byte {
	// One segment per character keeps the glyph-id lookup independent of idDelta overflow.
	tooBig := 16+8*(len(runes)+1)+2*len(runes) > 65535
	if tooBig {
		runes = nil
	}
	segCount := len(runes) + 1
	length := 16 + 8*segCount + 2*len(runes)
	out := make([]byte, length)
	binary.BigEndian.PutUint16(out, 4)
	binary.BigEndian.PutUint16(out[2:], uint16(length))
	binary.BigEndian.PutUint16(out[6:], uint16(segCount*2))
	pow, selector := 1, 0
	for pow*2 <= segCount {
		pow *= 2
		selector++
	}
	binary.BigEndian.PutUint16(out[8:], uint16(pow*2))
	binary.BigEndian.PutUint16(out[10:], uint16(selector))
	binary.BigEndian.PutUint16(out[12:], uint16(2*segCount-pow*2))
	endAt := 14
	startAt := 16 + 2*segCount
	deltaAt := 16 + 4*segCount
	offsetAt := 16 + 6*segCount
	glyphAt := 16 + 8*segCount
	for i, r := range runes {
		binary.BigEndian.PutUint16(out[endAt+2*i:], uint16(r))
		binary.BigEndian.PutUint16(out[startAt+2*i:], uint16(r))
		binary.BigEndian.PutUint16(out[offsetAt+2*i:], uint16(2*segCount))
		binary.BigEndian.PutUint16(out[glyphAt+2*i:], glyph[r])
	}
	last := segCount - 1
	binary.BigEndian.PutUint16(out[endAt+2*last:], 0xFFFF)
	binary.BigEndian.PutUint16(out[startAt+2*last:], 0xFFFF)
	binary.BigEndian.PutUint16(out[deltaAt+2*last:], 1)
	return out
}

func buildFormat12(runes []rune, glyph map[rune]uint16) []byte {
	type group struct{ start, end, glyph int }
	var groups []group
	for _, r := range runes {
		gid := int(glyph[r])
		if len(groups) > 0 {
			last := &groups[len(groups)-1]
			if int(r) == last.end+1 && gid == last.glyph+(last.end-last.start)+1 {
				last.end = int(r)
				continue
			}
		}
		groups = append(groups, group{start: int(r), end: int(r), glyph: gid})
	}
	out := make([]byte, 16+len(groups)*12)
	binary.BigEndian.PutUint16(out, 12)
	binary.BigEndian.PutUint32(out[4:], uint32(len(out)))
	binary.BigEndian.PutUint32(out[12:], uint32(len(groups)))
	for i, group := range groups {
		at := 16 + i*12
		binary.BigEndian.PutUint32(out[at:], uint32(group.start))
		binary.BigEndian.PutUint32(out[at+4:], uint32(group.end))
		binary.BigEndian.PutUint32(out[at+8:], uint32(group.glyph))
	}
	return out
}

func assembleTTF(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for tag, data := range tables {
		if len(tag) == 4 && data != nil {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	count := len(tags)
	offset := 12 + count*16
	type rec struct {
		tag  string
		data []byte
		off  int
		sum  uint32
	}
	recs := make([]rec, 0, count)
	for _, tag := range tags {
		if offset%4 != 0 {
			offset += 4 - offset%4
		}
		data := tables[tag]
		recs = append(recs, rec{tag: tag, data: data, off: offset, sum: tableChecksum(data)})
		offset += len(data)
	}
	file := make([]byte, offset)
	binary.BigEndian.PutUint32(file, 0x00010000)
	binary.BigEndian.PutUint16(file[4:], uint16(count))
	pow, selector := 1, 0
	for pow*2 <= count {
		pow *= 2
		selector++
	}
	binary.BigEndian.PutUint16(file[6:], uint16(pow*16))
	binary.BigEndian.PutUint16(file[8:], uint16(selector))
	binary.BigEndian.PutUint16(file[10:], uint16(count*16-pow*16))
	for i, rec := range recs {
		at := 12 + i*16
		copy(file[at:], rec.tag)
		binary.BigEndian.PutUint32(file[at+4:], rec.sum)
		binary.BigEndian.PutUint32(file[at+8:], uint32(rec.off))
		binary.BigEndian.PutUint32(file[at+12:], uint32(len(rec.data)))
		copy(file[rec.off:], rec.data)
	}
	headOff := 0
	for _, rec := range recs {
		if rec.tag == "head" {
			headOff = rec.off
		}
	}
	sum := tableChecksum(file)
	binary.BigEndian.PutUint32(file[headOff+8:], 0xB1B0AFBA-sum)
	return file
}

func tableChecksum(data []byte) uint32 {
	pad := (4 - len(data)%4) % 4
	buf := data
	if pad != 0 {
		buf = append(append([]byte{}, data...), make([]byte, pad)...)
	}
	var sum uint32
	for i := 0; i < len(buf); i += 4 {
		sum += binary.BigEndian.Uint32(buf[i:])
	}
	return sum
}
