package core

import (
	"fmt"
	"strconv"
	"strings"
)

// The Word version of the sidebar, split and banner layouts: the columns are the cells of a borderless table and the
// fills are cell shading. Word lays the cells out itself, so the side fill only reaches as far down as the row.

func hexOf(c rgb) string { return fmt.Sprintf("%02X%02X%02X", c.r, c.g, c.b) }

// docxInk are the colors (hex) of one column.
type docxInk struct{ ink, title, sub, loc, date, head, rule string }

func columnInk(c colInk) docxInk {
	return docxInk{ink: hexOf(c.ink), title: hexOf(c.title), sub: hexOf(c.sub), loc: hexOf(c.loc), date: hexOf(c.date), head: hexOf(c.head), rule: hexOf(c.rule)}
}

// docxCell is one cell of a layout table.
type docxCell struct {
	w          int
	fill       string // shading, hex
	borderLeft string // thin rule left of the cell, hex
	mar        [4]int // top, left, bottom, right, twips
	vAlign     string
	body       string
}

// docxTable is a borderless one-row table; rowH > 0 is its minimum height.
func docxTable(rowH int, cells ...docxCell) string {
	total := 0
	var grid, tcs strings.Builder
	for _, c := range cells {
		total += c.w
		fmt.Fprintf(&grid, `<w:gridCol w:w="%d"/>`, c.w)
		fmt.Fprintf(&tcs, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/>`, c.w)
		if c.borderLeft != "" {
			fmt.Fprintf(&tcs, `<w:tcBorders><w:left w:val="single" w:sz="4" w:space="0" w:color="%s"/></w:tcBorders>`, c.borderLeft)
		}
		if c.fill != "" {
			fmt.Fprintf(&tcs, `<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, c.fill)
		}
		fmt.Fprintf(&tcs, `<w:tcMar><w:top w:w="%d" w:type="dxa"/><w:left w:w="%d" w:type="dxa"/><w:bottom w:w="%d" w:type="dxa"/><w:right w:w="%d" w:type="dxa"/></w:tcMar>`,
			c.mar[0], c.mar[1], c.mar[2], c.mar[3])
		if c.vAlign != "" {
			fmt.Fprintf(&tcs, `<w:vAlign w:val="%s"/>`, c.vAlign)
		}
		body := c.body
		if body == "" {
			body = para(pp{})
		}
		tcs.WriteString(`</w:tcPr>` + body + `</w:tc>`)
	}
	tr := `<w:tr>`
	if rowH > 0 {
		tr += `<w:trPr><w:trHeight w:val="` + strconv.Itoa(rowH) + `" w:hRule="atLeast"/></w:trPr>`
	}
	return `<w:tbl><w:tblPr><w:tblW w:w="` + strconv.Itoa(total) + `" w:type="dxa"/><w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:bottom w:val="nil"/><w:right w:val="nil"/><w:insideH w:val="nil"/><w:insideV w:val="nil"/></w:tblBorders><w:tblLayout w:type="fixed"/><w:tblCellMar><w:left w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr>` +
		`<w:tblGrid>` + grid.String() + `</w:tblGrid>` + tr + tcs.String() + `</w:tr></w:tbl>`
}

// tinyP keeps two tables apart (and ends the body after a table) without adding height.
const tinyP = `<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="20" w:lineRule="exact"/></w:pPr></w:p>`

