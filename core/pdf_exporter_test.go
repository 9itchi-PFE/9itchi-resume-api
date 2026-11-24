package core

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

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
		Summary: "<script>alert(1)</script> (unbalanced \\ ) \x00 ‮ emoji \U0001F600 中文",
		Skills:  []string{strings.Repeat("W", 5000)},
	}
	out, err := NewPDFExporter().Render(context.Background(), c, "classic", RenderOptions{})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))

	out, err = NewPDFExporter().Render(context.Background(), api.ResumeContent{}, "classic", RenderOptions{})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
}

func TestPDFText(t *testing.T) {
	assert.Equal(t, "a?b?c", pdfText("a中b\U0001F600c"))
	assert.Equal(t, "a b c", pdfText("a\tb\nc"))
	assert.Equal(t, "caf\xe9", pdfText("café"))
}

func TestPDFExporterCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewPDFExporter().Render(ctx, sampleContent(), "classic", RenderOptions{})
	assert.Error(t, err)
}

func TestDefaultExporters(t *testing.T) {
	reg := DefaultExporters()
	require.Len(t, reg, 1)
	assert.Equal(t, "pdf", reg["pdf"].Format())
}
