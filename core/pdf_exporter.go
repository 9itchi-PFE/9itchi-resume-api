package core

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"strings"
	"unicode"

	"9itchi-resume-api/models/api"

	"github.com/go-pdf/fpdf"
)

// The three layouts follow the CV templates drawn for screens 4.1 and 4.3:
//
//   - classic: Source Sans 3, optional round photo, name and headline in blue small caps (#2F5597),
//     blue uppercase section titles over a blue rule, dates on the right.
//   - modern: a dark side column (#1F2A44) with photo, contact, skills and languages; Source Serif 4 titles.
//   - compact: black and white, optional round photo above the name, centred name, bold uppercase titles over a black rule.
//
// All stay text based (no text in images) so applicant tracking systems can read them.

//go:embed fonts/*.ttf
var fontFiles embed.FS

const (
	sans     = "SourceSans3"
	sansSemi = "SourceSans3SB"
	serif    = "SourceSerif4"
	ptToMM   = 0.3528
)

type rgb struct{ r, g, b int }

var (
	cvBlue    = rgb{47, 85, 151}   // #2F5597
	cvInk     = rgb{26, 26, 26}    // #1A1A1A
	cvGrey    = rgb{51, 51, 51}    // #333333
	cvMuted   = rgb{89, 96, 112}   // #596070
	cvNavy    = rgb{31, 42, 68}    // #1F2A44
	cvSideTxt = rgb{232, 236, 244} // #E8ECF4
	cvBlack   = rgb{17, 17, 17}    // #111111
	cvCompact = rgb{68, 68, 68}    // #444444
)

type pdfExporter struct{}

func NewPDFExporter() Exporter { return pdfExporter{} }

func (pdfExporter) Format() string { return "pdf" }

// headings are the section titles per language.
var headings = map[string]map[string]string{
	"fr": {"summary": "Profil", "experience": "Expérience professionnelle", "education": "Formation", "projects": "Projets académiques",
		"skills": "Compétences", "languages": "Langues", "certifications": "Certifications", "contact": "Contact"},
	"en": {"summary": "Summary", "experience": "Professional experience", "education": "Education", "projects": "Academic projects",
		"skills": "Skills", "languages": "Languages", "certifications": "Certifications", "contact": "Contact"},
}

// BrandMark is the footer printed on free-plan downloads.
const BrandMark = "CV créé avec 9itchi · 9itchi-pfe.com"

// inFont reports whether the embedded (Latin) subset has a glyph for r.
func inFont(r rune) bool {
	switch {
	case r >= 0x20 && r <= 0x7e, r >= 0xa0 && r <= 0x17f, r == 0x192, r >= 0x2c6 && r <= 0x2dc,
		r >= 0x2000 && r <= 0x2027, r >= 0x2030 && r <= 0x203a, r == 0x20ac, r == 0x2122, r >= 0x2190 && r <= 0x2193, r == 0x2212:
		return true
	}
	return false
}

// cleanText keeps what the fonts can draw: control characters become spaces and other missing glyphs '?'.
// fpdf escapes the text, so no markup can leak into the PDF.
func cleanText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r' || r < 0x20 || r == 0x7f:
			b.WriteByte(' ')
		case inFont(r):
			b.WriteRune(r)
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

func nonEmpty(parts ...string) []string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func join(sep string, parts ...string) string { return strings.Join(nonEmpty(parts...), sep) }

func dates(start, end string) string { return join(" — ", start, end) }

// splitLanguage reads "Arabe: Langue maternelle" (or "Arabe — courant", "Arabe - B2") as name and level.
func splitLanguage(s string) (string, string) {
	for _, sep := range []string{":", " — ", " – ", " - "} {
		if i := strings.Index(s, sep); i > 0 {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len(sep):])
		}
	}
	return strings.TrimSpace(s), ""
}

func shortLink(l string) string {
	l = strings.TrimSpace(l)
	for _, p := range []string{"https://", "http://", "www."} {
		l = strings.TrimPrefix(l, p)
	}
	return strings.TrimSuffix(l, "/")
}