func (d *docx) twoColumns(layout string, sp twoColSpec, branded bool) string {
	lk := newLook(layout, d.c.Style)
	if sp.photoRight && lk.style.PhotoPos == "" {
		lk.photoRight = true
	}
	accent := sp.accentFill(lk)
	mainCols := columnInk(mainInk(lk, sp))
	sideInk := mainCols
	if sp.fill == "accent" {
		sideInk = columnInk(lightInk(accent))
	}
	nameCol, titleCol := hexOf(lk.col("name", sp.accent)), hexOf(lk.col("title", cvMuted))
	textCol, linkCol := hexOf(lk.ink(cvGrey)), hexOf(lk.col("links", sp.accent))
	if sp.header == "band" {
		li := columnInk(lightInk(accent))
		nameCol, titleCol, textCol, linkCol = li.ink, li.ink, li.sub, li.ink
	}
	nameP := rp{size: lk.name, bold: true, color: nameCol}
	if lk.nameSerif {
		nameP.font = "Cambria"
	}
	titleP := rp{size: lk.title, color: titleCol}
	align := ""
	if lk.center {
		align = "center"
	}
	inline := lk.style.HeaderLayout == "inline" && !lk.center && d.c.Contact.Headline != ""
	header := func() string {
		if inline {
			return para(pp{align: align, after: 40}, run(d.c.Contact.Name+"  ", nameP), run(d.c.Contact.Headline, titleP))
		}
		out := para(pp{align: align, after: 20}, run(d.c.Contact.Name, nameP))
		if d.c.Contact.Headline != "" {
			out += para(pp{align: align, after: 40}, run(d.c.Contact.Headline, titleP))
		}
		return out
	}
	contact := func() string { return para(pp{align: align}, d.contactRuns(lk.base-1, textCol, linkCol)...) }

	order := d.c.SectionOrder()
	column := func(side bool, ci docxInk, width int) string {
		secs := d.columnXML(layout, lk, sp, ci, width)
		var out strings.Builder
		for _, k := range order {
			if sp.sideKeys[k] == side {
				out.WriteString(secs[k])
			}
		}
		return out.String()
	}
	bottom := 0
	if branded {
		bottom = mm(10)
	}
	switch sp.header {
	case "full": // split
		width := pageW - 2*mm(lk.mx)
		if pic := d.drawing(2*sp.photoR, ""); pic != "" {
			pw := mm(2*sp.photoR + 6)
			cells := []docxCell{{w: pw, vAlign: "center", body: para(pp{}, pic)}, {w: width - pw, vAlign: "center", body: header() + contact()}}
			if lk.photoRight {
				cells[0], cells[1] = cells[1], cells[0]
			}
			d.add(docxTable(0, cells...) + tinyP)
		} else {
			d.add(header() + contact())
		}
		gutter := mm(4)
		mainW := int(float64(width) * (1 - sp.side))
		d.add(docxTable(0,
			docxCell{w: mainW, mar: [4]int{mm(2), 0, 0, gutter}, body: column(false, mainCols, mainW-gutter)},
			docxCell{w: width - mainW, borderLeft: hexOf(cvDivider), mar: [4]int{mm(2), gutter, 0, 0}, body: column(true, sideInk, width-mainW-gutter)}))
		d.add(tinyP)
		return sectPr(mm(lk.my), mm(lk.mx), mm(lk.my), mm(lk.mx))
	case "band": // banner
		pad, mx := mm(bandPad), mm(lk.mx)
		cells := []docxCell{{w: pageW, fill: hexOf(accent), vAlign: "center", mar: [4]int{pad, mx, pad, mx}, body: header() + contact()}}
		if pic := d.drawing(2*sp.photoR, ""); pic != "" {
			pw := mm(2*sp.photoR) + 2*mx
			cells[0].w -= pw
			pc := docxCell{w: pw, fill: hexOf(accent), vAlign: "center", mar: [4]int{pad, mx, pad, mx}, body: para(pp{align: "right"}, pic)}
			if lk.photoRight {
				cells = append(cells, pc)
			} else {
				pc.body = para(pp{}, pic)
				cells = append([]docxCell{pc}, cells...)
			}
		}
		d.add(docxTable(0, cells...) + tinyP)
		sideW := int(float64(pageW) * sp.side)
		pad7 := mm(7)
		d.add(docxTable(pageH-mm(70),
			docxCell{w: sideW, fill: hexOf(cvTint), mar: [4]int{mm(4), pad7, 0, pad7}, body: column(true, sideInk, sideW-2*pad7)},
			docxCell{w: pageW - sideW, mar: [4]int{mm(4), mx, 0, mx}, body: column(false, mainCols, pageW-sideW-2*mx)}))
		d.add(tinyP)
		return sectPr(0, 0, bottom, 0)
	}
	// sidebar: photo and contact atop the filled side column, name atop the main one.
	sideW := int(float64(pageW) * sp.side)
	pad, mx, top := mm(7), mm(lk.mx), mm(lk.my)
	var s strings.Builder
	if pic := d.drawing(2*sp.photoR, ""); pic != "" {
		s.WriteString(para(pp{align: "center", after: 120}, pic))
	}
	for _, it := range nonEmpty(d.c.Contact.Email, d.c.Contact.Phone, d.c.Contact.Location) {
		s.WriteString(para(pp{}, run(it, rp{size: lk.base - 0.5, color: sideInk.ink})))
	}
	for _, l := range nonEmpty(d.c.Contact.Links...) {
		s.WriteString(para(pp{}, run(shortLink(l), rp{size: lk.base - 0.5, color: sideInk.ink})))
	}
	s.WriteString(column(true, sideInk, sideW-2*pad))
	rowH := pageH - 60
	if branded {
		rowH = pageH - mm(12)
	}
	d.add(docxTable(rowH,
		docxCell{w: sideW, fill: hexOf(accent), mar: [4]int{top, pad, 0, pad}, body: s.String()},
		docxCell{w: pageW - sideW, mar: [4]int{top, mx, 0, mx}, body: header() + column(false, mainCols, pageW-sideW-2*mx)}))
	d.add(tinyP)
	return sectPr(0, 0, bottom, 0)
}

