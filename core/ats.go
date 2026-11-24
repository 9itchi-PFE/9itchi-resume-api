package core

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"9itchi-resume-api/models/api"

	"github.com/ledongthuc/pdf"
)

// ATS rule statuses: fix counts as a point to correct, tip depends on where the student applies.
const (
	RuleOK  = "ok"
	RuleFix = "fix"
	RuleTip = "tip"
)

// ATSInput is what the 24 rules look at.
type ATSInput struct {
	Content api.ResumeContent
	// Language is the CV's language (fr, en, ar) if known; TargetLanguage is the company's, if the student said it.
	Language       string
	TargetLanguage string
	// ExportFilename is the name exports get (prenom_nom.pdf), empty when the CV has no name.
	ExportFilename string
	// Rendered is the CV rendered as PDF by the current template (page count and size).
	Rendered []byte
}

// RuleIDs lists the 24 rules in screen order (4.2 "Vérification du CV"). Texts live in the client (fr, en, ar).
var RuleIDs = []string{
	"contact_details", "no_personal_details", "profile_links", "photo",
	"summary",
	"action_verbs", "quantified_results", "short_bullets", "no_first_person", "number_style", "reverse_chronological",
	"graduation_year", "gpa", "local_diplomas",
	"no_skill_ratings", "relevant_skills", "certifications",
	"one_page", "fonts", "colors", "bold_titles",
	"file_name", "pdf_size", "company_language",
}

var ruleGroups = map[string]string{
	"contact_details": "contact", "no_personal_details": "contact", "profile_links": "contact", "photo": "contact",
	"summary":      "summary",
	"action_verbs": "experience", "quantified_results": "experience", "short_bullets": "experience",
	"no_first_person": "experience", "number_style": "experience", "reverse_chronological": "experience",
	"graduation_year": "education", "gpa": "education", "local_diplomas": "education",
	"no_skill_ratings": "skills", "relevant_skills": "skills", "certifications": "skills",
	"one_page": "layout", "fonts": "layout", "colors": "layout", "bold_titles": "layout",
	"file_name": "file", "pdf_size": "file", "company_language": "file",
}