// cv draws one resume. x and w are the current column.
type cv struct {
	pdf     *fpdf.Fpdf
	c       api.ResumeContent
	titles  map[string]string
	lk      look
	body    float64 // pt
	x, w    float64 // column, mm
	pending []seg   // left side of the next row, set by the row's left callback
	lang    string
}

// tech writes the project's technologies under its description.
func (d *cv) tech(p api.Project, size float64, col rgb) {
	t := techText(p, d.lang)
	if t == "" {
		return
	}
	d.font(sans, "I", size, col)
	d.pdf.SetX(d.x + d.lk.indent)
	d.pdf.MultiCell(d.w-d.lk.indent, d.lh(size), cleanText(t), "", "L", false)
}

func (d *cv) lh(size float64) float64 { return size * ptToMM * d.lk.lineH }

func (d *cv) font(family, style string, size float64, col rgb) {
	d.pdf.SetFont(family, style, size)
	d.pdf.SetTextColor(col.r, col.g, col.b)
}

// para writes wrapped text across the column.
func (d *cv) para(text string, size float64) {
	if strings.TrimSpace(text) == "" {
		return
	}
	d.pdf.SetX(d.x)
	d.pdf.MultiCell(d.w, d.lh(size), cleanText(text), "", "L", false)
}

// row writes left text and right text on one line; the left text wraps before the right one.
func (d *cv) row(left, right func(), rightText string, size float64) {
	pdf := d.pdf
	if d.lk.dateBelow && strings.TrimSpace(rightText) != "" {
		d.row(left, right, "", size)
		right()
		pdf.SetX(d.x)
		pdf.CellFormat(d.w, d.lh(size), cleanText(rightText), "", 1, "L", false, 0, "")
		return
	}
	y := pdf.GetY()
	rw := 0.0
	if strings.TrimSpace(rightText) != "" {
		right()
		rw = pdf.GetStringWidth(cleanText(rightText)) + 2
		pdf.SetXY(d.x+d.w-rw, y)
		pdf.CellFormat(rw, d.lh(size), cleanText(rightText), "", 0, "R", false, 0, "")
	}
	pdf.SetXY(d.x, y)
	left()
	page := pdf.PageNo()
	end := y + d.lh(size)
	pdf.SetXY(d.x, y)
	d.leftText(d.w-rw, size)
	if pdf.PageNo() == page && pdf.GetY() < end {
		pdf.SetY(end)
	}
}

// leftText writes the segments the left callback of row put in d.pending.
func (d *cv) leftText(w, size float64) {
	if d.pending == nil {
		d.pdf.Ln(d.lh(size))
		return
	}
	segs := d.pending
	d.pending = nil
	y0 := d.pdf.GetY()
	defer func() {
		x := d.x
		for _, s := range segs {
			d.font(s.family, s.style, size, s.col)
			sw := d.pdf.GetStringWidth(cleanText(s.text))
			if s.link != "" {
				d.pdf.LinkString(x, y0, minf(sw, w), d.lh(size), s.link)
			}
			x += sw
		}
	}()
	total := 0.0
	for _, s := range segs {
		d.font(s.family, s.style, size, s.col)
		total += d.pdf.GetStringWidth(cleanText(s.text))
	}
	if total <= w || len(segs) == 1 {
		if len(segs) == 1 {
			d.font(segs[0].family, segs[0].style, size, segs[0].col)
			d.pdf.SetX(d.x)
			d.pdf.MultiCell(w, d.lh(size), cleanText(segs[0].text), "", "L", false)
			return
		}
		d.pdf.SetX(d.x)
		for _, s := range segs {
			d.font(s.family, s.style, size, s.col)
			t := cleanText(s.text)
			d.pdf.CellFormat(d.pdf.GetStringWidth(t), d.lh(size), t, "", 0, "L", false, 0, "")
		}
		d.pdf.Ln(d.lh(size))
		return
	}
	// Too long for coloured runs: one wrapped run in the first segment's style.
	var all []string
	for _, s := range segs {
		all = append(all, s.text)
	}
	d.font(segs[0].family, segs[0].style, size, segs[0].col)
	d.pdf.SetX(d.x)
	d.pdf.MultiCell(w, d.lh(size), cleanText(strings.Join(all, "")), "", "L", false)
}

type seg struct {
	text, family, style string
	col                 rgb
	link                string
}

