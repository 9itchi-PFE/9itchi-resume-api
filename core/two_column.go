package core

import "strings"

// twoColSpec is how a two-column layout shares the page: the size and place of the side column, what fills it, where
// the name goes and which sections the side column holds. Every other section (extras included) goes in the main
// column; both keep the user's order and page break on their own.
type twoColSpec struct {
	side       float64         // width of the side column: share of the page, or of the text width when sideRight
	sideRight  bool            // side column right of the main one, inside the margins; otherwise against the left edge
	accent     rgb             // the template's accent: name, headings, rules, and the side fill or band
	header     string          // "main": name atop the main column, photo and contact atop the side one; "full": header across the page; "band": the same on an accent band
	fill       string          // side column fill: "accent", "tint" (light grey) or "" (none)
	rule       bool            // thin grey rule between the columns
	headRule   float64         // rule under the section headings, mm (0 = none)
	photoR     float64         // photo radius, mm
	photoRight bool            // the header photo sits on the right unless the user chose a side
	sideKeys   map[string]bool // sections of the side column
	legacy     bool            // modern's own drawing: spaced side titles, "Title — Company" rows unless an entry layout is chosen
}

// twoColSpecs registers the two-column layouts; a new one is a new entry.
var twoColSpecs = map[string]twoColSpec{
	"modern": {side: 0.34, accent: cvNavy, header: "main", fill: "accent", photoR: 11, legacy: true,
		sideKeys: map[string]bool{"skills": true, "languages": true}},
	"sidebar": {side: 0.33, accent: cvSlate, header: "main", fill: "accent", photoR: 16, headRule: 0.3,
		sideKeys: map[string]bool{"skills": true, "languages": true, "certifications": true}},
	"split": {side: 0.36, sideRight: true, accent: cvTeal, header: "full", rule: true, photoR: 13, headRule: 0.3,
		sideKeys: map[string]bool{"education": true, "skills": true, "languages": true, "certifications": true}},
	"banner": {side: 0.32, accent: cvWine, header: "band", fill: "tint", photoR: 13, photoRight: true, headRule: 0.3,
		sideKeys: map[string]bool{"education": true, "skills": true, "languages": true, "certifications": true}},
}

// accentFill is the color of the side fill or band: a chosen accent replaces the template's (modern keeps its navy
// unless the accent is picked for the rules).
func (sp twoColSpec) accentFill(lk look) rgb {
	if sp.legacy {
		return lk.colKeep("rules", sp.accent)
	}
	if lk.accent != nil {
		return *lk.accent
	}
	return sp.accent
}

// colInk are the text colors of one column.
type colInk struct {
	ink, title, sub, loc, date, head, rule rgb // body, entry titles, subtitles, locations, dates, headings, heading rules
}

func mainInk(lk look, sp twoColSpec) colInk {
	ink := lk.ink(cvInk)
	sub := cvMuted
	if !sp.legacy {
		sub = lk.col("subtitle", cvMuted)
	}
	return colInk{ink: ink, title: lk.col("entry_title", ink), sub: sub, loc: lk.col("location", cvMuted),
		date: lk.col("dates", cvMuted), head: lk.col("headings", sp.accent), rule: lk.col("rules", sp.accent)}
}

// lightInk is white text on a colored fill, secondary text at about 80%.
func lightInk(fill rgb) colInk {
	w := rgb{255, 255, 255}
	m := rgb{(4*255 + fill.r) / 5, (4*255 + fill.g) / 5, (4*255 + fill.b) / 5}
	return colInk{ink: w, title: w, sub: m, loc: m, date: m, head: w, rule: m}
}

const bandPad = 8.0 // mm above and below the banner's header text

