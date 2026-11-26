package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode"

	"9itchi-resume-api/models/api"

	"github.com/ledongthuc/pdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleContent() api.ResumeContent {
	return api.ResumeContent{
		Contact: api.Contact{Name: "Jane Doe", Email: "jane@example.com", Links: []string{"https://example.com"}},
		Summary: "Engineer",
		Experience: []api.Experience{
			{Company: "Acme", Title: "Dev", Start: "2020", End: "Present", Description: []string{"Built things"}},
		},
		Education: []api.Education{{School: "MIT", Degree: "BSc"}},
		Skills:    []string{"Go"},
		Languages: []string{"English"},
	}
}

func TestPDFExporterOutputsPDF(t *testing.T) {
	for _, layout := range []string{"classic", "modern", "compact", "unknown", ""} {
		t.Run(layout, func(t *testing.T) {
			out, err := NewPDFExporter().Render(context.Background(), sampleContent(), layout, RenderOptions{})
			require.NoError(t, err)
			assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
		})
	}
}

func TestPDFExporterEmptyAndHostileContent(t *testing.T) {
	c := api.ResumeContent{
		Summary: "<script>alert(1)</script> (unbalanced \\ ) \x00 \u202e emoji \U0001F600 中文",
		Skills:  []string{strings.Repeat("W", 5000)},
	}
	out, err := NewPDFExporter().Render(context.Background(), c, "classic", RenderOptions{})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))

	out, err = NewPDFExporter().Render(context.Background(), api.ResumeContent{}, "classic", RenderOptions{})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
}

func TestCleanText(t *testing.T) {
	assert.Equal(t, "a?b?c", cleanText("a中b\U0001F600c"))
	assert.Equal(t, "a b c", cleanText("a\tb\nc"))
	assert.Equal(t, "Élève — café · ok", cleanText("Élève — café · ok"))
}

func TestPDFExporterCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewPDFExporter().Render(ctx, sampleContent(), "classic", RenderOptions{})
	assert.Error(t, err)
}

func TestDefaultExporters(t *testing.T) {
	reg := DefaultExporters()
	require.Len(t, reg, 2)
	assert.Equal(t, "pdf", reg["pdf"].Format())
	assert.Equal(t, "docx", reg["docx"].Format())
}

// pageTexts returns the plain text of every page of a rendered PDF.
func pageTexts(t *testing.T, data []byte) []string {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	var out []string
	for i := 1; i <= r.NumPage(); i++ {
		txt, err := r.Page(i).GetPlainText(nil)
		require.NoError(t, err)
		out = append(out, strings.Join(strings.Fields(txt), " "))
	}
	return out
}

// A row near the bottom of a page must move to the next page as a whole: no page holding only dates
// and no blank page after it.
func TestPDFExporterNoBlankOrDatesOnlyPage(t *testing.T) {
	for _, layout := range []string{"classic", "modern", "compact"} {
		for n := 1; n <= 12; n++ {
			for bullets := 1; bullets <= 4; bullets++ {
				c := sampleContent()
				c.Experience = nil
				for i := 0; i < n; i++ {
					var desc []string
					for b := 0; b < bullets; b++ {
						desc = append(desc, fmt.Sprintf("Delivered project %d item %d with measurable results for the team", i, b))
					}
					c.Experience = append(c.Experience, api.Experience{
						Company: fmt.Sprintf("Company %d", i), Title: fmt.Sprintf("Developer %d", i),
						Location: "Tunis", Start: "2020", End: "2023", Description: desc,
					})
				}
				out, err := NewPDFExporter().Render(context.Background(), c, layout, RenderOptions{})
				require.NoError(t, err)
				texts := pageTexts(t, out)
				require.Equal(t, PageCount(out, "pdf"), len(texts))
				for i, txt := range texts {
					msg := fmt.Sprintf("%s n=%d bullets=%d: page %d of %d nearly empty: %q", layout, n, bullets, i+1, len(texts), txt)
					if i < len(texts)-1 {
						assert.Greater(t, len(txt), 60, msg)
						continue
					}
					// the last page may be short, but never only dates or bullet glyphs
					letters := strings.Map(func(r rune) rune {
						if unicode.IsLetter(r) {
							return r
						}
						return -1
					}, strings.ReplaceAll(txt, "Tunis", ""))
					assert.GreaterOrEqual(t, len(letters), 6, msg)
				}
			}
		}
	}
}