// linked makes a segment clickable when the entry has a web link.
func linked(s seg, url string) seg {
	s.link = safeURL(url)
	return s
}

func (d *cv) segs(s ...seg) func() { return func() { d.pending = s } }

func (d *cv) bullets(lines []string, size float64, col rgb) {
	indent := d.lk.indent
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		d.font(sans, "", size, col)
		y := d.pdf.GetY()
		d.pdf.SetXY(d.x+1, y)
		d.pdf.CellFormat(indent-1, d.lh(size), d.lk.listChar, "", 0, "L", false, 0, "")
		d.pdf.SetXY(d.x+indent, y)
		d.pdf.MultiCell(d.w-indent, d.lh(size), cleanText(l), "", "L", false)
	}
}

// smallCaps draws one line in faux small caps (capitals full size, lower case as smaller capitals),
// the way the classic template shows the headline. It falls back to wrapped capitals when too long.
func (d *cv) smallCaps(text string, size float64, col rgb, w float64) {
	text = cleanText(text)
	if strings.TrimSpace(text) == "" {
		return
	}
	type run struct {
		t     string
		small bool
	}
	var runs []run
	for _, r := range text {
		small := unicode.IsLower(r)
		t := string(unicode.ToUpper(r))
		if !inFont(unicode.ToUpper(r)) {
			t = string(r)
		}
		if n := len(runs); n > 0 && runs[n-1].small == small {
			runs[n-1].t += t
		} else {
			runs = append(runs, run{t, small})
		}
	}
	sizeOf := func(r run) float64 {
		if r.small {
			return size * 0.78
		}
		return size
	}
	total := 0.0
	for _, r := range runs {
		d.font(sans, "", sizeOf(r), col)
		total += d.pdf.GetStringWidth(r.t)
	}
	x0, y := d.pdf.GetX(), d.pdf.GetY()
	if total > w {
		d.font(sans, "", size*0.78, col)
		d.pdf.MultiCell(w, d.lh(size), strings.ToUpper(text), "", "L", false)
		return
	}
	base := y + d.lh(size)*0.78
	x := x0
	for _, r := range runs {
		d.font(sans, "", sizeOf(r), col)
		d.pdf.Text(x, base, r.t)
		x += d.pdf.GetStringWidth(r.t)
	}
	d.pdf.SetXY(x0, y+d.lh(size))
}

// photo draws the picture cropped to a circle. It returns false when the image cannot be read.
func (d *cv) photo(opts RenderOptions, cx, cy, r float64, border *rgb) bool {
	if len(opts.Photo) == 0 {
		return false
	}
	typ := "JPG"
	if opts.PhotoFormat == "png" {
		typ = "PNG"
	}
	info := d.pdf.RegisterImageOptionsReader("photo", fpdf.ImageOptions{ImageType: typ}, bytes.NewReader(opts.Photo))
	if d.pdf.Err() || info == nil || info.Width() <= 0 || info.Height() <= 0 {
		d.pdf.ClearError()
		return false
	}
	iw, ih := info.Width(), info.Height()
	scale := 2 * r / minf(iw, ih)
	w, h := iw*scale, ih*scale
	d.pdf.ClipCircle(cx, cy, r, false)
	d.pdf.ImageOptions("photo", cx-w/2, cy-h/2, w, h, false, fpdf.ImageOptions{ImageType: typ}, 0, "")
	d.pdf.ClipEnd()
	if border != nil {
		d.pdf.SetDrawColor(border.r, border.g, border.b)
		d.pdf.SetLineWidth(0.5)
		d.pdf.Circle(cx, cy, r, "D")
	}
	return true
}

func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

type fontFiles4 struct{ regular, italic, bold, semi string }

// bodyFonts are the typefaces a user can pick for the whole CV; each is registered under the body family names,
// so every piece of text follows the choice. Families without a semibold use their bold.
var bodyFonts = map[string]fontFiles4{
	"sourcesans": {"SourceSans3-Regular.ttf", "SourceSans3-It.ttf", "SourceSans3-Bold.ttf", "SourceSans3-Semibold.ttf"},
	"lato":       {"Lato-Regular.ttf", "Lato-Italic.ttf", "Lato-Bold.ttf", "Lato-Bold.ttf"},
	"carlito":    {"Carlito-Regular.ttf", "Carlito-Italic.ttf", "Carlito-Bold.ttf", "Carlito-Bold.ttf"},
	"tinos":      {"Tinos-Regular.ttf", "Tinos-Italic.ttf", "Tinos-Bold.ttf", "Tinos-Bold.ttf"},
}

