package core

import "9itchi-resume-api/models/api"

// entryLayout is how the title and the subtitle of an experience or education entry are laid out.
type entryLayout struct {
	inline   bool // one row "first â€” second"; otherwise two rows
	subFirst bool // the subtitle (company, school) comes before the title (job title, degree)
}

// entryDefaults is what each template does today, per layout and section.
var entryDefaults = map[string]map[string]entryLayout{
	"classic": {"experience": {}, "education": {}},
	"modern":  {"experience": {inline: true}, "education": {inline: true, subFirst: true}},
	"compact": {"experience": {inline: true, subFirst: true}, "education": {inline: true, subFirst: true}},
}

// entryLook resolves the layout of a section ("experience" or "education"). ok is false when the user chose neither
// entry_layout nor entry_order, so the caller keeps the template's own drawing code.
func entryLook(layout, section string, s *api.CVStyle) (l entryLayout, ok bool) {
	if s == nil || (s.EntryLayout == "" && s.EntryOrder == "") {
		return entryLayout{}, false
	}
	l = entryDefaults[layout][section]
	switch s.EntryLayout {
	case "stacked":
		l.inline = false
	case "inline":
		l.inline = true
	}
	switch s.EntryOrder {
	case "title_first":
		l.subFirst = false
	case "subtitle_first":
		l.subFirst = true
	}
	return l, true
}

// entryParts are the texts of one entry: its title (job title, degree), subtitle (company, school), kind and link.
type entryParts struct {
	title, subtitle, kind, url string
}

// subtitleKind is the subtitle followed by its kind in parentheses.
func (p entryParts) subtitleKind() string {
	if p.kind == "" {
		return p.subtitle
	}
	return join(" ", p.subtitle, "("+p.kind+")")
}

// kindSuffix is the kind in parentheses, spaced from the subtitle it follows.
func (p entryParts) kindSuffix() string {
	if p.subtitle == "" {
		return "(" + p.kind + ")"
	}
	return " (" + p.kind + ")"
}

// firstSecond are the texts of the first and second position; the kind stays with the subtitle.
func (p entryParts) firstSecond(l entryLayout) (first, second string) {
	if l.subFirst {
		return p.subtitleKind(), p.title
	}
	return p.title, p.subtitleKind()
}

// entryStyle are the segment and font builders a PDF template uses for its entry rows.
type entryStyle struct {
	bold, sub, kind, loc func(string) seg // first position, stacked second row, kind, location after the text
	date, locR           func()           // fonts of the right-hand dates and of the location on the second row
}

// entryRows draws the header of an experience or education entry in the user's layout.
func (d *cv) entryRows(l entryLayout, p entryParts, dateText, loc string, st entryStyle, size float64) {
	first, second := p.firstSecond(l)
	if l.inline {
		segs := []seg{linked(st.bold(join(" — ", first, second)), p.url)}
		if loc != "" {
			segs = append(segs, st.loc(prefix(", ", loc)))
		}
		d.row(d.segs(segs...), st.date, dateText, size)
		return
	}
	d.row(d.segs(linked(st.bold(first), p.url)), st.date, dateText, size)
	var rest []seg
	switch {
	case l.subFirst:
		if p.title != "" {
			rest = append(rest, st.sub(p.title))
		}
	default:
		if p.subtitle != "" {
			rest = append(rest, st.sub(p.subtitle))
		}
		if p.kind != "" {
			rest = append(rest, st.kind(p.kindSuffix()))
		}
	}
	if len(rest) > 0 || loc != "" {
		d.row(d.segs(rest...), st.locR, loc, size)
	}
}

// docxEntryStyle are the run properties a DOCX template uses for its entry rows.
type docxEntryStyle struct {
	bold, sub, kind, loc rp // first position, stacked second row, kind, location
	date                 rp // right-hand dates
	first                pp // paragraph of the first row
}

// entryXML is the header of an experience or education entry in the user's layout.
func entryXML(width int, l entryLayout, p entryParts, dateText, loc string, st docxEntryStyle) string {
	first, second := p.firstSecond(l)
	if l.inline {
		left := []string{run(join(" — ", first, second), st.bold)}
		if loc != "" {
			left = append(left, run(prefix(", ", loc), st.loc))
		}
		return rowP(width, left, dateText, st.date, st.first)
	}
	out := rowP(width, []string{run(first, st.bold)}, dateText, st.date, st.first)
	var rest []string
	switch {
	case l.subFirst:
		if p.title != "" {
			rest = append(rest, run(p.title, st.sub))
		}
	default:
		if p.subtitle != "" {
			rest = append(rest, run(p.subtitle, st.sub))
		}
		if p.kind != "" {
			rest = append(rest, run(p.kindSuffix(), st.kind))
		}
	}
	if len(rest) > 0 || loc != "" {
		out += rowP(width, rest, loc, st.loc, pp{})
	}
	return out
}
