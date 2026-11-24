package core

import (
	"context"
	"strings"
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func goodCV() api.ResumeContent {
	return api.ResumeContent{
		Contact: api.Contact{Name: "Ines Mansour", Email: "ines@example.tn", Phone: "+216 20 000 000", Location: "Sfax, Tunisie",
			Links: []string{"https://linkedin.com/in/ines", "https://github.com/ines"}},
		Summary: "Élève ingénieure en génie logiciel, passionnée par les systèmes embarqués. Projet de semestre : un lecteur NFC qui synchronise 300 badges hors ligne.",
		Experience: []api.Experience{
			{Company: "Sfax Digital", Title: "Stagiaire", Start: "juin 2025", End: "août 2025", Description: []string{
				"Développé en React la page de réservation d'une clinique, utilisée pour 400 rendez-vous par mois.",
				"Réduit de 30 % le temps de chargement avec une équipe de quatre.",
			}},
			{Company: "ENIS", Title: "Projet", Start: "2024-09", End: "2025-01", Description: []string{
				"Conçu un lecteur de badges NFC en C++ synchronisant 15 terminaux.",
			}},
		},
		Education:      []api.Education{{School: "ENIS", Degree: "Génie logiciel", Start: "2022", End: "2027"}},
		Skills:         []string{"C++", "React", "Linux", "Go"},
		Languages:      []string{"Français : courant", "Anglais : B2 (TOEIC 850)"},
		Certifications: []api.Certification{{Name: "Google Data Analytics", Issuer: "Coursera", Year: "2025"}},
	}
}

func check(c api.ResumeContent, mod ...func(*ATSInput)) map[string]api.ATSRuleResult {
	in := ATSInput{Content: c, Language: "fr", ExportFilename: "ines_mansour.pdf",
		Rendered: RenderForCheck(context.Background(), c, "classic", "fr")}
	for _, m := range mod {
		m(&in)
	}
	out := CheckATS(in)
	byID := map[string]api.ATSRuleResult{}
	for _, r := range out.Rules {
		byID[r.ID] = r
	}
	return byID
}

func TestRuleList(t *testing.T) {
	assert.Len(t, RuleIDs, 24)
	ids := sortedIDs()
	for i := 1; i < len(ids); i++ {
		assert.NotEqual(t, ids[i-1], ids[i], "duplicate rule id")
	}
	for _, id := range RuleIDs {
		assert.NotEmpty(t, ruleGroups[id], id)
	}
}

func TestGoodCVPassesEverything(t *testing.T) {
	c := goodCV()
	out := CheckATS(ATSInput{Content: c, Language: "fr", ExportFilename: "ines_mansour.pdf", Rendered: RenderForCheck(context.Background(), c, "classic", "fr")})
	for _, r := range out.Rules {
		assert.NotEqual(t, RuleFix, r.Status, "%s: %+v", r.ID, r.ATSFinding)
	}
	assert.Equal(t, 24, out.Passed)
	assert.Equal(t, 0, out.ToFix)
	assert.Equal(t, "ines_mansour.pdf", findRule(out, "file_name").Now)
}

func findRule(out api.ATSCheckResponse, id string) api.ATSRuleResult {
	for _, r := range out.Rules {
		if r.ID == id {
			return r
		}
	}
	return api.ATSRuleResult{}
}

func TestEachRuleCanFail(t *testing.T) {
	cases := []struct {
		id    string
		mod   func(c *api.ResumeContent)
		where string
	}{
		{"contact_details", func(c *api.ResumeContent) { c.Contact.Phone = "" }, "contact.phone"},
		{"no_personal_details", func(c *api.ResumeContent) { c.Summary = "Étudiante de 23 ans, célibataire, passionnée par le développement web et les systèmes embarqués, curieuse." }, "summary"},
		{"profile_links", func(c *api.ResumeContent) { c.Contact.Links = nil }, "contact.links"},
		{"summary", func(c *api.ResumeContent) { c.Summary = "Étudiante." }, "summary"},
		{"action_verbs", func(c *api.ResumeContent) { c.Experience[0].Description[1] = "Fonctionne sans réseau et synchronise 20 badges." }, "experience[0].description[1]"},
		{"quantified_results", func(c *api.ResumeContent) {
			c.Experience[0].Description = []string{"Développé la page de réservation.", "Réduit le temps de chargement."}
			c.Experience[1].Description = []string{"Conçu un lecteur de badges NFC."}
		}, "experience[0].description[0]"},
		{"short_bullets", func(c *api.ResumeContent) { c.Experience[1].Description[0] = "Conçu " + strings.Repeat("un lecteur, ", 30) + "15 fois." }, "experience[1].description[0]"},
		{"no_first_person", func(c *api.ResumeContent) { c.Experience[0].Description[0] = "Développé avec mon équipe 3 modules React." }, "experience[0].description[0]"},
		{"number_style", func(c *api.ResumeContent) { c.Experience[0].Description[1] = "Réduit le temps de chargement avec 4 développeurs, 30 % de gain." }, "experience[0].description[1]"},
		{"reverse_chronological", func(c *api.ResumeContent) { c.Experience[0], c.Experience[1] = c.Experience[1], c.Experience[0] }, "experience[1].start"},
		{"graduation_year", func(c *api.ResumeContent) { c.Education[0].End = "" }, "education[0].end"},
		{"no_skill_ratings", func(c *api.ResumeContent) { c.Languages = []string{"Anglais ★★★★☆"} }, "languages[0]"},
		{"relevant_skills", func(c *api.ResumeContent) { c.Skills = nil }, "skills"},
		{"certifications", func(c *api.ResumeContent) { c.Certifications = nil }, "certifications"},
	}
	for _, tc := range cases {
		c := goodCV()
		tc.mod(&c)
		r := check(c)[tc.id]
		assert.Equal(t, RuleFix, r.Status, tc.id)
		assert.Equal(t, tc.where, r.Where, tc.id)
	}
}

func TestPageSizeFileAndLanguageRules(t *testing.T) {
	c := goodCV()
	for i := 0; i < 12; i++ {
		c.Experience = append(c.Experience, api.Experience{Company: "X", Start: "2020", End: "2021",
			Description: []string{"Développé 3 modules.", "Développé 4 écrans.", "Développé 5 services."}})
	}
	r := check(c)
	assert.Equal(t, RuleFix, r["one_page"].Status)
	assert.Equal(t, RuleOK, r["pdf_size"].Status)

	r = check(goodCV(), func(in *ATSInput) { in.ExportFilename = "" })
	assert.Equal(t, RuleFix, r["file_name"].Status)
	assert.Equal(t, "contact.name", r["file_name"].Where)

	r = check(goodCV(), func(in *ATSInput) { in.TargetLanguage = "en" })
	assert.Equal(t, RuleFix, r["company_language"].Status)
	assert.Equal(t, "en", r["company_language"].Now)
	r = check(goodCV(), func(in *ATSInput) { in.TargetLanguage = "fr" })
	assert.Equal(t, RuleOK, r["company_language"].Status)
	r = check(goodCV())
	assert.Equal(t, RuleTip, r["company_language"].Status, "no target language: a tip")
}

func TestStartsWithPastVerb(t *testing.T) {
	for _, s := range []string{"Développé une API", "Conçu un lecteur", "Mis en place la CI", "Réalisé", "Built a parser", "Designed", "Led a team", "• Optimisé les requêtes"} {
		assert.True(t, startsWithPastVerb(s), s)
	}
	for _, s := range []string{"Fonctionne sans réseau", "Développement d'une API", "Responsable de", "I built", "Ed tech", "Développe une API"} {
		assert.False(t, startsWithPastVerb(s), s)
	}
}

func TestNumberStyle(t *testing.T) {
	for _, s := range []string{"Encadré 4 stagiaires", "Testé quinze anomalies", "Fixed twelve bugs"} {
		assert.True(t, badNumberStyle(s), s)
	}
	for _, s := range []string{"Encadré quatre stagiaires", "Corrigé 15 anomalies", "Réduit de 3 % le temps", "Migré vers Java 17", "Livré la v2"} {
		assert.False(t, badNumberStyle(s), s)
	}
}

func TestDateKey(t *testing.T) {
	assert.Equal(t, 202506, dateKey("2025-06"))
	assert.Equal(t, 202506, dateKey("06/2025"))
	assert.Equal(t, 202506, dateKey("juin 2025"))
	assert.Equal(t, 202509, dateKey("Sept. 2025"))
	assert.Equal(t, 202500, dateKey("2025"))
	assert.Equal(t, 0, dateKey("aujourd'hui"))
}

func TestPageCount(t *testing.T) {
	assert.Equal(t, 0, pageCount(nil))
	assert.Equal(t, 0, pageCount([]byte("%PDF-garbage")))
	out := RenderForCheck(context.Background(), goodCV(), "modern", "en")
	require.NotEmpty(t, out)
	assert.Equal(t, 1, pageCount(out))
}