func (d *cv) twoColumns(opts RenderOptions, layout string, sp twoColSpec) {
	pdf := d.pdf
	if sp.photoRight && d.lk.style.PhotoPos == "" {
		d.lk.photoRight = true
	}
	lk := d.lk
	d.body = lk.base
	top := lk.my
	const pad = 7.0 // inside a side column that runs to the page edge
	var sx, sw, mx, mw, edge, ruleX float64
	if sp.sideRight {
		const gutter = 8.0
		mx, mw = lk.mx, (210-2*lk.mx)*(1-sp.side)-gutter/2
		sx, ruleX = mx+mw+gutter, mx+mw+gutter/2
		sw = 210 - lk.mx - sx
	} else {
		edge = 210 * sp.side
		sx, sw = pad, edge-2*pad
		mx, mw = edge+lk.mx, 210-edge-2*lk.mx
	}
	accent := sp.accentFill(lk)
	var fill *rgb
	switch sp.fill {
	case "accent":
		fill = &accent
	case "tint":
		fill = &cvTint
	}
	bandH := 0.0
	if sp.header == "band" {
		_, _, th, block, _ := d.headerBlock(opts, sp)
		bandH = maxf(th, block) + 2*bandPad
	}
	pdf.SetHeaderFunc(func() {
		if lk.bg != nil {
			pdf.SetFillColor(lk.bg.r, lk.bg.g, lk.bg.b)
			pdf.Rect(0, 0, 210, 297, "F")
		}
		y0 := 0.0
		if bandH > 0 && pdf.PageNo() == 1 {
			pdf.SetFillColor(accent.r, accent.g, accent.b)
			pdf.Rect(0, 0, 210, bandH, "F")
			y0 = bandH
		}
		if fill != nil {
			pdf.SetFillColor(fill.r, fill.g, fill.b)
			pdf.Rect(0, y0, edge, 297-y0, "F")
		}
	})
	// A column that reaches a page another column already opened goes on there instead of adding one.
	pdf.SetAcceptPageBreakFunc(func() bool {
		if pdf.PageNo() >= pdf.PageCount() {
			return true
		}
		x := pdf.GetX()
		d.nextPage()
		pdf.SetX(x)
		return false
	})
	setCol := func(x, w float64) {
		d.x, d.w = x, w
		pdf.SetMargins(x, top, 210-x-w)
	}
	setCol(sx, sw)
	pdf.SetAutoPageBreak(true, top)
	pdf.AddPage()
	colTop := top
	if sp.header != "main" {
		colTop = d.wideHeader(opts, sp, accent, bandH)
		setCol(sx, sw)
	}
	order := d.c.SectionOrder()

	// Side column.
	side := mainInk(d.lk, sp)
	if sp.fill == "accent" {
		side = lightInk(accent)
		d.iconBg = &accent
	}
	y := colTop
	if sp.header == "main" && d.photo(opts, edge/2, y+sp.photoR, sp.photoR, nil) {
		y += 2*sp.photoR + 4
	}
	pdf.SetY(y)
	if sp.legacy {
		d.modernSide(order)
	} else {
		if sp.header == "main" {
			d.contactColumn(side)
		}
		d.column(order, true, layout, sp, side)
	}
	d.iconBg = nil

	// Main column, from the first page again.
	d.toPage(1)
	setCol(mx, mw)
	pdf.SetXY(d.x, colTop)
	if sp.header == "main" {
		d.font(d.nameFamily(), "B", lk.name, lk.col("name", sp.accent))
		d.para(d.c.Contact.Name, lk.name)
		if d.c.Contact.Headline != "" {
			pdf.Ln(0.5)
			d.font(sans, "", lk.title, lk.col("title", cvMuted))
			d.para(d.c.Contact.Headline, lk.title)
		}
	}
	d.column(order, false, layout, sp, mainInk(d.lk, sp))

	if sp.rule {
		for p := 1; p <= pdf.PageCount(); p++ {
			d.toPage(p)
			y0 := top
			if p == 1 {
				y0 = colTop
			}
			pdf.SetDrawColor(cvDivider.r, cvDivider.g, cvDivider.b)
			pdf.SetLineWidth(0.3)
			pdf.Line(ruleX, y0, ruleX, 297-top)
		}
	}
	d.toPage(pdf.PageCount())
}

