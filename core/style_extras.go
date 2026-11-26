package core

import (
	"fmt"
	"strings"

	"9itchi-resume-api/models/api"
)

// skillSep is what the style puts between skills; def is the template's own separator.
func skillSep(s *api.CVStyle, def string) string {
	if s != nil {
		switch s.SkillsStyle {
		case "comma":
			return ", "
		case "bullet":
			return "  •  "
		case "bar":
			return "  |  "
		}
	}
	return def
}

// inlineLanguages reports whether the languages are one paragraph "**English** (professional), ...".
func inlineLanguages(s *api.CVStyle) bool { return s != nil && s.LanguagesStyle == "inline" }

// headingRuleWidth is the rule under section headings in mm: def is the template's own rule (0 = none), "thick" draws a
// heavier one even where the template has none, "none" removes it.
func headingRuleWidth(s *api.CVStyle, def float64) float64 {
	if s != nil {
		switch s.HeadingRule {
		case "none":
			return 0
		case "thick":
			return 0.55
		}
	}
	return def
}

// headingRuleSz is headingRuleWidth for a DOCX paragraph border, in eighths of a point.
func headingRuleSz(s *api.CVStyle, def int) int {
	if s != nil {
		switch s.HeadingRule {
		case "none":
			return 0
		case "thick":
			return 12
		}
	}
	return def
}

// sectionGap is the space before a section heading in mm: the user's, else the template's def.
func sectionGap(s *api.CVStyle, def float64) float64 {
	if s != nil && s.SectionGap != nil {
		return *s.SectionGap
	}
	return def
}

// sectionGapTw is sectionGap for DOCX, in twips; def is the template's own.
func sectionGapTw(s *api.CVStyle, def int) int {
	if s != nil && s.SectionGap != nil {
		return mm(*s.SectionGap)
	}
	return def
}

// roleGroups splits the experience into runs of consecutive entries at the same company when grouping is on;
// otherwise (and for a lone entry) each entry is its own group.
func roleGroups(exps []api.Experience, s *api.CVStyle) [][]api.Experience {
	on := s != nil && s.GroupRoles != nil && *s.GroupRoles
	var out [][]api.Experience
	for _, e := range exps {
		if n := len(out); on && n > 0 && sameCompany(out[n-1][0].Company, e.Company) {
			out[n-1] = append(out[n-1], e)
			continue
		}
		out = append(out, []api.Experience{e})
	}
	return out
}

func sameCompany(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	return a != "" && strings.EqualFold(a, b)
}

// groupURL is the first web link among the roles of a group.
func groupURL(g []api.Experience) string {
	for _, e := range g {
		if strings.TrimSpace(e.URL) != "" {
			return e.URL
		}
	}
	return ""
}

// roleIndent is how far the roles of a group sit right of their company, mm.
const roleIndent = 4.0

var cvRule = rgb{204, 204, 204} // #CCCCCC, the line along grouped roles

// languagesInline writes the languages as one wrapped paragraph: the name in bold, the level after it in parentheses.
func (d *cv) languagesInline(size float64, nameCol, levelCol rgb) {
	pdf := d.pdf
	left, top, right, _ := pdf.GetMargins()
	pdf.SetLeftMargin(d.x)
	pdf.SetRightMargin(210 - d.x - d.w)
	pdf.SetX(d.x)
	first := true
	for _, l := range d.c.Languages {
		name, level := splitLanguage(l)
		if name == "" {
			continue
		}
		if !first {
			d.font(sans, "", size, levelCol)
			pdf.Write(d.lh(size), ", ")
		}
		first = false
		d.font(sans, "B", size, nameCol)
		pdf.Write(d.lh(size), cleanText(name))
		if level != "" {
			d.font(sans, "", size, levelCol)
			pdf.Write(d.lh(size), cleanText(" ("+level+")"))
		}
	}
	pdf.Ln(d.lh(size))
	pdf.SetMargins(left, top, right)
}

