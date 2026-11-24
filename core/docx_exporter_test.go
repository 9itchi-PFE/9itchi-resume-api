package core

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func zipFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		out[f.Name] = string(b)
	}
	return out
}

func pngPhoto(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func richContent() api.ResumeContent {
	c := sampleContent()
	c.Contact.Headline = "Élève ingénieure"
	c.Projects = []api.Project{{Name: "Lecteur NFC", Tech: "C++", Year: "2026", Description: []string{"Conçu un lecteur"}}}
	c.SkillGroups = []api.SkillGroup{{Label: "Langages", Items: []string{"C++", "Go"}}}
	c.Languages = []string{"Arabe: Langue maternelle"}
	c.Extra = []api.ExtraSection{{Title: "Clubs", Items: []string{"IEEE ENIS"}}}
	return c
}

func TestDOCXLayouts(t *testing.T) {
	photo := pngPhoto(t, 40, 20)
	for _, layout := range []string{"classic", "modern", "compact", ""} {
		t.Run(layout, func(t *testing.T) {
			out, err := NewDOCXExporter().Render(context.Background(), richContent(), layout, RenderOptions{Language: "fr", Photo: photo, PhotoFormat: "png"})
			require.NoError(t, err)
			files := zipFiles(t, out)
			doc := files["word/document.xml"]
			for _, want := range []string{"Jane Doe", "Élève ingénieure", "Lecteur NFC", "Langages", "Langue maternelle", "IEEE ENIS"} {
				assert.Contains(t, doc, want)
			}
			_, hasPhoto := files["word/media/photo.png"]
			assert.Equal(t, layout != "compact", hasPhoto, "compact never shows the photo")
			assert.Equal(t, hasPhoto, strings.Contains(doc, `prst="ellipse"`))
			if layout == "modern" {
				assert.Contains(t, doc, `w:fill="1F2A44"`, "dark side column")
			}
			if layout == "classic" || layout == "" {
				assert.Contains(t, doc, `<w:smallCaps/>`)
				assert.Contains(t, doc, `2F5597`)
				assert.Contains(t, doc, `l="25000"`, "a wide photo is cropped to a square")
			}
			text, err := ExtractText("docx", out)
			require.NoError(t, err)
			assert.Contains(t, text, "Jane Doe")
		})
	}
}

func TestDOCXBrandingEscapingAndBadPhoto(t *testing.T) {
	c := api.ResumeContent{Contact: api.Contact{Name: `<b>"Tom" & Jerry</b>` + "\x00\x01"}, Summary: "محمد"}
	out, err := NewDOCXExporter().Render(context.Background(), c, "classic", RenderOptions{Branded: true, Photo: []byte("not an image")})
	require.NoError(t, err)
	files := zipFiles(t, out)
	doc := files["word/document.xml"]
	assert.Contains(t, doc, "&lt;b&gt;&quot;Tom&quot; &amp; Jerry&lt;/b&gt;")
	assert.NotContains(t, doc, "\x00")
	assert.Contains(t, doc, "محمد", "Word can show Arabic")
	assert.Contains(t, files["word/footer1.xml"], "9itchi-pfe.com")
	assert.Contains(t, doc, `rIdFooter`)
	assert.NotContains(t, files["word/_rels/document.xml.rels"], "rIdPhoto", "an unreadable photo is skipped")

	out, err = NewDOCXExporter().Render(context.Background(), api.ResumeContent{}, "modern", RenderOptions{})
	require.NoError(t, err)
	_, hasFooter := zipFiles(t, out)["word/footer1.xml"]
	assert.False(t, hasFooter)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewDOCXExporter().Render(ctx, c, "classic", RenderOptions{})
	assert.Error(t, err)
}

func TestPDFPhotoAndNewSections(t *testing.T) {
	photo := pngPhoto(t, 20, 40)
	for _, layout := range []string{"classic", "modern", "compact"} {
		out, err := NewPDFExporter().Render(context.Background(), richContent(), layout, RenderOptions{Photo: photo, PhotoFormat: "png"})
		require.NoError(t, err, layout)
		text, err := ExtractText("pdf", out)
		require.NoError(t, err)
		for _, want := range []string{"Lecteur NFC", "Langages", "IEEE ENIS"} {
			assert.Contains(t, text, want, layout)
		}
		assert.True(t, bytes.Contains(out, []byte("/Subtype /Image")), layout)
	}
	out, err := NewPDFExporter().Render(context.Background(), richContent(), "classic", RenderOptions{Photo: []byte("garbage"), PhotoFormat: "jpeg"})
	require.NoError(t, err, "an unreadable photo is skipped")
	assert.False(t, bytes.Contains(out, []byte("/Subtype /Image")))
}

func TestSplitLanguage(t *testing.T) {
	for in, want := range map[string][2]string{
		"Arabe: Langue maternelle": {"Arabe", "Langue maternelle"},
		"Anglais — B2":             {"Anglais", "B2"},
		"Français":                 {"Français", ""},
	} {
		n, l := splitLanguage(in)
		assert.Equal(t, want, [2]string{n, l}, in)
	}
}