// headerBlock measures the header across the page: where its text goes, the text height and the height of the
// block it sits in (the photo's when taller).
func (d *cv) headerBlock(opts RenderOptions, sp twoColSpec) (textX, tw, th, block float64, inline bool) {
	lk := d.lk
	d.x, d.w = lk.mx, 210-2*lk.mx
	textX, tw = d.x, d.w
	if len(opts.Photo) > 0 {
		textX, tw = textAfterPhoto(d, sp.photoR)
		block = 2 * sp.photoR
	}
	inline = lk.style.HeaderLayout == "inline" && !lk.center && d.c.Contact.Headline != ""
	lines := func(s string, w float64) float64 {
		return maxf(1, float64(len(d.pdf.SplitLines([]byte(cleanText(s)), w))))
	}
	d.font(d.nameFamily(), "B", lk.name, cvInk)
	if inline {
		th = d.inlineLines(tw) * lk.name * ptToMM * 1.15
	} else {
		th = lines(d.c.Contact.Name, tw) * lk.name * ptToMM * 1.15
		if d.c.Contact.Headline != "" {
			d.font(sans, "", lk.title, cvInk)
			th += 0.8 + lines(d.c.Contact.Headline, tw)*d.lh(lk.title)
		}
	}
	if items := d.contactItems(); len(items) > 0 {
		if lk.sep == "icon" {
			_, rows := d.contactIconRows(items, tw, lk.base-1)
			th += 1.2 + float64(len(rows))*d.lh(lk.base-1)*1.15
		} else {
			parts := make([]string, len(items))
			for i, it := range items {
				parts[i] = it.text
			}
			d.font(sans, "", lk.base-1, cvInk)
			th += 1.2 + lines(join(lk.separator(), parts...), tw)*d.lh(lk.base-1)
		}
	}
	return textX, tw, th, maxf(block, th), inline
}

// inlineLines counts the lines pdf.Write gives the name, two spaces and the title written on one row within w: the
// name in the name font (set by the caller), the title in the title font.
func (d *cv) inlineLines(w float64) float64 {
	pdf := d.pdf
	n, x := 1.0, 0.0
	run := func(text string, lead float64) {
		for i, word := range strings.Fields(cleanText(text)) {
			ww := pdf.GetStringWidth(word)
			if i > 0 {
				lead = pdf.GetStringWidth(" ")
			}
			if x > 0 && x+lead+ww > w {
				n, x = n+1, ww
				continue
			}
			if x > 0 {
				x += lead
			}
			x += ww
		}
	}
	run(d.c.Contact.Name, 0)
	gap := pdf.GetStringWidth("  ")
	d.font(sans, "", d.lk.title, cvInk)
	run(d.c.Contact.Headline, gap)
	return n
}