// CheckATS runs the 24 rules. Where (a JSON path such as experience[0].description[1]) and Was point at the
// first offending text so the client can highlight it and ask ai-api for a rewrite; Now is set only when the fix
// is mechanical (the file name).
func CheckATS(in ATSInput) api.ATSCheckResponse {
	c := in.Content
	results := map[string]api.ATSRuleResult{}
	set := func(id, status string, finding *api.ATSFinding) {
		r := api.ATSRuleResult{ID: id, Group: ruleGroups[id], Status: status}
		if finding != nil && status == RuleFix {
			r.Where, r.Was, r.Now = finding.Where, finding.Was, finding.Now
		}
		results[id] = r
	}
	okOr := func(id string, f *api.ATSFinding) {
		if f == nil {
			set(id, RuleOK, nil)
		} else {
			set(id, RuleFix, f)
		}
	}

	// Contact
	var missing []string
	if strings.TrimSpace(c.Contact.Phone) == "" {
		missing = append(missing, "phone")
	}
	if strings.TrimSpace(c.Contact.Email) == "" {
		missing = append(missing, "email")
	}
	if strings.TrimSpace(c.Contact.Location) == "" {
		missing = append(missing, "location")
	}
	if len(missing) > 0 {
		okOr("contact_details", &api.ATSFinding{Where: "contact." + missing[0], Was: strings.Join(missing, ",")})
	} else {
		okOr("contact_details", nil)
	}
	okOr("no_personal_details", firstMatch(c, personalDetails))
	if len(c.Contact.Links) == 0 {
		okOr("profile_links", &api.ATSFinding{Where: "contact.links"})
	} else {
		okOr("profile_links", nil)
	}
	set("photo", RuleTip, nil)

	// Summary: two to five lines, roughly 80 to 500 characters.
	sum := strings.TrimSpace(c.Summary)
	if n := len([]rune(sum)); n < 80 || n > 500 {
		okOr("summary", &api.ATSFinding{Where: "summary", Was: sum})
	} else {
		okOr("summary", nil)
	}

	// Experience bullets
	okOr("action_verbs", firstBullet(c, func(b string) bool { return !startsWithPastVerb(b) }))
	okOr("quantified_results", quantified(c))
	okOr("short_bullets", firstBullet(c, func(b string) bool { return len([]rune(b)) > 200 }))
	if f := firstBullet(c, hasFirstPerson); f != nil {
		okOr("no_first_person", f)
	} else if hasFirstPerson(sum) {
		okOr("no_first_person", &api.ATSFinding{Where: "summary", Was: sum})
	} else {
		okOr("no_first_person", nil)
	}
	okOr("number_style", firstBullet(c, badNumberStyle))
	okOr("reverse_chronological", chronology(c))

	// Education
	okOr("graduation_year", graduation(c))
	set("gpa", RuleTip, nil)
	set("local_diplomas", RuleTip, nil)

	// Skills
	okOr("no_skill_ratings", ratings(c))
	if n := len(c.Skills); n == 0 || n > 20 {
		okOr("relevant_skills", &api.ATSFinding{Where: "skills", Was: strconv.Itoa(n)})
	} else {
		okOr("relevant_skills", nil)
	}
	if len(c.Certifications) == 0 {
		okOr("certifications", &api.ATSFinding{Where: "certifications"})
	} else {
		okOr("certifications", nil)
	}

	// Layout: fonts, colours and bold are guaranteed by every 9itchi template.
	if pages := pageCount(in.Rendered); pages > 1 {
		okOr("one_page", &api.ATSFinding{Where: "layout", Was: strconv.Itoa(pages)})
	} else {
		okOr("one_page", nil)
	}
	set("fonts", RuleOK, nil)
	set("colors", RuleOK, nil)
	set("bold_titles", RuleTip, nil)

	// File
	if in.ExportFilename == "" {
		okOr("file_name", &api.ATSFinding{Where: "contact.name"})
	} else {
		set("file_name", RuleOK, nil)
		r := results["file_name"]
		r.Now = in.ExportFilename
		results["file_name"] = r
	}
	if len(in.Rendered) >= 1<<20 {
		okOr("pdf_size", &api.ATSFinding{Where: "layout", Was: strconv.Itoa(len(in.Rendered))})
	} else {
		okOr("pdf_size", nil)
	}
	switch {
	case in.TargetLanguage == "":
		set("company_language", RuleTip, nil)
	case in.Language != "" && in.Language != in.TargetLanguage:
		okOr("company_language", &api.ATSFinding{Where: "language", Was: in.Language, Now: in.TargetLanguage})
	default:
		okOr("company_language", nil)
	}

	resp := api.ATSCheckResponse{Total: len(RuleIDs), Rules: make([]api.ATSRuleResult, 0, len(RuleIDs))}
	for _, id := range RuleIDs {
		r := results[id]
		if r.Status != RuleFix {
			resp.Passed++
		}
		resp.Rules = append(resp.Rules, r)
	}
	resp.ToFix = resp.Total - resp.Passed
	return resp
}

// bullets walks experience descriptions with their JSON path.
func bullets(c api.ResumeContent, fn func(path, text string) bool) {
	for i, e := range c.Experience {
		for j, d := range e.Description {
			if strings.TrimSpace(d) == "" {
				continue
			}
			if fn(fmt.Sprintf("experience[%d].description[%d]", i, j), strings.TrimSpace(d)) {
				return
			}
		}
	}
}

func firstBullet(c api.ResumeContent, bad func(string) bool) *api.ATSFinding {
	var f *api.ATSFinding
	bullets(c, func(path, text string) bool {
		if bad(text) {
			f = &api.ATSFinding{Where: path, Was: text}
			return true
		}
		return false
	})
	return f
}

