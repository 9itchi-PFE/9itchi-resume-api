package core

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // photo size
	_ "image/png"  // photo size
	"strconv"
	"strings"
	"unicode/utf8"

	"9itchi-resume-api/models/api"
)

// The Word export follows the same three layouts as the PDF. Word users rarely have Source Sans 3 or
// Source Serif 4, so the file uses the Office fonts closest to them (Calibri, Cambria); colours, titles,
// the photo circle and the dark side column are the same.

type docxExporter struct{}

func NewDOCXExporter() Exporter { return docxExporter{} }

func (docxExporter) Format() string { return "docx" }

const (
	twPerMM  = 56.7
	emuPerMM = 36000
	pageW    = 11906 // A4 in twips
	pageH    = 16838
)

// xmlText escapes s for a w:t element and drops characters XML 1.0 forbids.
func xmlText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x20, r == 0xFFFE, r == 0xFFFF, r == utf8.RuneError, r >= 0xD800 && r <= 0xDFFF:
			// dropped
		case r == '&':
			b.WriteString("&amp;")
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// rp is a run's look.
type rp struct {
	bold, italic, caps, smallCaps bool
	size                          float64 // pt, 0 = default
	color                         string  // hex without '#'
	font                          string
	spacing                       int // twentieths of a point
}