// wideHeader draws the name, professional title and contact details across the page, on the accent band when bandH
// is set, and returns where the columns start.
func (d *cv) wideHeader(opts RenderOptions, sp twoColSpec, accent rgb, bandH float64) float64 {
	pdf, lk := d.pdf, d.lk
	textX, tw, th, block, inline := d.headerBlock(opts, sp)
	nameCol, titleCol := lk.col("name", sp.accent), lk.col("title", cvMuted)
	textCol, linkCol := lk.ink(cvGrey), lk.col("links", sp.accent)
	top := lk.my
	if bandH > 0 {
		li := lightInk(accent)
		nameCol, titleCol, textCol, linkCol = li.ink, li.ink, li.sub, li.ink
		d.iconBg = &accent
		top, block = bandPad, bandH-2*bandPad
	}
	r := sp.photoR
	if len(opts.Photo) > 0 && !d.photo(opts, photoCenter(d, r), top+block/2, r, nil) {
		textX, tw = d.x, d.w
	}
	y := top
	switch free := block - th; {
	case free <= 0:
	case lk.headerValign == "bottom":
		y += free
	case lk.headerValign == "middle" || lk.headerValign == "" && bandH > 0:
		y += free / 2
	}
	align := "L"
	if lk.center {
		align = "C"
	}
	nameH := lk.name * ptToMM * 1.15
	pdf.SetXY(textX, y)
	d.font(d.nameFamily(), "B", lk.name, nameCol)
	if inline {
		pdf.SetMargins(textX, lk.my, 210-textX-tw)
		pdf.SetX(textX)
		pdf.Write(nameH, cleanText(d.c.Contact.Name)+"  ")
		d.font(sans, "", lk.title, titleCol)
		pdf.Write(nameH, cleanText(d.c.Contact.Headline))
		pdf.Ln(nameH)
		pdf.SetMargins(d.x, lk.my, lk.mx)
	} else {
		pdf.MultiCell(tw, nameH, cleanText(d.c.Contact.Name), "", align, false)
		if d.c.Contact.Headline != "" {
			pdf.Ln(0.8)
			d.font(sans, "", lk.title, titleCol)
			pdf.SetX(textX)
			pdf.MultiCell(tw, d.lh(lk.title), cleanText(d.c.Contact.Headline), "", align, false)
		}
	}
	pdf.Ln(1.2)
	d.contactLine(textX, tw, lk.base-1, textCol, linkCol, align)
	d.iconBg = nil
	if bandH > 0 {
		return maxf(bandH, pdf.GetY()) + 2
	}
	return maxf(pdf.GetY(), top+block) + 3
}

// contactColumn writes the contact details one per line, each after its icon unless the user chose a separator.
func (d *cv) contactColumn(ci colInk) {
	pdf := d.pdf
	size := d.body - 0.5
	icons := d.lk.style.Separator == "" || d.lk.style.Separator == "icon"
	is := size * ptToMM * 0.95
	for _, it := range d.contactItems() {
		d.need(d.lh(size))
		y, x := pdf.GetY(), d.x
		if icons {
			d.icon(it.kind, x, y+(d.lh(size)-is)/2, is, ci.ink)
			x += is + 2
		}
		d.font(sans, "", size, ci.ink)
		pdf.SetXY(x, y)
		pdf.MultiCell(d.x+d.w-x, d.lh(size), cleanText(it.text), "", "L", false)
		if it.url != "" {
			pdf.LinkString(d.x, y, d.w, d.lh(size), it.url)
		}
	}
	pdf.Ln(1)
}

// column draws the sections of one column (side or main) in the user's order.
func (d *cv) column(order []string, side bool, layout string, sp twoColSpec, ci colInk) {
	sections, head := d.columnSections(layout, sp, ci)
	d.locCol = ci.loc
	for _, key := range order {
		if sp.sideKeys[key] != side {
			continue
		}
		if f := sections[key]; f != nil {
			f()
		} else if x, ok := extraAt(d.c, key); ok && len(nonEmpty(x.Items...)) > 0 {
			head(x.Title)
			d.bullets(x.Items, d.body, ci.ink)
		}
	}
}