func bodyFontKey(s *api.CVStyle) string {
	if s != nil {
		if _, ok := bodyFonts[s.BodyFont]; ok {
			return s.BodyFont
		}
	}
	return "sourcesans"
}

func (pdfExporter) Render(ctx context.Context, c api.ResumeContent, layout string, opts RenderOptions) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	body := bodyFonts[bodyFontKey(c.Style)]
	for _, f := range []struct{ family, style, file string }{
		{sans, "", body.regular}, {sans, "I", body.italic}, {sans, "B", body.bold},
		{sansSemi, "", body.semi}, {serif, "B", "SourceSerif4-Bold.ttf"},
	} {
		data, err := fontFiles.ReadFile("fonts/" + f.file)
		if err != nil {
			return nil, fmt.Errorf("load font: %w", err)
		}
		pdf.AddUTF8FontFromBytes(f.family, f.style, data)
	}
	pdf.SetTitle(cleanText(c.Contact.Name), true)
	pdf.SetCreator("9itchi", true)
	pdf.SetCellMargin(0)
	titles, ok := headings[opts.Language]
	if !ok {
		titles = headings["fr"]
	}
	d := &cv{pdf: pdf, c: visibleEntries(c), titles: titles, lk: newLook(layout, c.Style), lang: opts.Language}
	if !d.lk.showPhoto {
		opts.Photo = nil
	}
	if d.lk.bg != nil && layout != "modern" {
		pdf.SetHeaderFunc(func() {
			pdf.SetFillColor(d.lk.bg.r, d.lk.bg.g, d.lk.bg.b)
			pdf.Rect(0, 0, 210, 297, "F")
		})
	}
	if opts.Branded {
		pdf.SetFooterFunc(func() {
			pdf.SetY(-9)
			pdf.SetX(0)
			d.font(sans, "I", 7.5, rgb{110, 110, 110})
			pdf.CellFormat(210, 4, BrandMark, "", 0, "C", false, 0, "https://9itchi-pfe.com")
		})
	}
	switch layout {
	case "modern":
		d.modern(opts)
	case "compact":
		d.compact(opts)
	default:
		d.classic(opts)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// ---- Classique ----

func (d *cv) classic(opts RenderOptions) {
	pdf := d.pdf
	lk := d.lk
	d.body = lk.base
	pdf.SetMargins(lk.mx, lk.my, lk.mx)
	pdf.SetAutoPageBreak(true, lk.my)
	pdf.AddPage()
	d.x, d.w = lk.mx, 210-2*lk.mx
	blue := lk.col("rules", cvBlue)

	// Header: photo, name, headline in small caps, contact line.
	top := pdf.GetY()
	textX := d.x
	if d.photo(opts, d.x+13, top+13, 13, &blue) {
		textX = d.x + 26 + 6
	}
	tw := d.x + d.w - textX
	align := "L"
	if lk.center {
		align = "C"
	}
	pdf.SetXY(textX, top)
	d.font(d.nameFamily(), "B", lk.name, lk.col("name", lk.ink(rgb{17, 17, 17})))
	pdf.MultiCell(tw, lk.name*ptToMM*1.15, cleanText(d.c.Contact.Name), "", align, false)
	pdf.Ln(0.8)
	pdf.SetX(textX)
	if lk.center {
		d.font(sans, "", lk.title, lk.col("title", cvBlue))
		pdf.MultiCell(tw, d.lh(lk.title), cleanText(d.c.Contact.Headline), "", "C", false)
	} else {
		d.smallCaps(d.c.Contact.Headline, lk.title, lk.col("title", cvBlue), tw)
	}
	pdf.Ln(0.6)
	d.contactLine(textX, tw, lk.base-1, lk.ink(cvGrey), lk.col("links", cvBlue), align)
	if pdf.GetY() < top+26 && textX != d.x {
		pdf.SetY(top + 26)
	}
	pdf.Ln(2.5)

	head := func(t string) {
		pdf.Ln(2.4)
		d.font(sans, "", lk.head, lk.col("headings", cvBlue))
		pdf.SetX(d.x)
		pdf.CellFormat(d.w, d.lh(lk.head)*0.9, cleanText(lk.heading(t)), "", 1, "L", false, 0, "")
		d.rule(blue, 0.35)
		pdf.Ln(1)
	}
	b, eb := d.body, lk.entry
	ink := lk.ink(cvInk)
	entry := lk.col("entry_title", cvBlue)
	bold := func(t string) seg { return seg{text: t, family: sans, style: "B", col: ink} }
	plain := func(t string) seg { return seg{text: t, family: sans, style: pick(lk.subStyle, ""), col: ink} }
	accented := func(t string) seg { return seg{text: t, family: sans, style: pick(lk.subStyle, ""), col: entry} }
	dateCol := lk.col("dates", ink)
	rightPlain := func() { d.font(sans, pick(lk.dateStyle, ""), eb, dateCol) }
	rightBold := func() { d.font(sans, pick(lk.dateStyle, "B"), eb, dateCol) }
	between := func() { pdf.Ln(lk.gapOr(1)) }

	sections := map[string]func(){
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
			for i, e := range d.c.Education {
				if i > 0 {
					between()
				}
				d.row(d.segs(linked(bold(e.Degree), e.URL)), rightPlain, dates(e.Start, e.End), eb)
				if e.School != "" || e.Location != "" {
					d.row(d.segs(plain(e.School)), rightPlain, e.Location, eb)
				}
			}
		},
		"experience": func() {
			if len(d.c.Experience) == 0 {
				return
			}
			head(d.titles["experience"])
			for i, e := range d.c.Experience {
				if i > 0 {
					between()
				}
				d.row(d.segs(bold(e.Title)), rightBold, dates(e.Start, e.End), eb)
				if e.Company != "" || e.Kind != "" || e.Location != "" {
					parts := []seg{linked(accented(e.Company), e.URL)}
					if e.Kind != "" {
						parts = append(parts, plain(" ("+e.Kind+")"))
					}
					d.row(d.segs(parts...), rightPlain, e.Location, eb)
				}
				d.bullets(e.Description, b, ink)
			}
		},
		"projects": func() {
			if len(d.c.Projects) == 0 {
				return
			}
			head(d.titles["projects"])
			for _, p := range d.c.Projects {
				d.row(d.segs(linked(bold(p.Name), p.URL)), rightPlain, p.Year, eb)
				d.bullets(p.Description, b, ink)
				d.tech(p, b, ink)
			}
		},
		"skills": func() { d.skillsGrid(head, b, ink) },
		"certifications": func() {
			if len(d.c.Certifications) == 0 {
				return
			}
			head(d.titles["certifications"])
			for _, ce := range d.c.Certifications {
				d.row(d.segs(linked(bold(ce.Name), ce.URL), plain(prefix(", ", ce.Issuer))), rightPlain, ce.Year, eb)
			}
		},
		"languages": func() {
			if len(d.c.Languages) == 0 {
				return
			}
			head(d.titles["languages"])
			for _, l := range d.c.Languages {
				name, level := splitLanguage(l)
				d.row(d.segs(bold(name)), rightPlain, level, eb)
			}
		},
	}
	for _, key := range d.c.SectionOrder() {
		if f := sections[key]; f != nil {
			f()
		} else if x, ok := extraAt(d.c, key); ok && len(nonEmpty(x.Items...)) > 0 {
			head(x.Title)
			d.bullets(x.Items, b, ink)
		}
	}
}

