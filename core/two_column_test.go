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

var twoColLayouts = []string{"sidebar", "split", "banner"}

func twoColContent() api.ResumeContent {
	return api.ResumeContent{
		Contact: api.Contact{Name: "Yasmine Ben Salah", Headline: "Élève ingénieure en génie logiciel", Email: "yasmine.bensalah@gmail.com",
			Phone: "+216 22 345 678", Location: "Sfax, Tunisie", Links: []string{"https://linkedin.com/in/yasmine-bensalah", "https://github.com/yasminebs"}},
		Summary: "Élève ingénieure en dernière année à l'ENIS, passionnée par le développement backend et le cloud.",
		Experience: []api.Experience{
			{Company: "Vermeg", Title: "Stagiaire développeuse backend", Location: "Tunis", Start: "juil. 2025", End: "sept. 2025",
				Description: []string{"Conçu une API REST en Go pour le suivi des ordres", "Réduit de 40 % le temps de réponse d'un rapport"}},
			{Company: "Sofrecom", Title: "Stagiaire DevOps", Location: "Tunis", Start: "juil. 2024", End: "août 2024",
				Description: []string{"Conteneurisé trois services Java avec Docker"}},
		},
		Education: []api.Education{
			{School: "École Nationale d'Ingénieurs de Sfax", Degree: "Diplôme d'ingénieur en informatique", Location: "Sfax", Start: "2022", End: "2026"},
			{School: "IPEIS", Degree: "Cycle préparatoire MP", Start: "2020", End: "2022"},
		},
		Projects:       []api.Project{{Name: "Lecteur NFC", Tech: "C++", Year: "2026", Description: []string{"Conçu un lecteur"}}},
		Skills:         []string{"Go", "Java", "PostgreSQL", "Docker", "Kubernetes"},
		Languages:      []string{"Arabe: langue maternelle", "Anglais: B2"},
		Certifications: []api.Certification{{Name: "AWS Cloud Practitioner", Issuer: "Amazon", Year: "2025"}},
		Extra:          []api.ExtraSection{{Title: "Clubs", Items: []string{"IEEE ENIS"}}},
	}
}

func TestTwoColumnLayoutsRender(t *testing.T) {
	photo := pngPhoto(t, 40, 20)
	for _, layout := range twoColLayouts {
		for name, style := range map[string]*api.CVStyle{"nil": nil, "default": api.DefaultCVStyle()} {
			for _, format := range []string{"pdf", "docx"} {
				t.Run(layout+"/"+name+"/"+format, func(t *testing.T) {
					c := twoColContent()
					c.Style = style
					out, err := DefaultExporters()[format].Render(context.Background(), c, layout, RenderOptions{Language: "fr", Photo: photo, PhotoFormat: "png"})
					require.NoError(t, err)
					text, err := ExtractText(format, out)
					require.NoError(t, err)
					text = strings.Join(strings.Fields(text), " ")
					for _, want := range []string{"Yasmine Ben Salah", "Stagiaire DevOps", "Kubernetes", "Anglais", "AWS Cloud Practitioner",
						"Cycle préparatoire MP", "Lecteur NFC", "IEEE ENIS"} {
						assert.Contains(t, text, want)
					}
					if format == "pdf" {
						assert.Equal(t, 1, PageCount(out, format), "a short CV fits one page")
					}
				})
			}
		}
	}
}

func TestTwoColumnDOCXFills(t *testing.T) {
	for layout, fills := range map[string][]string{"sidebar": {"1F3A5F"}, "banner": {"7A2E3B", "F3F4F6"}, "split": {"D9DCE1"}} {
		out, err := NewDOCXExporter().Render(context.Background(), twoColContent(), layout, RenderOptions{})
		require.NoError(t, err)
		doc := zipFiles(t, out)["word/document.xml"]
		for _, f := range fills {
			assert.Contains(t, doc, `"`+f+`"`, layout)
		}
	}
	// a chosen accent replaces the sidebar fill and the banner band
	c := twoColContent()
	c.Style = &api.CVStyle{Accent: "#123456"}
	for _, layout := range []string{"sidebar", "banner"} {
		out, err := NewDOCXExporter().Render(context.Background(), c, layout, RenderOptions{})
		require.NoError(t, err)
		assert.Contains(t, zipFiles(t, out)["word/document.xml"], `w:fill="123456"`, layout)
	}
}