func (p rp) xml() string {
	var b strings.Builder
	if p.font != "" {
		fmt.Fprintf(&b, `<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>`, p.font, p.font, p.font)
	}
	if p.bold {
		b.WriteString(`<w:b/><w:bCs/>`)
	}
	if p.italic {
		b.WriteString(`<w:i/><w:iCs/>`)
	}
	if p.caps {
		b.WriteString(`<w:caps/>`)
	}
	if p.smallCaps {
		b.WriteString(`<w:smallCaps/>`)
	}
	if p.color != "" {
		fmt.Fprintf(&b, `<w:color w:val="%s"/>`, p.color)
	}
	if p.spacing != 0 {
		fmt.Fprintf(&b, `<w:spacing w:val="%d"/>`, p.spacing)
	}
	if p.size > 0 {
		hp := int(p.size*2 + 0.5)
		fmt.Fprintf(&b, `<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, hp, hp)
	}
	return b.String()
}

func run(text string, p rp) string {
	if text == "" {
		return ""
	}
	return `<w:r><w:rPr>` + p.xml() + `</w:rPr><w:t xml:space="preserve">` + xmlText(text) + `</w:t></w:r>`
}

const tabRun = `<w:r><w:tab/></w:r>`

// pp is a paragraph's look.
type pp struct {
	align            string // left, center, right
	before, after    int    // twips
	rightTab         int    // position of a right tab, 0 = none
	leftTab          int
	indLeft, hanging int
	bullet           bool
	borderBottom     string // colour
	borderTop        string
	borderSz         int // eighths of a point
	keepNext         bool
}

func (p pp) xml() string {
	var b strings.Builder
	if p.keepNext {
		b.WriteString(`<w:keepNext/>`)
	}
	if p.bullet {
		b.WriteString(`<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>`)
	}
	if p.borderBottom != "" || p.borderTop != "" {
		sz := p.borderSz
		if sz == 0 {
			sz = 6
		}
		b.WriteString(`<w:pBdr>`)
		if p.borderTop != "" {
			fmt.Fprintf(&b, `<w:top w:val="single" w:sz="%d" w:space="4" w:color="%s"/>`, sz, p.borderTop)
		}
		if p.borderBottom != "" {
			fmt.Fprintf(&b, `<w:bottom w:val="single" w:sz="%d" w:space="1" w:color="%s"/>`, sz, p.borderBottom)
		}
		b.WriteString(`</w:pBdr>`)
	}
	if p.rightTab > 0 || p.leftTab > 0 {
		b.WriteString(`<w:tabs>`)
		if p.leftTab > 0 {
			fmt.Fprintf(&b, `<w:tab w:val="left" w:pos="%d"/>`, p.leftTab)
		}
		if p.rightTab > 0 {
			fmt.Fprintf(&b, `<w:tab w:val="right" w:pos="%d"/>`, p.rightTab)
		}
		b.WriteString(`</w:tabs>`)
	}
	fmt.Fprintf(&b, `<w:spacing w:before="%d" w:after="%d"/>`, p.before, p.after)
	if p.indLeft > 0 || p.hanging > 0 {
		fmt.Fprintf(&b, `<w:ind w:left="%d" w:hanging="%d"/>`, p.indLeft, p.hanging)
	}
	if p.align != "" {
		fmt.Fprintf(&b, `<w:jc w:val="%s"/>`, p.align)
	}
	return b.String()
}

func para(p pp, runs ...string) string {
	return `<w:p><w:pPr>` + p.xml() + `</w:pPr>` + strings.Join(runs, "") + `</w:p>`
}

// docx collects the body of one resume.
// docBodyFont maps the chosen typeface to the Office font Word users have: Source Sans 3 and Carlito are drawn as Calibri.
func docBodyFont(s *api.CVStyle) string {
	switch bodyFontKey(s) {
	case "lato":
		return "Lato"
	case "tinos":
		return "Times New Roman"
	}
	return "Calibri"
}

type docx struct {
	c      api.ResumeContent
	titles map[string]string
	lang   string
	body   strings.Builder
	photo  *docxPhoto
}

type docxPhoto struct {
	data   []byte
	ext    string
	crop   [4]int // l, t, r, b in thousandths of a percent
	usedID int
}

func newDocxPhoto(data []byte, format string) *docxPhoto {
	if len(data) == 0 {
		return nil
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil
	}
	ext := "jpeg"
	if kind == "png" || format == "png" {
		ext = "png"
	}
	ph := &docxPhoto{data: data, ext: ext}
	w, h := float64(cfg.Width), float64(cfg.Height)
	if w > h {
		side := int((1 - h/w) / 2 * 100000)
		ph.crop = [4]int{side, 0, side, 0}
	} else if h > w {
		side := int((1 - w/h) / 2 * 100000)
		ph.crop = [4]int{0, side, 0, side}
	}
	return ph
}

// drawing is the inline round photo, diameter in mm.
func (d *docx) drawing(diameter float64, border string) string {
	if d.photo == nil {
		return ""
	}
	d.photo.usedID++
	e := strconv.Itoa(int(diameter * emuPerMM))
	ln := `<a:ln><a:noFill/></a:ln>`
	if border != "" {
		ln = `<a:ln w="19050"><a:solidFill><a:srgbClr val="` + border + `"/></a:solidFill></a:ln>`
	}
	cr := d.photo.crop
	return `<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="` + e + `" cy="` + e + `"/>` +
		`<wp:docPr id="` + strconv.Itoa(d.photo.usedID) + `" name="Photo" descr="Photo"/>` +
		`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic>` +
		`<pic:nvPicPr><pic:cNvPr id="0" name="photo.` + d.photo.ext + `"/><pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="rIdPhoto"/>` +
		fmt.Sprintf(`<a:srcRect l="%d" t="%d" r="%d" b="%d"/>`, cr[0], cr[1], cr[2], cr[3]) +
		`<a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + e + `" cy="` + e + `"/></a:xfrm><a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>` + ln + `</pic:spPr>` +
		`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`
}

func (d *docx) add(xml string) { d.body.WriteString(xml) }

func (docxExporter) Render(ctx context.Context, c api.ResumeContent, layout string, opts RenderOptions) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	titles, ok := headings[opts.Language]
	if !ok {
		titles = headings["fr"]
	}
	d := &docx{c: visibleEntries(c), titles: titles, lang: opts.Language}
	if layout != "compact" {
		d.photo = newDocxPhoto(opts.Photo, opts.PhotoFormat)
	}
	var sect string
	switch layout {
	case "modern":
		sect = d.modern(opts.Branded)
	case "compact":
		sect = d.compact()
	default:
		sect = d.classic()
	}
	if opts.Branded {
		sect = strings.Replace(sect, `<w:sectPr>`, `<w:sectPr><w:footerReference w:type="default" r:id="rIdFooter"/>`, 1)
	}
	return d.pack(sect, opts.Branded)
}

func sectPr(top, right, bottom, left int) string {
	return fmt.Sprintf(`<w:sectPr><w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="0" w:footer="280" w:gutter="0"/></w:sectPr>`,
		pageW, pageH, top, right, bottom, left)
}

func mm(v float64) int { return int(v*twPerMM + 0.5) }

const (
	hexBlue  = "2F5597"
	hexInk   = "1A1A1A"
	hexGrey  = "333333"
	hexNavy  = "1F2A44"
	hexSide  = "E8ECF4"
	hexMuted = "596070"
	hexBlack = "111111"
	hexDim   = "444444"
)

// rowP is "left<TAB>right" with a right tab at the column edge.
func rowP(width int, left []string, right string, rightP rp, extra pp) string {
	extra.rightTab = width
	runs := append([]string{}, left...)
	if strings.TrimSpace(right) != "" {
		runs = append(runs, tabRun, run(right, rightP))
	}
	return para(extra, runs...)
}

// ---- Classique ----

func (d *docx) classic() string {
	width := pageW - 2*mm(14)
	b := 10.5
	ink := rp{size: b, color: hexInk}
	bold := rp{size: b, color: hexInk, bold: true}
	blue := rp{size: b, color: hexBlue}

	// Header.
	var head strings.Builder
	head.WriteString(para(pp{after: 20}, run(d.c.Contact.Name, rp{size: 23, bold: true, color: hexBlack})))
	if d.c.Contact.Headline != "" {
		head.WriteString(para(pp{after: 20}, run(d.c.Contact.Headline, rp{size: 12, color: hexBlue, smallCaps: true, spacing: 4})))
	}
	head.WriteString(para(pp{}, d.contactRuns(9.5, hexGrey, hexBlue)...))
	if pic := d.drawing(26, hexBlue); pic != "" {
		pw := mm(32)
		d.add(`<w:tbl><w:tblPr><w:tblW w:w="` + strconv.Itoa(width) + `" w:type="dxa"/><w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:bottom w:val="nil"/><w:right w:val="nil"/><w:insideH w:val="nil"/><w:insideV w:val="nil"/></w:tblBorders><w:tblLayout w:type="fixed"/><w:tblCellMar><w:left w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr>` +
			`<w:tblGrid><w:gridCol w:w="` + strconv.Itoa(pw) + `"/><w:gridCol w:w="` + strconv.Itoa(width-pw) + `"/></w:tblGrid><w:tr>` +
			`<w:tc><w:tcPr><w:tcW w:w="` + strconv.Itoa(pw) + `" w:type="dxa"/><w:vAlign w:val="center"/></w:tcPr>` + para(pp{}, pic) + `</w:tc>` +
			`<w:tc><w:tcPr><w:tcW w:w="` + strconv.Itoa(width-pw) + `" w:type="dxa"/><w:vAlign w:val="center"/></w:tcPr>` + head.String() + `</w:tc></w:tr></w:tbl>`)
	} else {
		d.add(head.String())
	}
	o := newOrdered(&d.body)
	o.begin()
	heading := func(t string) {
		d.add(para(pp{before: 160, after: 60, borderBottom: hexBlue, keepNext: true}, run(t, rp{size: 13, color: hexBlue, caps: true})))
	}
	if s := strings.TrimSpace(d.c.Summary); s != "" {
		heading(d.titles["summary"])
		d.add(para(pp{before: 60}, run(s, ink)))
	}
	o.cut("summary")
	if len(d.c.Education) > 0 {
		heading(d.titles["education"])
		for i, e := range d.c.Education {
			before := 0
			if i > 0 {
				before = 60
			}
			d.add(rowP(width, []string{run(e.Degree, bold)}, dates(e.Start, e.End), ink, pp{before: before, keepNext: true}))
			if e.School != "" || e.Location != "" {
				d.add(rowP(width, []string{run(e.School, ink)}, e.Location, ink, pp{}))
			}
		}
	}
	o.cut("education")
	if len(d.c.Experience) > 0 {
		heading(d.titles["experience"])
		for i, e := range d.c.Experience {
			before := 0
			if i > 0 {
				before = 60
			}
			d.add(rowP(width, []string{run(e.Title, bold)}, dates(e.Start, e.End), bold, pp{before: before, keepNext: true}))
			if e.Company != "" || e.Kind != "" || e.Location != "" {
				left := []string{run(e.Company, blue)}
				if e.Kind != "" {
					left = append(left, run(" ("+e.Kind+")", ink))
				}
				d.add(rowP(width, left, e.Location, ink, pp{}))
			}
			d.bullets(e.Description, ink)
		}
	}
	o.cut("experience")
	if len(d.c.Projects) > 0 {
		heading(d.titles["projects"])
		for _, p := range d.c.Projects {
			d.add(rowP(width, []string{run(p.Name, bold)}, p.Year, ink, pp{keepNext: true}))
			d.bullets(p.Description, ink)
			d.techLine(p, ink)
		}
	}
	o.cut("projects")
	if len(d.c.SkillGroups) > 0 || len(nonEmpty(d.c.Skills...)) > 0 {
		heading(d.titles["skills"])
		if len(d.c.SkillGroups) == 0 {
			d.add(para(pp{}, run(strings.Join(nonEmpty(d.c.Skills...), ", "), ink)))
		}
		col := int(float64(width) * 0.38)
		for _, g := range d.c.SkillGroups {
			d.add(para(pp{leftTab: col, indLeft: col, hanging: col}, run(g.Label, bold), tabRun, run(strings.Join(nonEmpty(g.Items...), ", "), ink)))
		}
	}
	o.cut("skills")
	if len(d.c.Certifications) > 0 {
		heading(d.titles["certifications"])
		for _, ce := range d.c.Certifications {
			d.add(rowP(width, []string{run(ce.Name, bold), run(prefix(", ", ce.Issuer), ink)}, ce.Year, ink, pp{}))
		}
	}
	o.cut("certifications")
	if len(d.c.Languages) > 0 {
		heading(d.titles["languages"])
		for _, l := range d.c.Languages {
			name, level := splitLanguage(l)
			d.add(rowP(width, []string{run(name, bold)}, level, ink, pp{}))
		}
	}
	o.cut("languages")
	d.extra(heading, ink, o)
	o.replace(d.c.SectionOrder())
	return sectPr(mm(12), mm(14), mm(12), mm(14))
}

func (d *docx) contactRuns(size float64, col, linkCol string) []string {
	var out []string
	add := func(t, c string) {
		if len(out) > 0 {
			out = append(out, run(" · ", rp{size: size, color: col}))
		}
		out = append(out, run(t, rp{size: size, color: c}))
	}
	for _, p := range nonEmpty(d.c.Contact.Email, d.c.Contact.Phone, d.c.Contact.Location) {
		add(p, col)
	}
	for _, l := range nonEmpty(d.c.Contact.Links...) {
		add(shortLink(l), linkCol)
	}
	return out
}

func (d *docx) bullets(lines []string, p rp) {
	for _, l := range nonEmpty(lines...) {
		d.add(para(pp{bullet: true}, run(l, p)))
	}
}

// techLine writes the project's technologies under its description, unless the user hid them.
func (d *docx) techLine(p api.Project, rpr rp) {
	if t := techText(p, d.lang); t != "" {
		rpr.italic = true
		d.add(para(pp{bullet: false}, run(t, rpr)))
	}
}

// listXML gives the bullet glyph and indent the user chose.
func listXML(s *api.CVStyle) (glyph string, left, hang int) {
	glyph, left, hang = "•", 284, 227
	if s != nil && s.ListStyle == "hyphen" {
		glyph = "-"
	}
	if s != nil && s.IndentBody != nil && !*s.IndentBody {
		left, hang = 170, 170
	}
	return
}

func (d *docx) extra(heading func(string), p rp, o *ordered) {
	for i, x := range d.c.Extra {
		if len(nonEmpty(x.Items...)) > 0 {
			heading(x.Title)
			d.bullets(x.Items, p)
		}
		o.cut("extra:" + strconv.Itoa(i))
	}
}

// ---- Moderne ----

func (d *docx) modern(branded bool) string {
	sideW := pageW * 34 / 100
	mainW := pageW - sideW
	padS, padM, top := mm(7), mm(9), mm(12)
	textW := mainW - 2*padM
	b := 10.0
	side := rp{size: b - 0.5, color: hexSide}

	var s strings.Builder
	if pic := d.drawing(22, ""); pic != "" {
		s.WriteString(para(pp{align: "center", after: 120}, pic))
	}
	sideHead := func(t string) {
		s.WriteString(para(pp{before: 160, after: 40, keepNext: true}, run(t, rp{size: 9, bold: true, caps: true, color: hexSide, spacing: 30})))
	}
	sideLine := func(t string, p rp) {
		if strings.TrimSpace(t) != "" {
			s.WriteString(para(pp{}, run(t, p)))
		}
	}
	contact := nonEmpty(d.c.Contact.Email, d.c.Contact.Phone, d.c.Contact.Location)
	for _, l := range nonEmpty(d.c.Contact.Links...) {
		contact = append(contact, shortLink(l))
	}
	if len(contact) > 0 {
		sideHead(d.titles["contact"])
		for _, l := range contact {
			sideLine(l, side)
		}
	}
	os := newOrdered(&s)
	os.begin()
	if len(d.c.SkillGroups) > 0 || len(nonEmpty(d.c.Skills...)) > 0 {
		sideHead(d.titles["skills"])
		if len(d.c.SkillGroups) == 0 {
			sk := nonEmpty(d.c.Skills...)
			for i := 0; i < len(sk); i += 2 {
				sideLine(strings.Join(sk[i:min(i+2, len(sk))], " · "), side)
			}
		}
		for _, g := range d.c.SkillGroups {
			sideLine(g.Label, rp{size: b - 0.5, color: hexSide, bold: true})
			sideLine(strings.Join(nonEmpty(g.Items...), " · "), side)
		}
	}
	os.cut("skills")
	if len(d.c.Languages) > 0 {
		sideHead(d.titles["languages"])
		for _, l := range d.c.Languages {
			name, level := splitLanguage(l)
			sideLine(join(" · ", name, level), side)
		}
	}
	os.cut("languages")
	order := d.c.SectionOrder()
	os.replace(order)
	if s.Len() == 0 {
		s.WriteString(para(pp{}))
	}

	var m strings.Builder
	ink := rp{size: b, color: hexInk}
	bold := rp{size: b, color: hexInk, bold: true}
	muted := rp{size: b, color: hexMuted}
	m.WriteString(para(pp{after: 20}, run(d.c.Contact.Name, rp{size: 22, bold: true, color: hexNavy, font: "Cambria"})))
	if d.c.Contact.Headline != "" {
		m.WriteString(para(pp{}, run(d.c.Contact.Headline, rp{size: b + 0.5, color: hexMuted})))
	}
	om := newOrdered(&m)
	om.begin()
	heading := func(t string) {
		m.WriteString(para(pp{before: 180, after: 40, keepNext: true}, run(t, rp{size: 13, bold: true, color: hexNavy, font: "Cambria"})))
	}
	if sum := strings.TrimSpace(d.c.Summary); sum != "" {
		heading(d.titles["summary"])
		m.WriteString(para(pp{before: 60}, run(sum, ink)))
	}
	om.cut("summary")
	row := func(left []string, right string, p pp) { m.WriteString(rowP(textW, left, right, muted, p)) }
	bullets := func(lines []string) {
		for _, l := range nonEmpty(lines...) {
			m.WriteString(para(pp{bullet: true}, run(l, ink)))
		}
	}
	if len(d.c.Education) > 0 {
		heading(d.titles["education"])
		for _, e := range d.c.Education {
			row([]string{run(join(" — ", e.School, e.Degree), bold)}, dates(e.Start, e.End), pp{before: 40})
			if e.Location != "" {
				row([]string{run(e.Location, muted)}, "", pp{})
			}
		}
	}
	om.cut("education")
	if len(d.c.Experience) > 0 {
		heading(d.titles["experience"])
		for _, e := range d.c.Experience {
			row([]string{run(join(" — ", e.Title, e.Company), bold)}, dates(e.Start, e.End), pp{before: 40, keepNext: true})
			if l := join(" · ", e.Kind, e.Location); l != "" {
				row([]string{run(l, muted)}, "", pp{})
			}
			bullets(e.Description)
		}
	}
	om.cut("experience")
	if len(d.c.Projects) > 0 {
		heading(d.titles["projects"])
		for _, p := range d.c.Projects {
			row([]string{run(p.Name, bold)}, p.Year, pp{before: 40, keepNext: true})
			bullets(p.Description)
			d.techLine(p, ink)
		}
	}
	om.cut("projects")
	if len(d.c.Certifications) > 0 {
		heading(d.titles["certifications"])
		for _, ce := range d.c.Certifications {
			row([]string{run(ce.Name, bold), run(prefix(", ", ce.Issuer), muted)}, ce.Year, pp{})
		}
	}
	om.cut("certifications")
	for i, x := range d.c.Extra {
		if len(nonEmpty(x.Items...)) > 0 {
			heading(x.Title)
			bullets(x.Items)
		}
		om.cut("extra:" + strconv.Itoa(i))
	}
	om.replace(order)

	// One full-height row: the dark side cell and the main cell.
	rowH := pageH - 60
	if branded {
		rowH = pageH - mm(12)
	}
	d.add(`<w:tbl><w:tblPr><w:tblW w:w="` + strconv.Itoa(pageW) + `" w:type="dxa"/><w:tblBorders><w:top w:val="nil"/><w:left w:val="nil"/><w:bottom w:val="nil"/><w:right w:val="nil"/><w:insideH w:val="nil"/><w:insideV w:val="nil"/></w:tblBorders><w:tblLayout w:type="fixed"/><w:tblCellMar><w:left w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPr>` +
		`<w:tblGrid><w:gridCol w:w="` + strconv.Itoa(sideW) + `"/><w:gridCol w:w="` + strconv.Itoa(mainW) + `"/></w:tblGrid>` +
		`<w:tr><w:trPr><w:trHeight w:val="` + strconv.Itoa(rowH) + `" w:hRule="atLeast"/></w:trPr>` +
		`<w:tc><w:tcPr><w:tcW w:w="` + strconv.Itoa(sideW) + `" w:type="dxa"/><w:shd w:val="clear" w:color="auto" w:fill="` + hexNavy + `"/>` +
		fmt.Sprintf(`<w:tcMar><w:top w:w="%d" w:type="dxa"/><w:left w:w="%d" w:type="dxa"/><w:right w:w="%d" w:type="dxa"/></w:tcMar>`, top, padS, padS) +
		`</w:tcPr>` + s.String() + `</w:tc>` +
		`<w:tc><w:tcPr><w:tcW w:w="` + strconv.Itoa(mainW) + `" w:type="dxa"/>` +
		fmt.Sprintf(`<w:tcMar><w:top w:w="%d" w:type="dxa"/><w:left w:w="%d" w:type="dxa"/><w:right w:w="%d" w:type="dxa"/></w:tcMar>`, top, padM, padM) +
		`</w:tcPr>` + m.String() + `</w:tc></w:tr></w:tbl>`)
	// Word needs a paragraph after a table; keep it tiny so it does not add a page.
	d.add(`<w:p><w:pPr><w:spacing w:before="0" w:after="0" w:line="20" w:lineRule="exact"/></w:pPr></w:p>`)
	bottom := 0
	if branded {
		bottom = mm(10)
	}
	return sectPr(0, 0, bottom, 0)
}

// ---- Compact ----

func (d *docx) compact() string {
	width := pageW - 2*mm(15)
	b := 10.0
	ink := rp{size: b, color: hexBlack}
	bold := rp{size: b, color: hexBlack, bold: true}
	d.add(para(pp{align: "center"}, run(d.c.Contact.Name, rp{size: 17, bold: true, caps: true, color: hexBlack, spacing: 30})))
	if d.c.Contact.Headline != "" {
		d.add(para(pp{align: "center"}, run(d.c.Contact.Headline, rp{size: b, color: hexDim})))
	}
	contact := nonEmpty(d.c.Contact.Email, d.c.Contact.Phone, d.c.Contact.Location)
	for _, l := range nonEmpty(d.c.Contact.Links...) {
		contact = append(contact, shortLink(l))
	}
	if len(contact) > 0 {
		d.add(para(pp{align: "center"}, run(strings.Join(contact, " · "), rp{size: b - 0.5, color: hexDim})))
	}
	o := newOrdered(&d.body)
	o.begin()
	if s := strings.TrimSpace(d.c.Summary); s != "" {
		d.add(para(pp{before: 100}, run(s, ink)))
	}
	o.cut("summary")
	heading := func(t string) {
		d.add(para(pp{before: 150, after: 60, borderBottom: hexBlack, borderSz: 6, keepNext: true}, run(t, rp{size: 11, bold: true, caps: true, color: hexBlack})))
	}
	if len(d.c.Education) > 0 {
		heading(d.titles["education"])
		for _, e := range d.c.Education {
			d.add(rowP(width, []string{run(join(" — ", e.School, e.Degree), bold)}, dates(e.Start, e.End), ink, pp{}))
		}
	}
	o.cut("education")
	if len(d.c.Experience) > 0 {
		heading(d.titles["experience"])
		for _, e := range d.c.Experience {
			d.add(rowP(width, []string{run(join(" : ", e.Company, e.Title), bold), run(prefix(", ", e.Location), ink)}, dates(e.Start, e.End), ink, pp{keepNext: true}))
			d.bullets(e.Description, ink)
		}
	}
	o.cut("experience")
	if len(d.c.Projects) > 0 {
		heading(d.titles["projects"])
		for _, p := range d.c.Projects {
			d.add(rowP(width, []string{run(p.Name, bold)}, p.Year, ink, pp{keepNext: true}))
			d.bullets(p.Description, ink)
			d.techLine(p, ink)
		}
	}
	o.cut("projects")
	if len(d.c.SkillGroups) > 0 || len(nonEmpty(d.c.Skills...)) > 0 {
		heading(d.titles["skills"])
		if len(d.c.SkillGroups) == 0 {
			d.add(para(pp{}, run(strings.Join(nonEmpty(d.c.Skills...), ", "), ink)))
		}
		for _, g := range d.c.SkillGroups {
			d.add(para(pp{}, run(g.Label+suffixIf(g.Label, " : "), bold), run(strings.Join(nonEmpty(g.Items...), ", "), ink)))
		}
	}
	o.cut("skills")
	if len(d.c.Certifications) > 0 {
		heading(d.titles["certifications"])
		for _, ce := range d.c.Certifications {
			d.add(rowP(width, []string{run(ce.Name, bold), run(prefix(", ", ce.Issuer), ink)}, ce.Year, ink, pp{}))
		}
	}
	o.cut("certifications")
	if len(d.c.Languages) > 0 {
		heading(d.titles["languages"])
		var parts []string
		for _, l := range d.c.Languages {
			name, level := splitLanguage(l)
			if level != "" {
				name += " (" + level + ")"
			}
			parts = append(parts, name)
		}
		d.add(para(pp{}, run(strings.Join(nonEmpty(parts...), " · "), ink)))
	}
	o.cut("languages")
	d.extra(heading, ink, o)
	o.replace(d.c.SectionOrder())
	return sectPr(mm(12), mm(15), mm(12), mm(15))
}

// ---- Package ----

const docNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"`

const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

func (d *docx) pack(sect string, branded bool) ([]byte, error) {
	files := []struct{ name, body string }{}
	add := func(name, body string) { files = append(files, struct{ name, body string }{name, body}) }

	types := `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Default Extension="jpeg" ContentType="image/jpeg"/><Default Extension="png" ContentType="image/png"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
		`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`
	if branded {
		types += `<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>`
	}
	add("[Content_Types].xml", xmlHead+types+`</Types>`)
	add("_rels/.rels", xmlHead+`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>`+
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>`+
		`</Relationships>`)
	add("docProps/core.xml", xmlHead+`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">`+
		`<dc:title>`+xmlText(d.c.Contact.Name)+`</dc:title><dc:creator>9itchi</dc:creator></cp:coreProperties>`)

	rels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
		`<Relationship Id="rIdNumbering" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`
	if d.photo != nil && d.photo.usedID > 0 {
		rels += `<Relationship Id="rIdPhoto" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/photo.` + d.photo.ext + `"/>`
	}
	if branded {
		rels += `<Relationship Id="rIdFooter" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>`
		add("word/footer1.xml", xmlHead+`<w:ftr `+docNS+`>`+para(pp{align: "center"}, run(BrandMark, rp{size: 7.5, italic: true, color: "6E6E6E"}))+`</w:ftr>`)
	}
	add("word/_rels/document.xml.rels", xmlHead+rels+`</Relationships>`)
	add("word/styles.xml", xmlHead+`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`+
		`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="`+docBodyFont(d.c.Style)+`" w:hAnsi="`+docBodyFont(d.c.Style)+`" w:eastAsia="Calibri" w:cs="Arial"/><w:color w:val="1A1A1A"/><w:sz w:val="21"/><w:szCs w:val="21"/><w:lang w:val="fr-FR"/></w:rPr></w:rPrDefault>`+
		`<w:pPrDefault><w:pPr><w:spacing w:after="0" w:line="252" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>`+
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>`+
		`</w:styles>`)
	glyph, nl, nh := listXML(d.c.Style)
	add("word/numbering.xml", xmlHead+`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`+
		`<w:abstractNum w:abstractNumId="0"><w:multiLevelType w:val="singleLevel"/><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="`+glyph+`"/><w:lvlJc w:val="left"/>`+
		`<w:pPr><w:ind w:left="`+strconv.Itoa(nl)+`" w:hanging="`+strconv.Itoa(nh)+`"/></w:pPr></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>`)
	add("word/document.xml", xmlHead+`<w:document `+docNS+`><w:body>`+d.body.String()+sect+`</w:body></w:document>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, fmt.Errorf("docx: %w", err)
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			return nil, fmt.Errorf("docx: %w", err)
		}
	}
	if d.photo != nil && d.photo.usedID > 0 {
		w, err := zw.Create("word/media/photo." + d.photo.ext)
		if err != nil {
			return nil, fmt.Errorf("docx: %w", err)
		}
		if _, err := w.Write(d.photo.data); err != nil {
			return nil, fmt.Errorf("docx: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("docx: %w", err)
	}
	return buf.Bytes(), nil
}