// nameFamily is the font of the name and the modern headings.
func (d *cv) nameFamily() string {
	if d.lk.nameSerif {
		return serif
	}
	return sans
}

func prefix(p, s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return p + s
}

// rule draws a horizontal line across the column at the current position.
func (d *cv) rule(col rgb, width float64) {
	y := d.pdf.GetY()
	d.pdf.SetDrawColor(col.r, col.g, col.b)
	d.pdf.SetLineWidth(width)
	d.pdf.Line(d.x, y, d.x+d.w, y)
}

// contactLine writes email · phone · location · links as one wrapped line, links in linkCol. A line that fits is
// centred when align is "C".
func (d *cv) contactLine(x, w, size float64, col, linkCol rgb, align string) {
	pdf := d.pdf
	type part struct {
		t    string
		link bool
	}
	var parts []part
	for _, p := range nonEmpty(d.c.Contact.Email, d.c.Contact.Phone, d.c.Contact.Location) {
		parts = append(parts, part{p, false})
	}
	for _, l := range nonEmpty(d.c.Contact.Links...) {
		parts = append(parts, part{shortLink(l), true})
	}
	if len(parts) == 0 {
		return
	}
	sep := d.lk.separator()
	left, top, right, _ := pdf.GetMargins()
	pdf.SetLeftMargin(x)
	pdf.SetRightMargin(210 - x - w)
	start := x
	if align == "C" {
		d.font(sans, "", size, col)
		total := pdf.GetStringWidth(cleanText(sep)) * float64(len(parts)-1)
		for _, p := range parts {
			total += pdf.GetStringWidth(cleanText(p.t))
		}
		if total < w {
			start = x + (w-total)/2
		}
	}
	pdf.SetX(start)
	for i, p := range parts {
		if i > 0 {
			d.font(sans, "", size, col)
			pdf.Write(d.lh(size), sep)
		}
		c := col
		if p.link {
			c = linkCol
		}
		d.font(sans, "", size, c)
		pdf.Write(d.lh(size), cleanText(p.t))
	}
	pdf.Ln(d.lh(size))
	pdf.SetMargins(left, top, right)
}

