package core

import (
	"strings"

	"github.com/go-pdf/fpdf"
)

type contactItem struct {
	kind, text, url string
	link            bool
}

func (d *cv) contactItems() []contactItem {
	var out []contactItem
	c := d.c.Contact
	if strings.TrimSpace(c.Email) != "" {
		out = append(out, contactItem{kind: "mail", text: c.Email})
	}
	if strings.TrimSpace(c.Phone) != "" {
		out = append(out, contactItem{kind: "phone", text: c.Phone})
	}
	if strings.TrimSpace(c.Location) != "" {
		out = append(out, contactItem{kind: "pin", text: c.Location})
	}
	for _, l := range nonEmpty(c.Links...) {
		k := "web"
		switch ll := strings.ToLower(l); {
		case strings.Contains(ll, "linkedin."):
			k = "linkedin"
		case strings.Contains(ll, "github."):
			k = "github"
		}
		out = append(out, contactItem{kind: k, text: shortLink(l), url: safeURL(l), link: true})
	}
	return out
}

// contactIcons lays the contact details out as icon + text items that wrap onto several lines.
func (d *cv) contactIcons(x, w, size float64, col, linkCol rgb, align string) {
	items := d.contactItems()
	if len(items) == 0 {
		return
	}
	pdf := d.pdf
	is := size * ptToMM * 0.95
	gapIcon, gapItem := is*0.45, 5.0
	d.font(sans, "", size, col)
	widths := make([]float64, len(items))
	for i, it := range items {
		widths[i] = is + gapIcon + pdf.GetStringWidth(cleanText(it.text))
	}
	var lines [][]int
	cur, curW := []int{}, 0.0
	for i := range items {
		add := widths[i]
		if len(cur) > 0 {
			add += gapItem
		}
		if len(cur) > 0 && curW+add > w {
			lines = append(lines, cur)
			cur, curW, add = []int{}, 0, widths[i]
		}
		cur = append(cur, i)
		curW += add
	}
	lines = append(lines, cur)
	lh := d.lh(size) * 1.15
	for _, ln := range lines {
		y := pdf.GetY()
		total := gapItem * float64(len(ln)-1)
		for _, i := range ln {
			total += widths[i]
		}
		px := x
		if align == "C" && total < w {
			px = x + (w-total)/2
		}
		for _, i := range ln {
			it := items[i]
			d.icon(it.kind, px, y+(lh-is)/2, is, col)
			c := col
			if it.link {
				c = linkCol
			}
			d.font(sans, "", size, c)
			tw := pdf.GetStringWidth(cleanText(it.text))
			pdf.SetXY(px+is+gapIcon, y)
			pdf.CellFormat(tw, lh, cleanText(it.text), "", 0, "L", false, 0, it.url)
			px += widths[i] + gapItem
		}
		pdf.SetXY(x, y+lh)
	}
}

// icon draws a small solid pictogram in the box (x, y, s, s).
func (d *cv) icon(kind string, x, y, s float64, col rgb) {
	p := d.pdf
	p.SetFillColor(col.r, col.g, col.b)
	p.SetDrawColor(255, 255, 255)
	p.SetLineWidth(s * 0.09)
	switch kind {
	case "mail":
		p.Rect(x, y+s*0.15, s, s*0.7, "F")
		p.Line(x, y+s*0.15, x+s/2, y+s*0.55)
		p.Line(x+s, y+s*0.15, x+s/2, y+s*0.55)
	case "phone":
		p.RoundedRect(x+s*0.2, y, s*0.6, s, s*0.12, "1234", "F")
		p.SetFillColor(255, 255, 255)
		p.Rect(x+s*0.3, y+s*0.15, s*0.4, s*0.55, "F")
	case "pin":
		p.Circle(x+s/2, y+s*0.38, s*0.36, "F")
		p.Polygon([]fpdf.PointType{{X: x + s*0.17, Y: y + s*0.55}, {X: x + s*0.83, Y: y + s*0.55}, {X: x + s/2, Y: y + s}}, "F")
		p.SetFillColor(255, 255, 255)
		p.Circle(x+s/2, y+s*0.38, s*0.14, "F")
	case "linkedin":
		p.RoundedRect(x, y, s, s, s*0.12, "1234", "F")
		p.SetFillColor(255, 255, 255)
		p.Rect(x+s*0.2, y+s*0.4, s*0.14, s*0.4, "F")
		p.Circle(x+s*0.27, y+s*0.26, s*0.09, "F")
		p.Rect(x+s*0.42, y+s*0.4, s*0.13, s*0.4, "F")
		p.Rect(x+s*0.7, y+s*0.55, s*0.12, s*0.25, "F")
		p.Circle(x+s*0.64, y+s*0.52, s*0.12, "F")
	case "github":
		p.Circle(x+s/2, y+s/2, s/2, "F")
		p.SetFillColor(255, 255, 255)
		p.Circle(x+s/2, y+s*0.52, s*0.25, "F")
	default:
		p.SetDrawColor(col.r, col.g, col.b)
		p.SetLineWidth(s * 0.08)
		p.Circle(x+s/2, y+s/2, s*0.46, "D")
		p.Ellipse(x+s/2, y+s/2, s*0.2, s*0.46, 0, "D")
		p.Line(x+s*0.04, y+s/2, x+s*0.96, y+s/2)
	}
	p.SetDrawColor(0, 0, 0)
}
