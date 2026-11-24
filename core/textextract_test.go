package core

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func docx(t *testing.T, documentXML string) []byte {
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
	data := docx(t, `<?xml version="1.0"?><w:document xmlns:w="w"><w:body>
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
	_, err = ExtractText("docx", docx(t, `<w:document xmlns:w="w"><w:body><w:p/></w:body></w:document>`))
	assert.ErrorIs(t, err, ErrNoText)
	_, err = ExtractText("odt", nil)
	assert.Error(t, err)
}

func TestExtractTextIsCapped(t *testing.T) {
	long := strings.Repeat("é", MaxExtractedText+100)
	text, err := ExtractText("docx", docx(t, `<w:document xmlns:w="w"><w:body><w:p><w:r><w:t>`+long+`</w:t></w:r></w:p></w:body></w:document>`))
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
	assert.Contains(t, text, "Google Data Analytics, Coursera, 2025")
	assert.Contains(t, text, "9itchi-pfe.com")

	data, err = NewPDFExporter().Render(context.Background(), c, "classic", RenderOptions{Language: "en"})
	require.NoError(t, err)
	text, err = ExtractText("pdf", data)
	require.NoError(t, err)
	assert.Contains(t, text, "SKILLS")
	assert.NotContains(t, text, "9itchi-pfe.com", "paid downloads carry no mark")
}