// skillsGrid prints groups as "label | items" rows (38% / 62%), or the flat list.
func (d *cv) skillsGrid(head func(string), size float64, col rgb) {
	pdf := d.pdf
	if len(d.c.SkillGroups) == 0 {
		if len(nonEmpty(d.c.Skills...)) == 0 {
			return
		}
		head(d.titles["skills"])
		d.font(sans, "", size, col)
		d.para(strings.Join(nonEmpty(d.c.Skills...), ", "), size)
		return
	}
	head(d.titles["skills"])
	lw := d.w * 0.38
	for _, g := range d.c.SkillGroups {
		items := strings.Join(nonEmpty(g.Items...), ", ")
		if items == "" && g.Label == "" {
			continue
		}
		y := pdf.GetY()
		d.font(sans, "B", size, col)
		pdf.SetXY(d.x, y)
		pdf.MultiCell(lw-2, d.lh(size), cleanText(g.Label), "", "L", false)
		y1 := pdf.GetY()
		d.font(sans, "", size, col)
		pdf.SetXY(d.x+lw, y)
		pdf.MultiCell(d.w-lw, d.lh(size), cleanText(items), "", "L", false)
		if pdf.GetY() < y1 {
			pdf.SetY(y1)
		}
	}
}

// ---- Moderne ----

