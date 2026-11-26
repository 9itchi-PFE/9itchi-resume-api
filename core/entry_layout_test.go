package core

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/ledongthuc/pdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func entryLayoutContent(s *api.CVStyle) api.ResumeContent {
	c := sampleContent()
	c.Experience = []api.Experience{{Company: "Acme", Title: "Developer", Kind: "Stage", Location: "Tunis", Start: "2020", End: "2021"}}
	c.Education = []api.Education{{School: "MIT", Degree: "Bachelor", Start: "2016", End: "2019"}}
	c.Style = s
	return c
}

func entryLines(t *testing.T, e Exporter, layout string, s *api.CVStyle) []string {
	t.Helper()
	out, err := e.Render(context.Background(), entryLayoutContent(s), layout, RenderOptions{Language: "en"})
	require.NoError(t, err)
	if e.Format() == "pdf" {
		return pdfLines(t, out)
	}
	text, err := ExtractText(e.Format(), out)
	require.NoError(t, err)
	return strings.Split(text, "\n")
}

// pdfLines reads the first page as visual lines, left to right; ExtractText flattens them. Texts less than 4 pt apart
// vertically share a line (the left and right cells of a row sit at slightly different baselines).
func pdfLines(t *testing.T, data []byte) []string {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	texts := r.Page(1).Content().Text
	sort.SliceStable(texts, func(i, j int) bool { return texts[i].Y > texts[j].Y })
	var lines []string
	for i := 0; i < len(texts); {
		j := i
		for j < len(texts) && texts[i].Y-texts[j].Y < 4 {
			j++
		}
		row := texts[i:j]
		sort.SliceStable(row, func(a, b int) bool { return row[a].X < row[b].X })
		var parts []string
		for _, x := range row {
			parts = append(parts, x.S)
		}
		lines = append(lines, strings.Join(strings.Fields(strings.Join(parts, "")), " "))
		i = j
	}
	return lines
}

func lineWith(lines []string, sub string) int {
	for i, l := range lines {
		if strings.Contains(l, sub) {
			return i
		}
	}
	return -1
}

func TestEntryLookFallsBackPerField(t *testing.T) {
	_, ok := entryLook("modern", "experience", nil)
	assert.False(t, ok)
	_, ok = entryLook("modern", "experience", &api.CVStyle{BodyFont: "lato"})
	assert.False(t, ok)

	l, ok := entryLook("modern", "education", &api.CVStyle{EntryLayout: "stacked"})
	assert.True(t, ok)
	assert.Equal(t, entryLayout{inline: false, subFirst: true}, l, "order stays the template's")
	l, _ = entryLook("compact", "experience", &api.CVStyle{EntryOrder: "title_first"})
	assert.Equal(t, entryLayout{inline: true, subFirst: false}, l, "layout stays the template's")
	l, _ = entryLook("classic", "experience", &api.CVStyle{EntryLayout: "inline"})
	assert.Equal(t, entryLayout{inline: true, subFirst: false}, l)
}

func TestEntryLayoutPDFAndDOCX(t *testing.T) {
	inlineSub := &api.CVStyle{EntryLayout: "inline", EntryOrder: "subtitle_first"}
	inlineTitle := &api.CVStyle{EntryLayout: "inline", EntryOrder: "title_first"}
	stackedTitle := &api.CVStyle{EntryLayout: "stacked", EntryOrder: "title_first"}
	stackedSub := &api.CVStyle{EntryLayout: "stacked", EntryOrder: "subtitle_first"}

	for _, layout := range []string{"classic", "modern", "compact"} {
		for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
			t.Run(layout+"/"+e.Format(), func(t *testing.T) {
				lines := entryLines(t, e, layout, inlineSub)
				assert.Contains(t, lines[max(lineWith(lines, "Acme"), 0)], "Acme (Stage), Developer")
				assert.Contains(t, lines[max(lineWith(lines, "Acme"), 0)], "Tunis", "location follows the text on the same row")
				assert.Contains(t, lines[max(lineWith(lines, "MIT"), 0)], "MIT, Bachelor")

				lines = entryLines(t, e, layout, inlineTitle)
				assert.Contains(t, lines[max(lineWith(lines, "Developer"), 0)], "Developer, Acme (Stage)")
				assert.Contains(t, lines[max(lineWith(lines, "Bachelor"), 0)], "Bachelor, MIT")

				lines = entryLines(t, e, layout, stackedTitle)
				title, company := lineWith(lines, "Developer"), lineWith(lines, "Acme")
				require.True(t, title >= 0 && company > title, "title row above company row: %v", lines)
				assert.Contains(t, lines[company], "(Stage)")
				assert.Contains(t, lines[company], "Tunis")
				assert.Contains(t, lines[title], "2020")
				degree, school := lineWith(lines, "Bachelor"), lineWith(lines, "MIT")
				assert.True(t, degree >= 0 && school > degree, "degree row above school row: %v", lines)

				lines = entryLines(t, e, layout, stackedSub)
				title, company = lineWith(lines, "Developer"), lineWith(lines, "Acme")
				assert.True(t, company >= 0 && title > company, "company row above title row: %v", lines)
			})
		}
	}
}

func TestEntryLayoutUnsetKeepsTemplateDefaults(t *testing.T) {
	want := map[string][2]string{
		"modern":  {"Developer — Acme", "MIT — Bachelor"},
		"compact": {"Acme : Developer", "MIT — Bachelor"},
	}
	for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
		for _, style := range []*api.CVStyle{nil, {}, {BaseSize: 11}} {
			for layout, w := range want {
				lines := entryLines(t, e, layout, style)
				assert.Contains(t, lines[max(lineWith(lines, "Acme"), 0)], w[0], layout+"/"+e.Format())
				assert.Contains(t, lines[max(lineWith(lines, "MIT"), 0)], w[1], layout+"/"+e.Format())
			}
			lines := entryLines(t, e, "classic", style)
			title, company := lineWith(lines, "Developer"), lineWith(lines, "Acme")
			assert.True(t, title >= 0 && company > title, "classic stacks title then company: %v", lines)
		}
	}
}

func TestEntryLayoutHonoursLocationBesideDates(t *testing.T) {
	for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
		lines := entryLines(t, e, "modern", &api.CVStyle{EntryLayout: "stacked", LocationPos: "date"})
		title, company := lineWith(lines, "Developer"), lineWith(lines, "Acme")
		require.True(t, title >= 0 && company >= 0, "%v", lines)
		assert.Contains(t, lines[title], "Tunis", "location moves beside the dates")
		assert.NotContains(t, lines[company], "Tunis")
	}
}