// Both columns break pages: a long main column and a long side column each go on to page 2, with no page holding
// only dates and no blank page.
func TestTwoColumnPageBreaks(t *testing.T) {
	for _, layout := range twoColLayouts {
		for _, style := range []*api.CVStyle{nil, api.DefaultCVStyle()} {
			for n := 4; n <= 16; n += 2 {
				for bullets := 1; bullets <= 3; bullets++ {
					c := twoColContent()
					c.Style = style
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
					if n == 16 && bullets == 3 {
						assert.Greater(t, len(texts), 1, "%s n=%d spills to page 2", layout, n)
					}
					for i, txt := range texts {
						msg := fmt.Sprintf("%s n=%d bullets=%d: page %d of %d nearly empty: %q", layout, n, bullets, i+1, len(texts), txt)
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
		// a side column longer than the page goes on to page 2 as well
		c := twoColContent()
		for i := 0; i < 60; i++ {
			c.Certifications = append(c.Certifications, api.Certification{Name: fmt.Sprintf("Certificate %d", i), Issuer: "Coursera", Year: "2024"})
		}
		out, err := NewPDFExporter().Render(context.Background(), c, layout, RenderOptions{})
		require.NoError(t, err)
		texts := pageTexts(t, out)
		require.Len(t, texts, 2, layout)
		assert.Contains(t, texts[1], "Certificate 59", layout)
		assert.Contains(t, texts[0], "Stagiaire DevOps", layout, "the main column stays on page 1")
	}
}

// The banner grows to hold a long title written inline after the name and a contact line of many icon items: no
// header text falls below the band.
func TestBannerHoldsLongHeader(t *testing.T) {
	c := twoColContent()
	c.Style = api.DefaultCVStyle()
	require.Equal(t, "inline", c.Style.HeaderLayout)
	c.Contact.Headline = "Ingénieure logicielle Zorglub spécialisée en systèmes distribués, cloud natif, observabilité et automatisation Zorglubend"
	for i := 0; i < 6; i++ {
		c.Contact.Links = append(c.Contact.Links, fmt.Sprintf("https://site%d.example.com/portfolio", i))
	}
	for _, photo := range [][]byte{nil, pngPhoto(t, 40, 20)} {
		out, err := NewPDFExporter().Render(context.Background(), c, "banner", RenderOptions{Language: "fr", Photo: photo, PhotoFormat: "png"})
		require.NoError(t, err)
		r, err := pdf.NewReader(bytes.NewReader(out), int64(len(out)))
		require.NoError(t, err)
		content := r.Page(1).Content()
		bottom := 0.0 // the band: full width, from the top of the page
		for _, rc := range content.Rect {
			lo, hi := min(rc.Min.Y, rc.Max.Y), max(rc.Min.Y, rc.Max.Y)
			if min(rc.Min.X, rc.Max.X) < 1 && max(rc.Min.X, rc.Max.X) > 594 && hi > 840 && hi-lo < 400 && lo > bottom {
				bottom = lo
			}
		}
		require.Positive(t, bottom, "band found")
		var in, below []string
		lowest := 1000.0 // baseline of the last header line
		for _, tx := range content.Text {
			if tx.Y > bottom {
				in = append(in, tx.S)
				lowest = min(lowest, tx.Y)
			} else {
				below = append(below, tx.S)
			}
		}
		band, rest := strings.Join(in, ""), strings.Join(below, "")
		for _, want := range []string{"Zorglubend", "site5.example.com", "Yasmine"} {
			assert.Contains(t, band, want)
		}
		for _, marker := range []string{"Zorglub", "example.com", "linkedin.com", "Sfax, Tunisie"} {
			assert.NotContains(t, rest, marker)
		}
		assert.GreaterOrEqual(t, lowest-bottom, 12.0, "the band keeps its padding under the last line")
	}
}
