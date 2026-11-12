package document

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"strings"

	apperrors "yunjian/internal/platform/errors"
)

const (
	maxPDFImageBytes = 240 << 10
	maxPDFImageTotal = 480 << 10
	jpegDataPrefix   = "data:image/jpeg;base64,"
)

type pdfPlacedImage struct {
	x, y float64
	w, h float64
	data []byte
}

func imageFromPDFStream(obj pdfObj, raw []byte) []byte {
	names := filterNames(obj)
	switch {
	case len(names) == 1 && (names[0] == "DCTDecode" || names[0] == "DCT"):
		return fitJPEG(raw)
	case len(names) == 0:
		return samplesToJPEG(obj, raw)
	case len(names) == 1 && (names[0] == "FlateDecode" || names[0] == "Fl"):
		data, err := inflatePDF(raw)
		if err != nil {
			return nil
		}
		return samplesToJPEG(obj, data)
	default:
		return nil
	}
}

func officeImageDataURL(raw []byte) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	if jpegBytes := fitJPEG(raw); len(jpegBytes) > 0 {
		return jpegDataPrefix + base64.StdEncoding.EncodeToString(jpegBytes), true
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8000 || cfg.Height > 8000 {
		return "", false
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", false
	}
	jpegBytes := shrinkJPEG(img)
	if len(jpegBytes) == 0 {
		return "", false
	}
	return jpegDataPrefix + base64.StdEncoding.EncodeToString(jpegBytes), true
}

func fitJPEG(raw []byte) []byte {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8000 || cfg.Height > 8000 {
		return nil
	}
	// CMYK JPEG 的字节不能配 /DeviceRGB。先解码再写成 RGB，浏览器和导出才是同一张图。
	if cfg.ColorModel != color.CMYKModel && len(raw) <= maxPDFImageBytes {
		return append([]byte(nil), raw...)
	}
	img, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	return shrinkJPEG(img)
}

func samplesToJPEG(obj pdfObj, data []byte) []byte {
	width := dictInt(obj, "Width")
	height := dictInt(obj, "Height")
	if width < 1 || height < 1 || width > 8000 || height > 8000 {
		return nil
	}
	bpc := dictInt(obj, "BitsPerComponent")
	if bpc == 0 {
		bpc = 8
	}
	colors, ok := imageColors(obj)
	if !ok || (bpc != 8 && bpc != 1) {
		return nil
	}
	samples, ok := undoPrediction(obj, data, width, height, colors, bpc)
	if !ok {
		return nil
	}
	img := imageFromSamples(samples, width, height, colors, bpc)
	if img == nil {
		return nil
	}
	return shrinkJPEG(img)
}

func imageColors(obj pdfObj) (int, bool) {
	switch obj.dict["ColorSpace"].text {
	case "DeviceGray", "G":
		return 1, true
	case "DeviceRGB", "RGB":
		return 3, true
	default:
		return 0, false
	}
}

func dictInt(obj pdfObj, key string) int {
	value := obj.dict[key]
	if value.kind != objNum {
		return 0
	}
	return int(value.num)
}

func undoPrediction(obj pdfObj, data []byte, width, height, colors, bpc int) ([]byte, bool) {
	columns := width
	predictor := 1
	if parms := obj.dict["DecodeParms"]; parms.kind == objDict {
		if n := dictInt(parms, "Columns"); n > 0 {
			columns = n
		}
		if n := dictInt(parms, "Predictor"); n > 0 {
			predictor = n
		}
		if n := dictInt(parms, "Colors"); n == 1 || n == 3 {
			colors = n
		}
		if n := dictInt(parms, "BitsPerComponent"); n == 1 || n == 8 {
			bpc = n
		}
	}
	rowBytes := (columns*colors*bpc + 7) / 8
	if rowBytes < 1 || height > (1<<28)/rowBytes {
		return nil, false
	}
	if predictor <= 1 {
		need := height * rowBytes
		if len(data) < need {
			return nil, false
		}
		return data[:need], true
	}
	if predictor == 2 && bpc == 8 {
		return tiffPredict(data, height, rowBytes, colors)
	}
	if predictor >= 10 && predictor <= 15 && bpc == 8 {
		return pngPredict(data, height, rowBytes, colors)
	}
	return nil, false
}

func tiffPredict(data []byte, height, rowBytes, colors int) ([]byte, bool) {
	if colors < 1 || len(data) < height*rowBytes {
		return nil, false
	}
	out := append([]byte(nil), data[:height*rowBytes]...)
	for y := 0; y < height; y++ {
		row := out[y*rowBytes : (y+1)*rowBytes]
		for i := colors; i < len(row); i++ {
			row[i] += row[i-colors]
		}
	}
	return out, true
}

func pngPredict(data []byte, height, rowBytes, colors int) ([]byte, bool) {
	bpp := colors
	if bpp < 1 {
		bpp = 1
	}
	out := make([]byte, height*rowBytes)
	prev := make([]byte, rowBytes)
	at := 0
	for y := 0; y < height; y++ {
		if at >= len(data) {
			return nil, false
		}
		filter := data[at]
		at++
		if at+rowBytes > len(data) {
			return nil, false
		}
		row := data[at : at+rowBytes]
		at += rowBytes
		dst := out[y*rowBytes : (y+1)*rowBytes]
		if !applyPNGFilter(filter, row, dst, prev, bpp) {
			return nil, false
		}
		copy(prev, dst)
	}
	return out, true
}

