package core

import (
	"context"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDOCXFollowsSectionOrder(t *testing.T) {
	for _, layout := range []string{"classic", "modern", "compact"} {
		t.Run(layout, func(t *testing.T) {
			c := richContent()
			c.SectionOrderKeys = []string{"projects", "education", "experience", "summary", "skills", "certifications", "languages", "extra:0"}
			out, err := NewDOCXExporter().Render(context.Background(), c, layout, RenderOptions{Language: "fr"})
			require.NoError(t, err)
			doc := zipFiles(t, out)["word/document.xml"]
			p := strings.Index(doc, "Lecteur NFC")
			e := strings.Index(doc, c.Education[0].School)
			require.True(t, p >= 0 && e >= 0)
			assert.Less(t, p, e, "projects must come before education")
		})
	}
}

func TestPDFWithStyleAndOrder(t *testing.T) {
	size := 11.0
	for _, layout := range []string{"classic", "modern", "compact"} {
		for _, font := range []string{"", "lato", "carlito", "tinos"} {
			t.Run(layout+"/"+font, func(t *testing.T) {
				c := richContent()
				c.SectionOrderKeys = []string{"skills", "projects", "experience", "education"}
				c.HiddenSections = []string{"languages"}
				c.Style = &api.CVStyle{BodyFont: font, BaseSize: size, HeadingCase: "uppercase", DatePosition: "below", ColorMode: "multi", Accent: "#aa3300", Text: "#111111", Background: "#fafafa"}
				out, err := NewPDFExporter().Render(context.Background(), c, layout, RenderOptions{Language: "fr"})
				require.NoError(t, err)
				assert.True(t, strings.HasPrefix(string(out), "%PDF"))
			})
		}
	}
}

func TestHiddenSectionsAreDropped(t *testing.T) {
	c := richContent()
	c.HiddenSections = []string{"projects", "extra:0"}
	assert.NotContains(t, c.SectionOrder(), "projects")
	assert.NotContains(t, c.SectionOrder(), "extra:0")
	for _, layout := range []string{"classic", "modern", "compact"} {
		out, err := NewDOCXExporter().Render(context.Background(), c, layout, RenderOptions{Language: "fr"})
		require.NoError(t, err)
		doc := zipFiles(t, out)["word/document.xml"]
		assert.NotContains(t, doc, "Lecteur NFC", layout)
		assert.NotContains(t, doc, "IEEE ENIS", layout)
		assert.Contains(t, doc, c.Education[0].School, layout)
	}
}
