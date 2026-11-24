package core

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func solidJPEG(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{200, 120, 40, 255})
		}
	}
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, img, nil))
	return b.Bytes()
}

func TestExtractPhotoFromPDF(t *testing.T) {
	banner, face := solidJPEG(t, 800, 100), solidJPEG(t, 300, 360)
	var pdf bytes.Buffer
	for _, img := range [][]byte{banner, face} {
		pdf.WriteString("<< /Subtype /Image /Filter /DCTDecode /Length 1 >>\nstream\n")
		pdf.Write(img)
		pdf.WriteString("\nendstream\nendobj\n")
	}
	got := ExtractPhoto("pdf", pdf.Bytes())
	require.NotNil(t, got)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(got))
	require.NoError(t, err)
	assert.Equal(t, 300, cfg.Width, "the portrait picture wins over the banner")
	assert.Nil(t, ExtractPhoto("pdf", []byte("%PDF no images")))
}

func TestExtractPhotoFromDocx(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("word/media/image1.jpeg")
	_, _ = w.Write(solidJPEG(t, 1200, 1200))
	require.NoError(t, zw.Close())
	got := ExtractPhoto("docx", buf.Bytes())
	require.NotNil(t, got)
	cfg, _, _ := image.DecodeConfig(bytes.NewReader(got))
	assert.Equal(t, maxPhotoSide, cfg.Width, "large photos are scaled down")
}

func TestExtractPhotoFromGeneratedPDF(t *testing.T) {
	p := fpdf.New("P", "mm", "A4", "")
	p.AddPage()
	p.RegisterImageOptionsReader("me", fpdf.ImageOptions{ImageType: "JPEG"}, bytes.NewReader(solidJPEG(t, 400, 400)))
	p.ImageOptions("me", 10, 10, 30, 30, false, fpdf.ImageOptions{ImageType: "JPEG"}, 0, "")
	var out bytes.Buffer
	require.NoError(t, p.Output(&out))
	assert.NotNil(t, ExtractPhoto("pdf", out.Bytes()))
}