// columnSections are the section drawers of a two-column layout in the colors ci, and its heading.
func (d *cv) columnSections(layout string, sp twoColSpec, ci colInk) (map[string]func(), func(string)) {
	pdf, lk := d.pdf, d.lk
	b, eb, ink := d.body, lk.entry, ci.ink
	head := func(t string) {
		if sp.legacy {
			d.need(12)
		} else {
			// the gap, the heading and the first two rows stay together
			d.need(sectionGap(lk.style, 3) + d.lh(lk.head) + 2 + 2*d.lh(lk.entry))
		}
		pdf.Ln(sectionGap(lk.style, 3))
		family, style := d.nameFamily(), lk.headFont("B")
		if style != "B" {
			family = sans // the serif is embedded in bold only
		}
		d.font(family, style, lk.head, ci.head)
		pdf.SetX(d.x)
		pdf.CellFormat(d.w, d.lh(lk.head), cleanText(lk.heading(t)), "", 1, "L", false, 0, "")
		if rw := headingRuleWidth(lk.style, sp.headRule); rw > 0 {
			d.rule(ci.rule, rw)
			pdf.Ln(0.6)
		}
		pdf.Ln(0.4)
	}
	bold := func(t string) seg { return seg{text: t, family: sans, style: "B", col: ci.title} }
	muted := func(t string) seg { return seg{text: t, family: sans, style: pick(lk.subStyle, ""), col: ci.sub} }
	locSeg := func(t string) seg { return seg{text: t, family: sans, style: pick(lk.subStyle, ""), col: ci.loc} }
	right := func() { d.font(sans, pick(lk.dateStyle, ""), eb, ci.date) }
	rightLoc := func() { d.font(sans, pick(lk.dateStyle, ""), eb, ci.loc) }
	between := func() { pdf.Ln(lk.gapOr(0.8)) }
	st := entryStyle{bold: bold, sub: muted, kind: muted, loc: locSeg, date: right, locR: rightLoc}
	// entry is the user's entry layout, else the template's; ok is false only where modern keeps its own rows.
	entry := func(section string) (entryLayout, bool) {
		ly, ok := entryLook(layout, section, lk.style)
		if !ok && !sp.legacy {
			return entryDefaults[layout][section], true
		}
		return ly, ok
	}
	return map[string]func(){
		"summary": func() {
			if s := strings.TrimSpace(d.c.Summary); s != "" {
				head(d.titles["summary"])
				d.font(sans, "", b, ink)
				d.para(s, b)
			}
		},
		"education": func() {
			if len(d.c.Education) == 0 {
				return
			}
			head(d.titles["education"])
			for _, e := range d.c.Education {
				if ly, ok := entry("education"); ok {
					d.entryRows(ly, entryParts{title: e.Degree, subtitle: e.School, url: e.URL}, dates(e.Start, e.End), e.Location, st, eb)
				} else {
					d.row(d.segs(linked(bold(join(" — ", e.School, e.Degree)), e.URL)), right, dates(e.Start, e.End), eb)
					if e.Location != "" {
						d.row(d.segs(locSeg(e.Location)), right, "", eb)
					}
				}
				between()
			}
		},
		"experience": func() {
			if len(d.c.Experience) == 0 {
				return
			}
			head(d.titles["experience"])
			for _, g := range roleGroups(d.c.Experience, lk.style) {
				if len(g) > 1 {
					d.groupedRoles(g, st, b, ink, between)
					between()
					continue
				}
				e := g[0]
				if ly, ok := entry("experience"); ok {
					d.entryRows(ly, entryParts{title: e.Title, subtitle: e.Company, kind: e.Kind, url: e.URL}, dates(e.Start, e.End), e.Location, st, eb)
				} else {
					d.row(d.segs(linked(bold(join(" — ", e.Title, e.Company)), e.URL)), right, dates(e.Start, e.End), eb)
					if e.Kind != "" || e.Location != "" {
						var parts []seg
						if e.Kind != "" {
							parts = append(parts, muted(e.Kind))
						}
						if e.Location != "" {
							parts = append(parts, locSeg(prefix(" · ", e.Kind)+e.Location))
						}
						d.row(d.segs(parts...), right, "", eb)
					}
				}
				d.bullets(e.Description, b, ink)
				between()
			}
		},
		"projects": func() {
			if len(d.c.Projects) == 0 {
				return
			}
			head(d.titles["projects"])
			for _, p := range d.c.Projects {
				d.row(d.segs(linked(bold(p.Name), p.URL)), right, p.Year, eb)
				d.bullets(p.Description, b, ink)
				d.tech(p, b, ink)
				between()
			}
		},
		"certifications": func() {
			if len(d.c.Certifications) == 0 {
				return
			}
			head(d.titles["certifications"])
			for _, ce := range d.c.Certifications {
				d.row(d.segs(linked(bold(ce.Name), ce.URL), muted(prefix(", ", ce.Issuer))), right, ce.Year, eb)
			}
		},
		"skills": func() {
			if len(d.c.SkillGroups) == 0 && len(nonEmpty(d.c.Skills...)) == 0 {
				return
			}
			head(d.titles["skills"])
			sep := skillSep(lk.style, " · ")
			for _, g := range d.c.SkillGroups {
				if g.Label != "" {
					d.font(sansSemi, "", b, ci.title)
					d.para(g.Label, b)
				}
				d.font(sans, "", b, ink)
				d.para(strings.Join(nonEmpty(g.Items...), sep), b)
			}
			if len(d.c.SkillGroups) == 0 {
				d.font(sans, "", b, ink)
				d.para(strings.Join(nonEmpty(d.c.Skills...), sep), b)
			}
		},
		"languages": func() {
			if len(d.c.Languages) == 0 {
				return
			}
			head(d.titles["languages"])
			if inlineLanguages(lk.style) {
				d.languagesInline(b, ink, ci.sub)
				return
			}
			level := func() { d.font(sans, "", eb, ci.sub) }
			for _, l := range d.c.Languages {
				name, lv := splitLanguage(l)
				d.row(d.segs(seg{text: name, family: sans, style: "B", col: ink}), level, lv, eb)
			}
		},
	}, head
}

