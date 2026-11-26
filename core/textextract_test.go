package core

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/ledongthuc/pdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func docxFile(t *testing.T, documentXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	require.NoError(t, err)
	_, err = w.Write([]byte(documentXML))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestExtractTextDOCX(t *testing.T) {
	data := docxFile(t, `<?xml version="1.0"?><w:document xmlns:w="w"><w:body>
<w:p><w:r><w:t>Ines Mansour</w:t></w:r></w:p>
<w:p><w:r><w:t xml:space="preserve">Élève ingénieure </w:t></w:r><w:r><w:tab/><w:t>Sfax</w:t></w:r></w:p>
<w:p><w:r><w:t>React &amp; Go</w:t><w:br/><w:t>2027</w:t></w:r></w:p>
</w:body></w:document>`)
	text, err := ExtractText("docx", data)
	require.NoError(t, err)
	assert.Equal(t, "Ines Mansour\nÉlève ingénieure \tSfax\nReact & Go\n2027", text)
}

func TestExtractTextPDFRoundTrip(t *testing.T) {
	data, err := NewPDFExporter().Render(context.Background(), api.ResumeContent{
		Contact: api.Contact{Name: "Ines Mansour", Email: "ines@example.com"},
		Skills:  []string{"Golang", "React"},
	}, "classic", RenderOptions{})
	require.NoError(t, err)
	text, err := ExtractText("pdf", data)
	require.NoError(t, err)
	assert.Contains(t, text, "Ines Mansour")
	assert.Contains(t, text, "Golang")
}

func TestExtractTextErrors(t *testing.T) {
	_, err := ExtractText("pdf", []byte("%PDF-1.4 garbage"))
	assert.EqualError(t, err, "unreadable pdf")
	_, err = ExtractText("docx", []byte("PK\x03\x04 not a zip"))
	assert.EqualError(t, err, "unreadable docx")
	_, err = ExtractText("docx", docxFile(t, `<w:document xmlns:w="w"><w:body><w:p/></w:body></w:document>`))
	assert.ErrorIs(t, err, ErrNoText)
	_, err = ExtractText("odt", nil)
	assert.Error(t, err)
}

func TestExtractTextIsCapped(t *testing.T) {
	long := strings.Repeat("é", MaxExtractedText+100)
	text, err := ExtractText("docx", docxFile(t, `<w:document xmlns:w="w"><w:body><w:p><w:r><w:t>`+long+`</w:t></w:r></w:p></w:body></w:document>`))
	require.NoError(t, err)
	assert.Len(t, []rune(text), MaxExtractedText)
}

func TestPDFBrandedAndLocalizedHeadings(t *testing.T) {
	c := api.ResumeContent{Contact: api.Contact{Name: "Ines"}, Skills: []string{"Go"}, Certifications: []api.Certification{{Name: "Google Data Analytics", Issuer: "Coursera", Year: "2025"}}}
	data, err := NewPDFExporter().Render(context.Background(), c, "classic", RenderOptions{Language: "fr", Branded: true})
	require.NoError(t, err)
	text, err := ExtractText("pdf", data)
	require.NoError(t, err)
	assert.Contains(t, text, "COMPÉTENCES")
	for _, want := range []string{"Google Data Analytics", "Coursera", "2025"} {
		assert.Contains(t, text, want)
	}
	assert.Contains(t, text, BrandLead)

	data, err = NewPDFExporter().Render(context.Background(), c, "classic", RenderOptions{Language: "en"})
	require.NoError(t, err)
	text, err = ExtractText("pdf", data)
	require.NoError(t, err)
	assert.Contains(t, text, "SKILLS")
	assert.NotContains(t, text, BrandLead, "paid downloads carry no mark")
}

func TestWriteRowsKeepsDrawingOrder(t *testing.T) {
	g := func(s string, x, y float64) pdf.Text { return pdf.Text{S: s, X: x, Y: y, FontSize: 10} }
	// Glyph by glyph: a heading, a line with a right-hand date, a date drawn before its title, then a second column
	// sharing the baselines of the first (it must not be mixed into it).
	texts := []pdf.Text{
		g("S", 20, 800), g("K", 26, 800),
		g("E", 20, 760), g("N", 26, 760), g(" ", 32, 760), g("I", 36, 760), g("S", 50, 760.5), g("2", 300, 760), g("0", 306, 760),
		g("2", 300, 740), g("4", 306, 740), g("G", 20, 740), g("o", 26, 740),
		g("M", 400, 800), g("P", 408, 800), g("X", 400, 760),
	}
	var sb strings.Builder
	writeRows(&sb, texts)
	assert.Equal(t, "SK\nEN I S   20\n24   Go\nMP\nX\n", sb.String())
}

func TestWriteRowsFontWithoutWidths(t *testing.T) {
	// Every glyph of a string at the string's X (no widths): words stay in drawing order, separate strings apart.
	var texts []pdf.Text
	for _, r := range "Degree" {
		texts = append(texts, pdf.Text{S: string(r), X: 20, Y: 700, FontSize: 10})
	}
	texts = append(texts, pdf.Text{S: ",", X: 50, Y: 700, FontSize: 10}, pdf.Text{S: "E", X: 54, Y: 700, FontSize: 10},
		pdf.Text{S: "N", X: 54, Y: 700, FontSize: 10}, pdf.Text{S: "I", X: 54, Y: 700, FontSize: 10}, pdf.Text{S: "S", X: 54, Y: 700, FontSize: 10},
		pdf.Text{S: "2", X: 70, Y: 700, FontSize: 10})
	var sb strings.Builder
	writeRows(&sb, texts)
	assert.Equal(t, "Degree,ENIS 2\n", sb.String())
}
