package core

import (
	"regexp"
	"strings"

	"9itchi-resume-api/models/api"
)

// visibleEntries drops the entries the user hid; they stay in the stored content.
func visibleEntries(c api.ResumeContent) api.ResumeContent {
	c.Experience = keep(c.Experience, func(e api.Experience) bool { return !e.Hidden })
	c.Education = keep(c.Education, func(e api.Education) bool { return !e.Hidden })
	c.Projects = keep(c.Projects, func(p api.Project) bool { return !p.Hidden })
	c.Certifications = keep(c.Certifications, func(x api.Certification) bool { return !x.Hidden })
	c.Summary = plain(c.Summary)
	for i := range c.Experience {
		c.Experience[i].Description = plainAll(c.Experience[i].Description)
	}
	for i := range c.Projects {
		c.Projects[i].Description = plainAll(c.Projects[i].Description)
	}
	return c
}

var (
	linkMark = regexp.MustCompile(`\[([^\]]+)\]\(https?://[^\s)]+\)`)
	boldMark = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	ulMark   = regexp.MustCompile(`__([^_]+)__`)
	itMark   = regexp.MustCompile(`\*([^*]+)\*`)
)

// plain removes the inline marks of the editor (bold, italic, underline, links) so they never print as symbols.
func plain(s string) string {
	s = linkMark.ReplaceAllString(s, "$1")
	s = boldMark.ReplaceAllString(s, "$1")
	s = ulMark.ReplaceAllString(s, "$1")
	return itMark.ReplaceAllString(s, "$1")
}

func plainAll(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = plain(x)
	}
	return out
}

func keep[T any](xs []T, ok func(T) bool) []T {
	if len(xs) == 0 {
		return xs
	}
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if ok(x) {
			out = append(out, x)
		}
	}
	return out
}

// safeURL keeps only web links, so a stored value can never become a script or file link.
func safeURL(u string) string {
	u = strings.TrimSpace(u)
	l := strings.ToLower(u)
	if strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") {
		return u
	}
	return ""
}

var techLabel = map[string]string{"fr": "Technologies : ", "en": "Technologies: "}

// techText is the project's technology line shown under its description, empty when the user hid it.
func techText(p api.Project, lang string) string {
	t := strings.TrimSpace(p.Tech)
	if t == "" || p.HideTech {
		return ""
	}
	l, ok := techLabel[lang]
	if !ok {
		l = techLabel["fr"]
	}
	return l + t
}