// groupedRoles draws roles held at one company: the company once, then each role indented with a thin line along
// their left side. between is the layout's space between roles.
func (d *cv) groupedRoles(g []api.Experience, st entryStyle, body float64, ink rgb, between func()) {
	pdf := d.pdf
	size := d.lk.entry
	d.need(3 * d.lh(size))
	d.row(d.segs(linked(st.bold(g[0].Company), groupURL(g))), st.date, "", size)
	x, w := d.x, d.w
	d.x, d.w = x+roleIndent, w-roleIndent
	page, y0 := pdf.PageNo(), pdf.GetY()
	for i, e := range g {
		if i > 0 {
			between()
		}
		segs := []seg{st.bold(e.Title)}
		if e.Kind != "" {
			segs = append(segs, st.kind(" ("+e.Kind+")"))
		}
		if e.Location != "" {
			segs = append(segs, st.loc(prefix(", ", e.Location)))
		}
		d.row(d.segs(segs...), st.date, dates(e.Start, e.End), size)
		d.bullets(e.Description, body, ink)
		if i == 0 && pdf.PageNo() != page {
			// the first role started on the next page: the line starts there too
			page, y0 = pdf.PageNo(), d.lk.my
		}
	}
	d.x, d.w = x, w
	d.vline(x+1, page, y0, pdf.PageNo(), pdf.GetY())
}

// vline draws a thin vertical line at x from (page0, y0) to (page1, y1), across page breaks.
func (d *cv) vline(x float64, page0 int, y0 float64, page1 int, y1 float64) {
	pdf := d.pdf
	_, ph := pdf.GetPageSize()
	_, tm, _, _ := pdf.GetMargins()
	_, bm := pdf.GetAutoPageBreak()
	for p := page0; p <= page1; p++ {
		top, bottom := tm, ph-bm
		if p == page0 {
			top = y0
		}
		if p == page1 {
			bottom = y1
		}
		if bottom <= top {
			continue
		}
		pdf.SetPage(p)
		pdf.SetDrawColor(cvRule.r, cvRule.g, cvRule.b) // after SetPage: each page stream needs its own
		pdf.SetLineWidth(0.3)
		pdf.Line(x, top, x, bottom)
	}
	pdf.SetPage(page1)
}

// docxRoleGroup is groupedRoles for DOCX: the company paragraph, then each role and its bullets indented with a left
// border. first is the look of the company paragraph.
func docxRoleGroup(width int, g []api.Experience, st docxEntryStyle, body rp, s *api.CVStyle) string {
	ind := mm(roleIndent)
	var out strings.Builder
	first := st.first
	first.keepNext = true
	out.WriteString(rowP(width, []string{run(g[0].Company, st.bold)}, "", st.date, first))
	glyph, left, hang := listXML(s)
	for i, e := range g {
		before := 0
		if i > 0 {
			before = 60
		}
		runs := []string{run(e.Title, st.bold)}
		if e.Kind != "" {
			runs = append(runs, run(" ("+e.Kind+")", st.kind))
		}
		if e.Location != "" {
			runs = append(runs, run(prefix(", ", e.Location), st.loc))
		}
		out.WriteString(rowP(width, runs, dates(e.Start, e.End), st.date, pp{before: before, keepNext: true, indLeft: ind, borderLeft: "CCCCCC"}))
		for _, l := range nonEmpty(e.Description...) {
			out.WriteString(para(pp{indLeft: ind + left, hanging: hang, leftTab: ind + left, borderLeft: "CCCCCC"},
				run(glyph, body), tabRun, run(l, body)))
		}
	}
	return out.String()
}

// docxLanguagesInline is the languages as one paragraph of runs: bold names, levels in parentheses.
func docxLanguagesInline(langs []string, name, level rp, p pp) string {
	var runs []string
	for _, l := range langs {
		n, lv := splitLanguage(l)
		if n == "" {
			continue
		}
		if len(runs) > 0 {
			runs = append(runs, run(", ", level))
		}
		runs = append(runs, run(n, name))
		if lv != "" {
			runs = append(runs, run(fmt.Sprintf(" (%s)", lv), level))
		}
	}
	return para(p, runs...)
}