func applyPNGFilter(filter byte, row, dst, prev []byte, bpp int) bool {
	switch filter {
	case 0:
		copy(dst, row)
	case 1:
		for i := range row {
			left := byte(0)
			if i >= bpp {
				left = dst[i-bpp]
			}
			dst[i] = row[i] + left
		}
	case 2:
		for i := range row {
			dst[i] = row[i] + prev[i]
		}
	case 3:
		for i := range row {
			left := byte(0)
			if i >= bpp {
				left = dst[i-bpp]
			}
			dst[i] = row[i] + byte((int(left)+int(prev[i]))/2)
		}
	case 4:
		for i := range row {
			left := byte(0)
			upLeft := byte(0)
			if i >= bpp {
				left = dst[i-bpp]
				upLeft = prev[i-bpp]
			}
			dst[i] = row[i] + paeth(left, prev[i], upLeft)
		}
	default:
		return false
	}
	return true
}

func paeth(left, up, upLeft byte) byte {
	a, b, c := int(left), int(up), int(upLeft)
	p := a + b - c
	pa, pb, pc := absInt(p-a), absInt(p-b), absInt(p-c)
	switch {
	case pa <= pb && pa <= pc:
		return left
	case pb <= pc:
		return up
	default:
		return upLeft
	}
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func imageFromSamples(samples []byte, width, height, colors, bpc int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	if bpc == 8 && colors == 1 {
		if len(samples) < width*height {
			return nil
		}
		for i := 0; i < width*height; i++ {
			v := samples[i]
			img.SetRGBA(i%width, i/width, color.RGBA{v, v, v, 255})
		}
		return img
	}
	if bpc == 8 && colors == 3 {
		if len(samples) < width*height*3 {
			return nil
		}
		for i := 0; i < width*height; i++ {
			img.SetRGBA(i%width, i/width, color.RGBA{samples[i*3], samples[i*3+1], samples[i*3+2], 255})
		}
		return img
	}
	if bpc == 1 && colors == 1 {
		rowBytes := (width + 7) / 8
		if len(samples) < rowBytes*height {
			return nil
		}
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				bit := samples[y*rowBytes+x/8] >> (7 - (x % 8)) & 1
				v := byte(0)
				if bit == 1 {
					v = 255
				}
				img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
			}
		}
		return img
	}
	return nil
}

func onWhite(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)
	return dst
}

func shrinkJPEG(img image.Image) []byte {
	var current image.Image = onWhite(img)
	for step := 0; step < 6; step++ {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, current, &jpeg.Options{Quality: 75}); err != nil {
			return nil
		}
		if buf.Len() <= maxPDFImageBytes {
			return buf.Bytes()
		}
		bounds := current.Bounds()
		if bounds.Dx() < 64 || bounds.Dy() < 64 {
			return nil
		}
		current = downscale(current)
	}
	return nil
}

func downscale(src image.Image) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx() / 2
	height := bounds.Dy() / 2
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dst.Set(x, y, src.At(bounds.Min.X+x*2, bounds.Min.Y+y*2))
		}
	}
	return dst
}

func (in *pdfInterp) placeImage(obj pdfObj) error {
	if len(obj.image) == 0 || in.doc.imageBytes+len(obj.image) > maxPDFImageTotal {
		return nil
	}
	x0, y0 := in.ctm.pt(0, 0)
	x1, y1 := in.ctm.pt(1, 0)
	x2, y2 := in.ctm.pt(0, 1)
	_, y3 := in.ctm.pt(1, 1)
	placed := pdfPlacedImage{
		x:    x0,
		y:    math.Max(math.Max(y0, y1), math.Max(y2, y3)),
		w:    math.Hypot(x1-x0, y1-y0),
		h:    math.Hypot(x2-x0, y2-y0),
		data: obj.image,
	}
	if placed.w < 2 || placed.h < 2 {
		return nil
	}
	in.doc.imageBytes += len(placed.data)
	in.images = append(in.images, placed)
	return nil
}

func blockFromPDFImage(image pdfPlacedImage) (wordBlock, bool) {
	if len(image.data) == 0 {
		return wordBlock{}, false
	}
	return wordBlock{
		Type:  "image",
		Align: "left",
		Runs:  []wordRun{{}},
		Image: jpegDataPrefix + base64.StdEncoding.EncodeToString(image.data),
	}, true
}

func validateWordImage(value string) error {
	if !strings.HasPrefix(value, jpegDataPrefix) {
		return apperrors.BadRequest(400, "图片内容无效")
	}
	raw, err := base64.StdEncoding.DecodeString(value[len(jpegDataPrefix):])
	if err != nil || len(raw) == 0 || len(raw) > maxPDFImageBytes {
		return apperrors.BadRequest(400, "图片内容无效")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return apperrors.BadRequest(400, "图片内容无效")
	}
	return nil
}

func jpegPayload(value string) ([]byte, bool) {
	if !strings.HasPrefix(value, jpegDataPrefix) {
		return nil, false
	}
	raw, err := base64.StdEncoding.DecodeString(value[len(jpegDataPrefix):])
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

func jpegInfo(data []byte) (int, int, string, bool) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return 0, 0, "", false
	}
	space := "/DeviceRGB"
	if cfg.ColorModel == color.GrayModel {
		space = "/DeviceGray"
	}
	return cfg.Width, cfg.Height, space, true
}