var (
	personalDetails = regexp.MustCompile(`(?i)\b(\d{2}\s*ans\b|years?\s+old|n[ée]e?\s+le\b|date\s+de\s+naissance|date\s+of\s+birth|nationalit[ée]|nationality|mari[ée]e?\b|c[ée]libataire|married\b|single\b|divorc)`)
	firstPerson     = regexp.MustCompile(`(?i)(^|[^\pL])(je|j'|j’|moi|mon|ma|mes|i|my|me)([^\pL]|$)`)
	digitRe         = regexp.MustCompile(`\d`)
	// a lone digit 1-9 used as a count ("4 développeurs"), not a version, date, percentage or unit
	smallDigit = regexp.MustCompile(`(^|[\s(])([1-9])\s+\pL{3,}`)
	bigWords   = regexp.MustCompile(`(?i)\b(dix|onze|douze|treize|quatorze|quinze|seize|vingt|trente|quarante|cinquante|cent|mille|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty|thirty|forty|fifty|hundred|thousand)\b`)
	ratingRe   = regexp.MustCompile(`[★☆●○◉◯■□]|\d+\s*/\s*(5|10)\b|\d+\s*%`)
	yearRe     = regexp.MustCompile(`(19|20)\d{2}`)
)

func firstMatch(c api.ResumeContent, re *regexp.Regexp) *api.ATSFinding {
	if re.MatchString(c.Summary) {
		return &api.ATSFinding{Where: "summary", Was: c.Summary}
	}
	var f *api.ATSFinding
	bullets(c, func(path, text string) bool {
		if re.MatchString(text) {
			f = &api.ATSFinding{Where: path, Was: text}
			return true
		}
		return false
	})
	return f
}

func hasFirstPerson(s string) bool { return firstPerson.MatchString(s) }

func badNumberStyle(s string) bool { return smallDigit.MatchString(s) || bigWords.MatchString(s) }

// quantified fails when fewer than half of the bullets contain a figure, pointing at the first one without.
func quantified(c api.ResumeContent) *api.ATSFinding {
	total, with := 0, 0
	var first *api.ATSFinding
	bullets(c, func(path, text string) bool {
		total++
		if digitRe.MatchString(text) || strings.Contains(text, "[NOMBRE]") || strings.Contains(text, "[NUMBER]") {
			with++
		} else if first == nil {
			first = &api.ATSFinding{Where: path, Was: text}
		}
		return false
	})
	if total == 0 {
		return &api.ATSFinding{Where: "experience"}
	}
	if with*2 >= total {
		return nil
	}
	return first
}

// irregular English past forms and French participles that do not end in é/i/u/t/s.
var irregularPast = map[string]bool{
	"built": true, "led": true, "made": true, "wrote": true, "ran": true, "won": true, "drove": true, "taught": true,
	"grew": true, "set": true, "cut": true, "got": true, "gave": true, "took": true, "found": true, "began": true,
	"brought": true, "held": true, "kept": true, "sold": true, "spent": true, "met": true, "chose": true, "rebuilt": true,
	"put": true, "shipped": true, "sped": true, "understood": true, "overcame": true, "oversaw": true, "undertook": true,
	"conçu": true, "mis": true, "pris": true, "écrit": true, "fait": true, "construit": true, "produit": true,
	"traduit": true, "réduit": true, "introduit": true, "conduit": true, "résolu": true, "obtenu": true, "soutenu": true,
	"maintenu": true, "acquis": true, "remis": true, "permis": true, "ouvert": true, "offert": true, "découvert": true,
	"couvert": true, "atteint": true, "rejoint": true, "suivi": true, "établi": true, "réussi": true, "fourni": true,
	"défini": true, "dirigé": true,
}

// startsWithPastVerb is a heuristic: the first word is a French past participle (Développé, Conçu, Mis en place...)
// or an English past tense (Built, Designed...).
func startsWithPastVerb(s string) bool {
	s = strings.TrimLeftFunc(s, func(r rune) bool { return !unicode.IsLetter(r) })
	word := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && r != '-' })
	if len(word) == 0 {
		return false
	}
	w := strings.ToLower(word[0])
	if irregularPast[w] {
		return true
	}
	for _, suf := range []string{"é", "ée", "és", "ées", "ed"} {
		if strings.HasSuffix(w, suf) && len([]rune(w)) > len([]rune(suf))+1 {
			return true
		}
	}
	return false
}

