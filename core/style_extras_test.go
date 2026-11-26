package core

import (
	"context"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allLayouts = []string{"classic", "modern", "compact"}

// styleText renders c with style s and returns its text as one line, spaces collapsed.
func styleText(t *testing.T, e Exporter, layout string, c api.ResumeContent, s *api.CVStyle) string {
	t.Helper()
	c.Style = s
	out, err := e.Render(context.Background(), c, layout, RenderOptions{Language: "en"})
	require.NoError(t, err)
	if e.Format() == "pdf" {
		return strings.Join(pdfLines(t, out), " ")
	}
	text, err := ExtractText(e.Format(), out)
	require.NoError(t, err)
	return strings.Join(strings.Fields(text), " ")
}

func TestEntryLookEducationOrder(t *testing.T) {
	l, ok := entryLook("classic", "education", &api.CVStyle{EducationOrder: "subtitle_first"})
	assert.True(t, ok, "education_order alone counts as a choice")
	assert.True(t, l.subFirst)

	s := &api.CVStyle{EntryOrder: "subtitle_first", EducationOrder: "title_first"}
	l, _ = entryLook("compact", "education", s)
	assert.False(t, l.subFirst, "education_order wins over entry_order")
	l, _ = entryLook("compact", "experience", s)
	assert.True(t, l.subFirst, "experience keeps entry_order")

	_, ok = entryLook("classic", "experience", &api.CVStyle{EducationOrder: "title_first"})
	assert.False(t, ok, "education_order does not touch experience")
}

func TestEducationOrderPDFAndDOCX(t *testing.T) {
	s := &api.CVStyle{EntryLayout: "inline", EntryOrder: "subtitle_first", EducationOrder: "title_first"}
	for _, layout := range allLayouts {
		for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
			lines := entryLines(t, e, layout, s)
			assert.Contains(t, lines[max(lineWith(lines, "MIT"), 0)], "Bachelor, MIT", layout+"/"+e.Format())
			assert.Contains(t, lines[max(lineWith(lines, "Acme"), 0)], "Acme (Stage), Developer", layout+"/"+e.Format())
		}
	}
}

func TestSkillsStyleSeparators(t *testing.T) {
	flat := sampleContent()
	flat.Skills = []string{"Go", "SQL", "Docker"}
	grouped := sampleContent()
	grouped.SkillGroups = []api.SkillGroup{{Label: "Tools", Items: []string{"Go", "SQL", "Docker"}}}
	for style, want := range map[string]string{"comma": "Go, SQL, Docker", "bullet": "Go • SQL • Docker", "bar": "Go | SQL | Docker"} {
		for _, layout := range allLayouts {
			for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
				name := style + "/" + layout + "/" + e.Format()
				assert.Contains(t, styleText(t, e, layout, flat, &api.CVStyle{SkillsStyle: style}), want, name)
				assert.Contains(t, styleText(t, e, layout, grouped, &api.CVStyle{SkillsStyle: style}), want, name+"/group")
			}
		}
	}
	for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
		assert.Contains(t, styleText(t, e, "classic", flat, nil), "Go, SQL, Docker", "unset keeps commas")
		assert.Contains(t, styleText(t, e, "modern", flat, nil), "Go · SQL", "unset keeps the modern pairs")
	}
}

func TestLanguagesInline(t *testing.T) {
	c := sampleContent()
	c.Languages = []string{"English: B2", "French - C1", "Arabic"}
	want := "English (B2), French (C1), Arabic" // short enough for the modern side column
	for _, layout := range allLayouts {
		for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
			assert.Contains(t, styleText(t, e, layout, c, &api.CVStyle{LanguagesStyle: "inline"}), want, layout+"/"+e.Format())
			assert.NotContains(t, styleText(t, e, layout, c, &api.CVStyle{LanguagesStyle: "rows"}), want, layout+"/"+e.Format())
		}
	}
	for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
		lines := entryLines(t, e, "classic", nil)
		assert.GreaterOrEqual(t, lineWith(lines, "English"), 0, "unset keeps one row per language")
	}
}