// modernSide is the side column of the modern layout: small spaced titles, contact, skills and languages.
func (d *cv) modernSide(order []string) {
	pdf, lk := d.pdf, d.lk
	b := d.body
	sideHead := func(t string) {
		pdf.Ln(sectionGap(lk.style, 2.5))
		d.font(sans, "B", 9, cvSideTxt)
		pdf.SetX(d.x)
		pdf.CellFormat(d.w, d.lh(9), cleanText(spaced(strings.ToUpper(t))), "", 1, "L", false, 0, "")
		pdf.Ln(0.6)
	}
	sideLines := func(lines ...string) {
		d.font(sans, "", b-0.5, cvSideTxt)
		for _, l := range nonEmpty(lines...) {
			d.para(l, b-0.5)
		}
	}
	var contact []string
	contact = append(contact, d.c.Contact.Email, d.c.Contact.Phone, d.c.Contact.Location)
	for _, l := range d.c.Contact.Links {
		contact = append(contact, shortLink(l))
	}
	if len(nonEmpty(contact...)) > 0 {
		sideHead(d.titles["contact"])
		sideLines(contact...)
	}
	sideSections := map[string]func(){
		"skills": func() {
			if len(d.c.SkillGroups) == 0 && len(nonEmpty(d.c.Skills...)) == 0 {
				return
			}
			sideHead(d.titles["skills"])
			sep := skillSep(lk.style, " · ")
			if len(d.c.SkillGroups) > 0 {
				for _, g := range d.c.SkillGroups {
					if g.Label != "" {
						d.font(sansSemi, "", b-0.5, cvSideTxt)
						d.para(g.Label, b-0.5)
					}
					sideLines(strings.Join(nonEmpty(g.Items...), sep))
				}
			} else if lk.style.SkillsStyle != "" {
				sideLines(strings.Join(nonEmpty(d.c.Skills...), sep))
			} else {
				sk := nonEmpty(d.c.Skills...)
				for i := 0; i < len(sk); i += 2 {
					sideLines(strings.Join(sk[i:min(i+2, len(sk))], " · "))
				}
			}
		},
		"languages": func() {
			if len(d.c.Languages) == 0 {
				return
			}
			sideHead(d.titles["languages"])
			if inlineLanguages(lk.style) {
				d.languagesInline(b-0.5, cvSideTxt, cvSideTxt)
				return
			}
			for _, l := range d.c.Languages {
				name, level := splitLanguage(l)
				sideLines(join(" · ", name, level))
			}
		},
	}
	for _, key := range order {
		if f := sideSections[key]; f != nil {
			f()
		}
	}
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
