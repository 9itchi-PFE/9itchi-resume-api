package core

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"9itchi-resume-api/models/api"
)

// sectionHeadings are lowercase words that name a section heading in a French, English or Arabic CV. A heading line
// containing one of them belongs to that section; the lists are tried in this order, so "certifications and
// publications" is certifications and "academic projects" is projects.
var sectionHeadings = []struct {
	key   string
	words []string
}{
	{"certifications", []string{"certification", "certificat", "licenses", "الشهادات"}},
	{"projects", []string{"project", "projet", "المشاريع"}},
	{"experience", []string{"experience", "expérience", "employment", "work history", "internship", "stages", "parcours professionnel", "الخبرة", "الخبرات"}},
	{"education", []string{"education", "formation", "études", "etudes", "academic background", "parcours académique", "diplômes", "التعليم", "التكوين"}},
	{"skills", []string{"skill", "compétence", "competence", "المهارات"}},
	{"languages", []string{"language", "langue", "اللغات"}},
	{"summary", []string{"summary", "profile", "profil", "about me", "à propos", "objective", "objectif", "نبذة", "الملخص"}},
}

// maxHeadingWords bounds the length of a heading line: longer lines are content that happens to use a heading word.
const maxHeadingWords = 4

// InferSectionOrder returns the section keys (see api.SectionKeys, extras as "extra:N") in the order their headings
// appear in the text of the uploaded CV, so the imported CV keeps the student's order. It returns nil when fewer than two
// headings are found; sections it does not find are placed by ResumeContent.SectionOrder after the found ones.
func InferSectionOrder(text string, c api.ResumeContent) []string {
	pos := map[string]int{}
	for i, line := range strings.Split(text, "\n") {
		h := headingText(line)
		if h == "" {
			continue
		}
		if key := headingKey(h, c.Extra); key != "" {
			if _, seen := pos[key]; !seen {
				pos[key] = i
			}
		}
	}
	if len(pos) < 2 {
		return nil
	}
	keys := make([]string, 0, len(pos))
	for k := range pos {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return pos[keys[a]] < pos[keys[b]] })
	return keys
}

// headingText returns the line lowercased when it can be a section heading (a few words, no digits, no e-mail), or "".
func headingText(line string) string {
	line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), ":•-–—|"))
	if line == "" || len(strings.Fields(line)) > maxHeadingWords || strings.ContainsAny(line, "@") {
		return ""
	}
	for _, r := range line {
		if unicode.IsDigit(r) {
			return ""
		}
	}
	return strings.ToLower(line)
}

// headingKey maps a heading to its section key: an extra section titled exactly like it (the parser keeps the heading
// as the title), then the known headings, then an extra section whose title it contains.
func headingKey(h string, extra []api.ExtraSection) string {
	titles := make([]string, len(extra))
	for i, e := range extra {
		titles[i] = strings.ToLower(strings.TrimSpace(e.Title))
		if titles[i] == h {
			return "extra:" + strconv.Itoa(i)
		}
	}
	for _, s := range sectionHeadings {
		for _, w := range s.words {
			if strings.Contains(h, w) {
				return s.key
			}
		}
	}
	for i, t := range titles {
		if len([]rune(t)) >= 4 && strings.Contains(h, t) {
			return "extra:" + strconv.Itoa(i)
		}
	}
	return ""
}