// chronology wants a start date on every experience and the most recent first.
func chronology(c api.ResumeContent) *api.ATSFinding {
	prev := 1 << 30
	for i, e := range c.Experience {
		k := dateKey(e.Start)
		if k == 0 {
			return &api.ATSFinding{Where: fmt.Sprintf("experience[%d].start", i), Was: e.Start}
		}
		if strings.TrimSpace(e.End) == "" {
			return &api.ATSFinding{Where: fmt.Sprintf("experience[%d].end", i), Was: e.End}
		}
		if k > prev {
			return &api.ATSFinding{Where: fmt.Sprintf("experience[%d].start", i), Was: e.Start}
		}
		prev = k
	}
	return nil
}

var monthNames = map[string]int{
	"jan": 1, "janv": 1, "janvier": 1, "january": 1, "feb": 2, "fev": 2, "fév": 2, "févr": 2, "fevrier": 2, "février": 2, "february": 2,
	"mar": 3, "mars": 3, "march": 3, "apr": 4, "avr": 4, "avril": 4, "april": 4, "may": 5, "mai": 5,
	"jun": 6, "juin": 6, "june": 6, "jul": 7, "juil": 7, "juillet": 7, "july": 7, "aug": 8, "aout": 8, "août": 8, "august": 8,
	"sep": 9, "sept": 9, "septembre": 9, "september": 9, "oct": 10, "octobre": 10, "october": 10,
	"nov": 11, "novembre": 11, "november": 11, "dec": 12, "déc": 12, "decembre": 12, "décembre": 12, "december": 12,
}

// dateKey turns "2025-06", "06/2025", "juin 2025" or "2025" into year*100+month (0 when there is no year).
func dateKey(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	y := yearRe.FindString(s)
	if y == "" {
		return 0
	}
	year, _ := strconv.Atoi(y)
	month := 0
	rest := strings.Replace(s, y, " ", 1)
	for _, tok := range strings.FieldsFunc(rest, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if m, ok := monthNames[strings.TrimSuffix(tok, ".")]; ok {
			month = m
			break
		}
		if n, err := strconv.Atoi(tok); err == nil && n >= 1 && n <= 12 {
			month = n
			break
		}
	}
	return year*100 + month
}

func graduation(c api.ResumeContent) *api.ATSFinding {
	if len(c.Education) == 0 {
		return &api.ATSFinding{Where: "education"}
	}
	for i, e := range c.Education {
		if !yearRe.MatchString(e.End) {
			return &api.ATSFinding{Where: fmt.Sprintf("education[%d].end", i), Was: e.End}
		}
	}
	return nil
}

func ratings(c api.ResumeContent) *api.ATSFinding {
	for i, s := range c.Skills {
		if ratingRe.MatchString(s) {
			return &api.ATSFinding{Where: fmt.Sprintf("skills[%d]", i), Was: s}
		}
	}
	for i, s := range c.Languages {
		if ratingRe.MatchString(s) {
			return &api.ATSFinding{Where: fmt.Sprintf("languages[%d]", i), Was: s}
		}
	}
	return nil
}

// pageCount reads the number of pages of a rendered PDF (0 when unknown).
func pageCount(data []byte) (n int) {
	if len(data) == 0 {
		return 0
	}
	defer func() {
		if recover() != nil {
			n = 0
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0
	}
	return r.NumPage()
}

// RenderForCheck renders the CV with the PDF exporter so the page and size rules see what students download.
func RenderForCheck(ctx context.Context, c api.ResumeContent, layout, language string) []byte {
	out, err := NewPDFExporter().Render(ctx, c, layout, RenderOptions{Language: language})
	if err != nil {
		return nil
	}
	return out
}

// sortedIDs is used by tests to check the rule list has no duplicates.
func sortedIDs() []string {
	ids := append([]string(nil), RuleIDs...)
	sort.Strings(ids)
	return ids
}
