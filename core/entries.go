package core

import (
	"regexp"
	"strings"

	"9itchi-resume-api/models/api"
)

// locMark wraps a location folded into the dates so the PDF can color the two apart; it is invisible and never printed.
const locMark = "\u2063"

// visibleEntries drops the entries the user hid; they stay in the stored content.
func visibleEntries(c api.ResumeContent) api.ResumeContent { return visibleEntriesMark(c, false) }

// visibleEntriesMark is visibleEntries; with mark the folded locations are wrapped in locMark.
func visibleEntriesMark(c api.ResumeContent, mark bool) api.ResumeContent {
	c.Experience = keep(c.Experience, func(e api.Experience) bool { return !e.Hidden })
	c.Education = keep(c.Education, func(e api.Education) bool { return !e.Hidden })
	c.Projects = keep(c.Projects, func(p api.Project) bool { return !p.Hidden })
	c.Certifications = keep(c.Certifications, func(x api.Certification) bool { return !x.Hidden })
	c.Summary = plain(c.Summary)
	beside := c.Style != nil && c.Style.LocationPos == "date"
	order := c.Style != nil && c.Style.MetaOrder == "location_first"
	sep := metaSep(c.Style)
	for i := range c.Experience {
		e := &c.Experience[i]
		e.Description = plainAll(e.Description)
		e.Title, e.Company, e.Kind, e.Location = plain(e.Title), plain(e.Company), plain(e.Kind), plain(e.Location)
		if beside && strings.TrimSpace(e.Location) != "" {
			e.Start, e.End, e.Location = metaText(e.Start, e.End, e.Location, order, mark, sep), "", ""
		}
	}
	for i := range c.Education {
		e := &c.Education[i]
		e.School, e.Degree, e.Location = plain(e.School), plain(e.Degree), plain(e.Location)
		if beside && strings.TrimSpace(e.Location) != "" {
			e.Start, e.End, e.Location = metaText(e.Start, e.End, e.Location, order, mark, sep), "", ""
		}
	}
	for i := range c.Projects {
		p := &c.Projects[i]
		p.Description = plainAll(p.Description)
		p.Name, p.Tech = plain(p.Name), plain(p.Tech)
		loc := ""
		if beside {
			loc = strings.TrimSpace(p.Location)
		}
		if r := metaText(p.Start, p.End, loc, order, mark, sep); r != "" {
			p.Year = r
		}
	}
	for i := range c.Certifications {
		c.Certifications[i].Name, c.Certifications[i].Issuer = plain(c.Certifications[i].Name), plain(c.Certifications[i].Issuer)
	}
	return c
}

// metaText is the right-hand text of an entry: its dates and, when given, the location beside them; mirrors the client's metaText.
// metaSep is what the style puts between the dates and the location: a bullet by default, a hyphen or a bar.
func metaSep(s *api.CVStyle) string {
	if s != nil {
		switch s.MetaSep {
		case "hyphen":
			return " - "
		case "bar":
			return " | "
		}
	}
	return " · "
}

func metaText(start, end, loc string, locationFirst, mark bool, sep string) string {
	dates := join(" — ", start, end)
	if strings.TrimSpace(loc) == "" {
		return dates
	}
	if mark {
		loc = locMark + strings.TrimSpace(loc) + locMark
	}
	if locationFirst {
		return join(sep, loc, dates)
	}
	return join(sep, dates, loc)
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