// columnXML is the XML of every section (extras included) in the colors of one column, keyed like SectionOrder.
func (d *docx) columnXML(layout string, lk look, sp twoColSpec, ci docxInk, width int) map[string]string {
	st := lk.style
	b := lk.base
	ink := rp{size: b, color: ci.ink}
	bold := rp{size: b, color: ci.title, bold: true}
	sub := rp{size: b, color: ci.sub, italic: lk.subStyle == "I", bold: lk.subStyle == "B"}
	date := rp{size: lk.entry, color: ci.date, italic: lk.dateStyle == "I", bold: lk.dateStyle == "B"}
	ruleDef := 0
	if sp.headRule > 0 {
		ruleDef = 6
	}
	headP := pp{before: sectionGapTw(st, 160), after: 40, keepNext: true}
	if sz := headingRuleSz(st, ruleDef); sz > 0 {
		headP.borderBottom, headP.borderSz = ci.rule, sz
	}
	headR := rp{size: lk.head, bold: lk.headFont("B") == "B", caps: lk.upper, color: ci.head}
	var w strings.Builder
	heading := func(t string) { w.WriteString(para(headP, run(t, headR))) }
	bullets := func(lines []string) {
		for _, l := range nonEmpty(lines...) {
			w.WriteString(para(pp{bullet: true}, run(l, ink)))
		}
	}
	entry := func(section string) entryLayout {
		ly, ok := entryLook(layout, section, st)
		if !ok {
			ly = entryDefaults[layout][section]
		}
		return ly
	}
	loc := sub
	loc.color = ci.loc
	es := docxEntryStyle{bold: bold, sub: sub, kind: sub, loc: loc, date: date}
	out := map[string]string{}
	cut := func(key string) {
		out[key] = w.String()
		w.Reset()
	}

	if s := strings.TrimSpace(d.c.Summary); s != "" {
		heading(d.titles["summary"])
		w.WriteString(para(pp{before: 40}, run(s, ink)))
	}
	cut("summary")
	if len(d.c.Education) > 0 {
		heading(d.titles["education"])
		for _, e := range d.c.Education {
			es.first = pp{before: 60, keepNext: true}
			w.WriteString(entryXML(width, entry("education"), entryParts{title: e.Degree, subtitle: e.School}, dates(e.Start, e.End), e.Location, es))
		}
	}
	cut("education")
	if len(d.c.Experience) > 0 {
		heading(d.titles["experience"])
		for _, g := range roleGroups(d.c.Experience, st) {
			es.first = pp{before: 60, keepNext: true}
			if len(g) > 1 {
				w.WriteString(docxRoleGroup(width, g, es, ink, st))
				continue
			}
			e := g[0]
			w.WriteString(entryXML(width, entry("experience"), entryParts{title: e.Title, subtitle: e.Company, kind: e.Kind}, dates(e.Start, e.End), e.Location, es))
			bullets(e.Description)
		}
	}
	cut("experience")
	if len(d.c.Projects) > 0 {
		heading(d.titles["projects"])
		for _, p := range d.c.Projects {
			w.WriteString(rowP(width, []string{run(p.Name, bold)}, p.Year, date, pp{before: 60, keepNext: true}))
			bullets(p.Description)
			if t := techText(p, d.lang); t != "" {
				w.WriteString(para(pp{}, run(t, rp{size: b, color: ci.ink, italic: true})))
			}
		}
	}
	cut("projects")
	if len(d.c.SkillGroups) > 0 || len(nonEmpty(d.c.Skills...)) > 0 {
		heading(d.titles["skills"])
		sep := skillSep(st, " · ")
		for _, g := range d.c.SkillGroups {
			if g.Label != "" {
				w.WriteString(para(pp{}, run(g.Label, bold)))
			}
			w.WriteString(para(pp{}, run(strings.Join(nonEmpty(g.Items...), sep), ink)))
		}
		if len(d.c.SkillGroups) == 0 {
			w.WriteString(para(pp{}, run(strings.Join(nonEmpty(d.c.Skills...), sep), ink)))
		}
	}
	cut("skills")
	if len(d.c.Certifications) > 0 {
		heading(d.titles["certifications"])
		for _, ce := range d.c.Certifications {
			w.WriteString(rowP(width, []string{run(ce.Name, bold), run(prefix(", ", ce.Issuer), sub)}, ce.Year, date, pp{}))
		}
	}
	cut("certifications")
	if len(d.c.Languages) > 0 {
		heading(d.titles["languages"])
		if inlineLanguages(st) {
			w.WriteString(docxLanguagesInline(d.c.Languages, rp{size: b, color: ci.ink, bold: true}, sub, pp{}))
		} else {
			for _, l := range d.c.Languages {
				name, level := splitLanguage(l)
				w.WriteString(rowP(width, []string{run(name, rp{size: b, color: ci.ink, bold: true})}, level, sub, pp{}))
			}
		}
	}
	cut("languages")
	for i, x := range d.c.Extra {
		if len(nonEmpty(x.Items...)) > 0 {
			heading(x.Title)
			bullets(x.Items)
		}
		cut("extra:" + strconv.Itoa(i))
	}
	return out
}
