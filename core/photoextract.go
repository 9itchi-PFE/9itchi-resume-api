package core

import (
	"archive/zip"
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // decoding embedded PNGs
	"io"
	"path"
	"strings"
)

const (
	minPhotoSide  = 80
	maxPhotoSide  = 600
	maxEmbedBytes = 8 << 20
)

// ExtractPhoto returns the profile photo embedded in an uploaded CV ("pdf" or "docx") as a JPEG, or nil when there is none.
// A photo is the largest embedded picture that is roughly portrait or square; logos and banners are skipped.
func ExtractPhoto(format string, data []byte) (photo []byte) {
	defer func() {
		if recover() != nil {
			photo = nil
		}
	}()
	var candidates [][]byte
	switch format {
	case "pdf":
		candidates = pdfJPEGs(data)
	case "docx":
		candidates = docxImages(data)
	}
	var best image.Image
	bestArea := 0
	for _, c := range candidates {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(c))
		if err != nil || cfg.Width < minPhotoSide || cfg.Height < minPhotoSide {
			continue
		}
		ratio := float64(cfg.Width) / float64(cfg.Height)
		if ratio < 0.6 || ratio > 1.6 || cfg.Width*cfg.Height <= bestArea {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(c))
		if err != nil {
			continue
		}
		best, bestArea = img, cfg.Width*cfg.Height
	}
	if best == nil {
		return nil
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, shrink(best, maxPhotoSide), &jpeg.Options{Quality: 88}); err != nil {
		return nil
	}
	return out.Bytes()
}

// shrink scales img down (nearest neighbour is enough for a profile picture) so its longest side is at most max.
func shrink(img image.Image, max int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return flatten(img)
	}
	nw, nh := max, max*h/w
	if h > w {
		nw, nh = max*w/h, max
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			dst.Set(x, y, img.At(b.Min.X+x*w/nw, b.Min.Y+y*h/nh))
		}
	}
	return flatten(dst)
}

// flatten paints img on white so transparent PNGs do not turn black as JPEG.
func flatten(img image.Image) image.Image {
	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Over)
	return dst
}

// pdfJPEGs returns the raw DCT (JPEG) image streams of a PDF; they are stored as plain JPEG files inside the stream.
func pdfJPEGs(data []byte) [][]byte {
	var out [][]byte
	marker := []byte("/DCTDecode")
	for off := 0; ; {
		i := bytes.Index(data[off:], marker)
		if i < 0 {
			return out
		}
		pos := off + i + len(marker)
		s := bytes.Index(data[pos:], []byte("stream"))
		if s < 0 {
			return out
		}
		start := pos + s + len("stream")
		if start < len(data) && data[start] == '\r' {
			start++
		}
		if start < len(data) && data[start] == '\n' {
			start++
		}
		e := bytes.Index(data[start:], []byte("endstream"))
		if e < 0 {
			return out
		}
		img := bytes.TrimRight(data[start:start+e], "\r\n ")
		if bytes.HasPrefix(img, []byte{0xFF, 0xD8}) && len(img) <= maxEmbedBytes {
			out = append(out, img)
		}
		off = start + e
	}
}

// docxImages returns the pictures stored under word/media.
func docxImages(data []byte) [][]byte {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	var out [][]byte
	for _, f := range zr.File {
		ext := strings.ToLower(path.Ext(f.Name))
		if !strings.HasPrefix(f.Name, "word/media/") || (ext != ".jpg" && ext != ".jpeg" && ext != ".png") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxEmbedBytes))
		rc.Close()
		if err == nil {
			out = append(out, b)
		}
	}
	return out
}
