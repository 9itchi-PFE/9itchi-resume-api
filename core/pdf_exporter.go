package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"9itchi-resume-api/models/api"

	"github.com/go-pdf/fpdf"
)

// pdfStyle is the per-layout look. Every layout stays single column, plain text, no images (ATS friendly).
type pdfStyle struct {
	family string
	body   float64
	margin float64
}

var pdfStyles = map[string]pdfStyle{
	"classic": {family: "Times", body: 11, margin: 18},
	"modern":  {family: "Helvetica", body: 10.5, margin: 18},
	"compact": {family: "Helvetica", body: 9.5, margin: 12},
}

type pdfExporter struct{}

func NewPDFExporter() Exporter { return pdfExporter{} }

func (pdfExporter) Format() string { return "pdf" }

// pdfText converts s to the core-font encoding (cp1252/latin-1 bytes). Control characters become spaces
// and runes the core fonts cannot show become '?'. fpdf escapes the text, so no markup can leak into the PDF.
func pdfText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			b.WriteByte(' ')
		case r < 0x7f || (r >= 0xa0 && r <= 0xff):
			b.WriteByte(byte(r))
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

func (pdfExporter) Render(ctx context.Context, c api.ResumeContent, layout string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	st, ok := pdfStyles[layout]
	if !ok {
		st = pdfStyles["classic"]
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(st.margin, st.margin, st.margin)
	pdf.SetAutoPageBreak(true, st.margin)
	pdf.SetTitle(pdfText(c.Contact.Name), false)
	pdf.AddPage()

	w := 0.0 // full width
	lh := st.body * 0.5

	heading := func(t string) {
		pdf.Ln(2)
		pdf.SetFont(st.family, "B", st.body+1.5)
		pdf.CellFormat(w, lh+1, pdfText(strings.ToUpper(t)), "B", 1, "L", false, 0, "")
		pdf.Ln(1)
		pdf.SetFont(st.family, "", st.body)
	}
	line := func(style string, size float64, t string) {
		if strings.TrimSpace(t) == "" {
			return
		}
		pdf.SetFont(st.family, style, size)
		pdf.MultiCell(w, lh, pdfText(t), "", "L", false)
	}
	join := func(sep string, parts ...string) string {
		var out []string
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				out = append(out, p)
			}
		}
		return strings.Join(out, sep)
	}

	line("B", st.body+7, c.Contact.Name)
	line("", st.body, join(" | ", c.Contact.Email, c.Contact.Phone, c.Contact.Location))
	line("", st.body, join(" | ", c.Contact.Links...))
	pdf.SetFont(st.family, "", st.body)

	if strings.TrimSpace(c.Summary) != "" {
		heading("Summary")
		line("", st.body, c.Summary)
	}
	if len(c.Experience) > 0 {
		heading("Experience")
		for _, e := range c.Experience {
			line("B", st.body, join(" - ", e.Title, e.Company))
			line("I", st.body-0.5, join(" - ", e.Start, e.End))
			for _, d := range e.Description {
				line("", st.body, "- "+d)
			}
			pdf.Ln(1.5)
		}
	}
	if len(c.Education) > 0 {
		heading("Education")
		for _, e := range c.Education {
			line("B", st.body, join(" - ", e.Degree, e.School))
			line("I", st.body-0.5, join(" - ", e.Start, e.End))
			pdf.Ln(1.5)
		}
	}
	if len(c.Skills) > 0 {
		heading("Skills")
		line("", st.body, strings.Join(c.Skills, ", "))
	}
	if len(c.Languages) > 0 {
		heading("Languages")
		line("", st.body, strings.Join(c.Languages, ", "))
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return buf.Bytes(), nil
}