func (d *cv) modern(opts RenderOptions) {
	pdf := d.pdf
	lk := d.lk
	const (
		sideW = 210 * 0.34
		pad   = 7.0
	)
	top, mainP := lk.my, lk.mx
	side := lk.col("rules", cvNavy)
	d.body = lk.base
	pdf.SetHeaderFunc(func() {
		if lk.bg != nil {
			pdf.SetFillColor(lk.bg.r, lk.bg.g, lk.bg.b)
			pdf.Rect(0, 0, 210, 297, "F")
		}
		pdf.SetFillColor(side.r, side.g, side.b)
		pdf.Rect(0, 0, sideW, 297, "F")
	})
	pdf.SetMargins(pad, top, 210-sideW+pad)
	pdf.SetAutoPageBreak(false, top)
	pdf.AddPage()

	// Side column.
	d.x, d.w = pad, sideW-2*pad
	y := top
	if d.photo(opts, sideW/2, y+11, 11, nil) {
		y += 22 + 4
	}
	pdf.SetY(y)
	b := d.body
	eb := lk.entry
	sideHead := func(t string) {
		pdf.Ln(2.5)
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
			if len(d.c.SkillGroups) > 0 {
				for _, g := range d.c.SkillGroups {
					if g.Label != "" {
						d.font(sansSemi, "", b-0.5, cvSideTxt)
						d.para(g.Label, b-0.5)
					}
					sideLines(strings.Join(nonEmpty(g.Items...), " · "))
				}
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
			for _, l := range d.c.Languages {
				name, level := splitLanguage(l)
				sideLines(join(" · ", name, level))
			}
		},
	}
	order := d.c.SectionOrder()
	for _, key := range order {
		if f := sideSections[key]; f != nil {
			f()
		}
	}

	// Main column.
	d.x, d.w = sideW+mainP, 210-sideW-2*mainP
	pdf.SetMargins(d.x, top, mainP)
	pdf.SetAutoPageBreak(true, top)
	pdf.SetXY(d.x, top)
	ink := lk.ink(cvInk)
	d.font(d.nameFamily(), "B", lk.name, lk.col("name", cvNavy))
	d.para(d.c.Contact.Name, lk.name)
	if d.c.Contact.Headline != "" {
		pdf.Ln(0.5)
		d.font(sans, "", lk.title, lk.col("title", cvMuted))
		d.para(d.c.Contact.Headline, lk.title)
	}
	head := func(t string) {
		pdf.Ln(3)
		d.font(d.nameFamily(), "B", lk.head, lk.col("headings", cvNavy))
		pdf.SetX(d.x)
		pdf.CellFormat(d.w, d.lh(lk.head), cleanText(lk.heading(t)), "", 1, "L", false, 0, "")
		pdf.Ln(0.4)
	}
	bold := func(t string) seg { return seg{text: t, family: sans, style: "B", col: lk.col("entry_title", ink)} }
	muted := func(t string) seg { return seg{text: t, family: sans, style: pick(lk.subStyle, ""), col: cvMuted} }
	right := func() { d.font(sans, pick(lk.dateStyle, ""), eb, lk.col("dates", cvMuted)) }
	between := func() { pdf.Ln(lk.gapOr(0.8)) }
	mainSections := map[string]func(){
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
				d.row(d.segs(linked(bold(join(" — ", e.School, e.Degree)), e.URL)), right, dates(e.Start, e.End), eb)
				if e.Location != "" {
					d.row(d.segs(muted(e.Location)), right, "", eb)
				}
				between()
			}
		},
		"experience": func() {
			if len(d.c.Experience) == 0 {
				return
			}
			head(d.titles["experience"])
			for _, e := range d.c.Experience {
				d.row(d.segs(linked(bold(join(" — ", e.Title, e.Company)), e.URL)), right, dates(e.Start, e.End), eb)
				if l := join(" · ", e.Kind, e.Location); l != "" {
					d.row(d.segs(muted(l)), right, "", eb)
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
	}
	for _, key := range order {
		if f := mainSections[key]; f != nil {
			f()
		} else if x, ok := extraAt(d.c, key); ok && len(nonEmpty(x.Items...)) > 0 {
			head(x.Title)
			d.bullets(x.Items, b, ink)
		}
	}
}

// spaced imitates the letter spacing of the small side titles.
func spaced(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ---- Compact ----

func (d *cv) compact(opts RenderOptions) {
	pdf := d.pdf
	lk := d.lk
	d.body = lk.base
	pdf.SetMargins(lk.mx, lk.my, lk.mx)
	pdf.SetAutoPageBreak(true, lk.my)
	pdf.AddPage()
	d.x, d.w = lk.mx, 210-2*lk.mx
	b, eb := d.body, lk.entry
	ink := lk.ink(cvBlack)
	rule := lk.col("rules", cvBlack)
	align := "L"
	if lk.center {
		align = "C"
	}

	nameText := cleanText(d.c.Contact.Name)
	if lk.upper {
		nameText = cleanText(spaced(strings.ToUpper(d.c.Contact.Name)))
	}
	textX, tw := d.x, d.w
	top := pdf.GetY()
	if len(opts.Photo) > 0 {
		const r = 11.0
		cx := d.x + r
		if lk.center {
			cx = 105
		}
		if d.photo(opts, cx, top+r, r, nil) {
			top += 2*r + 2
		}
	}
	pdf.SetXY(textX, top)
	d.font(d.nameFamily(), "B", lk.name, lk.col("name", ink))
	pdf.MultiCell(tw, d.lh(lk.name), nameText, "", align, false)
	if d.c.Contact.Headline != "" {
		d.font(sans, "", lk.title, lk.col("title", lk.ink(cvCompact)))
		pdf.SetX(textX)
		pdf.MultiCell(tw, d.lh(lk.title), cleanText(d.c.Contact.Headline), "", align, false)
	}
	var contact []string
	contact = append(contact, d.c.Contact.Email, d.c.Contact.Phone, d.c.Contact.Location)
	for _, l := range d.c.Contact.Links {
		contact = append(contact, shortLink(l))
	}
	if line := join(lk.separator(), contact...); line != "" {
		d.font(sans, "", b-0.5, lk.ink(cvCompact))
		pdf.SetX(textX)
		pdf.MultiCell(tw, d.lh(b-0.5), cleanText(line), "", align, false)
	}
	if pdf.GetY() < top {
		pdf.SetY(top)
	}
	head := func(t string) {
		pdf.Ln(2.6)
		d.font(sans, "B", lk.head, lk.col("headings", cvBlack))
		pdf.SetX(d.x)
		pdf.CellFormat(d.w, d.lh(lk.head)*0.9, cleanText(lk.heading(t)), "", 1, "L", false, 0, "")
		d.rule(rule, 0.3)
		pdf.Ln(1)
	}
	bold := func(t string) seg { return seg{text: t, family: sans, style: "B", col: lk.col("entry_title", ink)} }
	plain := func(t string) seg { return seg{text: t, family: sans, style: pick(lk.subStyle, ""), col: ink} }
	right := func() { d.font(sans, pick(lk.dateStyle, ""), eb, lk.col("dates", ink)) }
	sections := map[string]func(){
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
				d.row(d.segs(linked(bold(join(" — ", e.School, e.Degree)), e.URL)), right, dates(e.Start, e.End), eb)
			}
		},
		"experience": func() {
			if len(d.c.Experience) == 0 {
				return
			}
			head(d.titles["experience"])
			for _, e := range d.c.Experience {
				d.row(d.segs(linked(bold(join(" : ", e.Company, e.Title)), e.URL), plain(prefix(", ", e.Location))), right, dates(e.Start, e.End), eb)
				d.bullets(e.Description, b, ink)
				pdf.Ln(lk.gapOr(0))
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
				pdf.Ln(lk.gapOr(0))
			}
		},
		"skills": func() {
			if len(d.c.SkillGroups) == 0 && len(nonEmpty(d.c.Skills...)) == 0 {
				return
			}
			head(d.titles["skills"])
			if len(d.c.SkillGroups) == 0 {
				d.font(sans, "", b, ink)
				d.para(strings.Join(nonEmpty(d.c.Skills...), ", "), b)
			}
			for _, g := range d.c.SkillGroups {
				d.row(d.segs(bold(prefix("", g.Label)+suffixIf(g.Label, " : ")), plain(strings.Join(nonEmpty(g.Items...), ", "))), right, "", eb)
			}
		},
		"certifications": func() {
			if len(d.c.Certifications) == 0 {
				return
			}
			head(d.titles["certifications"])
			for _, ce := range d.c.Certifications {
				d.row(d.segs(bold(ce.Name), plain(prefix(", ", ce.Issuer))), right, ce.Year, eb)
			}
		},
		"languages": func() {
			if len(d.c.Languages) == 0 {
				return
			}
			head(d.titles["languages"])
			var parts []string
			for _, l := range d.c.Languages {
				name, level := splitLanguage(l)
				if level != "" {
					name += " (" + level + ")"
				}
				parts = append(parts, name)
			}
			d.font(sans, "", b, ink)
			d.para(strings.Join(nonEmpty(parts...), " · "), b)
		},
	}
	for _, key := range d.c.SectionOrder() {
		if f := sections[key]; f != nil {
			f()
		} else if x, ok := extraAt(d.c, key); ok && len(nonEmpty(x.Items...)) > 0 {
			head(x.Title)
			d.bullets(x.Items, b, ink)
		}
	}
}

func suffixIf(s, suf string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return suf
}