func TestGroupRoles(t *testing.T) {
	c := sampleContent()
	c.Experience = []api.Experience{
		{Company: "ReDX", Title: "Software Engineer Intern", Start: "02/2024", End: "06/2024", Description: []string{"Built a pipeline"}},
		{Company: " redx ", Title: "DevOps Intern", Start: "06/2023", End: "08/2023", Description: []string{"Wrote CI"}},
		{Company: "Acme", Title: "Dev", Start: "2022"},
	}
	on, off := true, false
	for _, layout := range allLayouts {
		for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
			name := layout + "/" + e.Format()
			text := styleText(t, e, layout, c, &api.CVStyle{GroupRoles: &on})
			assert.Equal(t, 1, strings.Count(strings.ToLower(text), "redx"), name+": company written once: "+text)
			for _, want := range []string{"Software Engineer Intern", "DevOps Intern", "02/2024", "08/2023", "Built a pipeline", "Wrote CI", "Acme"} {
				assert.Contains(t, text, want, name)
			}
			for _, s := range []*api.CVStyle{nil, {GroupRoles: &off}} {
				text = styleText(t, e, layout, c, s)
				assert.Equal(t, 2, strings.Count(strings.ToLower(text), "redx"), name+": ungrouped")
			}
		}
	}
}

func TestRoleGroups(t *testing.T) {
	on := true
	exps := []api.Experience{{Company: "A"}, {Company: "a "}, {Company: "B"}, {Company: ""}, {Company: ""}, {Company: "A"}}
	groups := roleGroups(exps, &api.CVStyle{GroupRoles: &on})
	sizes := []int{}
	for _, g := range groups {
		sizes = append(sizes, len(g))
	}
	assert.Equal(t, []int{2, 1, 1, 1, 1}, sizes, "only consecutive, non-empty, same companies group")
	assert.Len(t, roleGroups(exps, nil), len(exps))
}

func TestHeadingRuleAndSectionGap(t *testing.T) {
	gap := 6.0
	assert.Equal(t, 0.35, headingRuleWidth(nil, 0.35))
	assert.Equal(t, 0.35, headingRuleWidth(&api.CVStyle{HeadingRule: "thin"}, 0.35))
	assert.Equal(t, 0.0, headingRuleWidth(&api.CVStyle{HeadingRule: "thin"}, 0), "thin adds no rule where the template has none")
	assert.Equal(t, 0.55, headingRuleWidth(&api.CVStyle{HeadingRule: "thick"}, 0))
	assert.Equal(t, 0.0, headingRuleWidth(&api.CVStyle{HeadingRule: "none"}, 0.3))
	assert.Equal(t, 2.4, sectionGap(&api.CVStyle{}, 2.4))
	assert.Equal(t, 6.0, sectionGap(&api.CVStyle{SectionGap: &gap}, 2.4))

	doc := func(layout string, s *api.CVStyle) string {
		c := sampleContent()
		c.Style = s
		out, err := NewDOCXExporter().Render(context.Background(), c, layout, RenderOptions{Language: "en"})
		require.NoError(t, err)
		return zipFiles(t, out)["word/document.xml"]
	}
	assert.Contains(t, doc("classic", nil), `<w:bottom w:val="single" w:sz="6"`)
	assert.NotContains(t, doc("classic", &api.CVStyle{HeadingRule: "none"}), `<w:bottom w:val="single"`)
	assert.Contains(t, doc("classic", &api.CVStyle{HeadingRule: "thick"}), `<w:bottom w:val="single" w:sz="12"`)
	assert.NotContains(t, doc("modern", &api.CVStyle{HeadingRule: "thin"}), `<w:bottom w:val="single"`)
	assert.Contains(t, doc("modern", &api.CVStyle{HeadingRule: "thick"}), `<w:bottom w:val="single" w:sz="12"`)
	assert.Contains(t, doc("compact", &api.CVStyle{SectionGap: &gap}), `w:before="340"`)

	for _, layout := range allLayouts {
		for _, s := range []*api.CVStyle{{HeadingRule: "none"}, {HeadingRule: "thick", SectionGap: &gap}, api.DefaultCVStyle()} {
			c := sampleContent()
			c.Style = s
			out, err := NewPDFExporter().Render(context.Background(), c, layout, RenderOptions{Language: "en"})
			require.NoError(t, err, layout)
			assert.NotEmpty(t, out)
		}
	}
}
