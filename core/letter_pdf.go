package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"9itchi-resume-api/models/api"

	"github.com/go-pdf/fpdf"
)

// LetterInput is a cover letter to lay out as an A4 PDF. Contact is the sender block (nil: none); Style is the
// CV's style, whose body font and accent the letter borrows (nil: the defaults).
type LetterInput struct {
	Language    string // fr | en | ar
	Subject     string
	Body        string
	SenderEmail string // shown instead of the CV e-mail when set
	Contact     *api.Contact
	Style       *api.CVStyle
	Now         time.Time // the date printed, already in Tunis time
}

const (
	letterFamily   = "LetterBody"
	letterArabic   = "LetterArabic"
	letterMargin   = 22.0 // mm
	letterMinSize  = 10.0 // pt: a CV's 9 pt reads too small on a letter
	letterDefSize  = 11.0
	letterLineRate = 1.45
)

var letterMonths = map[string][12]string{
	"fr": {"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	// Maghreb month names, as written in Tunisia.
	"ar": {"جانفي", "فيفري", "مارس", "أفريل", "ماي", "جوان", "جويلية", "أوت", "سبتمبر", "أكتوبر", "نوفمبر", "ديسمبر"},
}

var subjectLabels = map[string]string{"fr": "Objet : ", "en": "Subject: ", "ar": "الموضوع: "}

// letterDate is the day in the letter's language, e.g. "28 novembre 2026".
func letterDate(lang string, t time.Time) string {
	months, ok := letterMonths[lang]
	if !ok {
		months = letterMonths["fr"]
	}
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

type letter struct {
	pdf   *fpdf.Fpdf
	rtl   bool
	size  float64
	w     float64
	ink   rgb
	plain string // family of Latin text
}

func (l *letter) lh(size float64) float64 { return size * ptToMM * letterLineRate }

// line writes one paragraph, wrapped to the text width. Arabic text is shaped and drawn in visual order; any
// other text goes through cleanText and fpdf's own wrapping.
func (l *letter) line(text, style string, size float64, col rgb, align string) {
	l.pdf.SetTextColor(col.r, col.g, col.b)
	l.pdf.SetX(letterMargin)
	if !hasArabic(text) {
		l.pdf.SetFont(l.plain, style, size)
		l.pdf.MultiCell(l.w, l.lh(size), cleanText(text), "", align, false)
		return
	}
	l.pdf.SetFont(letterArabic, style, size)
	for _, ln := range l.wrapArabic(cleanArabic(text)) {
		l.pdf.SetX(letterMargin)
		l.pdf.CellFormat(l.w, l.lh(size), visualLine(shapeArabic(ln)), "", 2, align, false, 0, "")
	}
}

// wrapArabic breaks a logical-order paragraph into lines that fit the width, measured on the shaped words.
func (l *letter) wrapArabic(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if l.pdf.GetStringWidth(shapeArabic(cur+" "+w)) > l.w {
			lines = append(lines, cur)
			cur = w
			continue
		}
		cur += " " + w
	}
	return append(lines, cur)
}

// cleanArabic turns control characters into spaces; the Arabic font draws the rest.
func cleanArabic(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// RenderLetter lays out a cover letter: the sender block, the date, the subject in bold, then the body
// paragraphs. Arabic letters are right aligned.
func RenderLetter(ctx context.Context, in LetterInput) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	body := bodyFonts[bodyFontKey(in.Style)]
	amiri := bodyFonts["amiri"]
	for _, f := range []struct{ family, style, file string }{
		{letterFamily, "", body.regular}, {letterFamily, "B", body.bold},
		{letterArabic, "", amiri.regular}, {letterArabic, "B", amiri.bold},
	} {
		data, err := fontFiles.ReadFile("fonts/" + f.file)
		if err != nil {
			return nil, fmt.Errorf("load font: %w", err)
		}
		pdf.AddUTF8FontFromBytes(f.family, f.style, data)
	}
	pdf.SetCreator("9itchi", true)
	pdf.SetCellMargin(0)
	pdf.SetMargins(letterMargin, letterMargin, letterMargin)
	pdf.SetAutoPageBreak(true, letterMargin)
	pdf.AddPage()

	l := &letter{pdf: pdf, rtl: in.Language == "ar", size: letterDefSize, w: 210 - 2*letterMargin, ink: cvInk, plain: letterFamily}
	accent := cvBlue
	if s := in.Style; s != nil {
		if s.BaseSize > 0 {
			l.size = max(s.BaseSize, letterMinSize)
		}
		if c := hexRGB(s.Accent); c != nil {
			accent = *c
		}
		if c := hexRGB(s.Text); c != nil {
			l.ink = *c
		}
	}
	start, end := "L", "R"
	if l.rtl {
		start, end = "R", "L"
	}

	if c := in.Contact; c != nil {
		if strings.TrimSpace(c.Name) != "" {
			pdf.SetTitle(cleanText(c.Name), true)
			l.line(c.Name, "B", l.size+4, accent, start)
		}
		email := c.Email
		if in.SenderEmail != "" {
			email = in.SenderEmail
		}
		for _, s := range []string{email, c.Phone, c.Location} {
			if strings.TrimSpace(s) != "" {
				l.line(s, "", l.size, cvMuted, start)
			}
		}
		pdf.Ln(l.lh(l.size))
	}
	l.line(letterDate(in.Language, in.Now), "", l.size, l.ink, end)
	pdf.Ln(l.lh(l.size))
	if subject := strings.TrimSpace(in.Subject); subject != "" {
		label, ok := subjectLabels[in.Language]
		if !ok {
			label = subjectLabels["fr"]
		}
		l.line(label+subject, "B", l.size, l.ink, start)
		pdf.Ln(l.lh(l.size))
	}
	align := "J"
	if l.rtl {
		align = "R"
	}
	for _, p := range strings.Split(strings.ReplaceAll(in.Body, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(p) == "" {
			pdf.Ln(l.lh(l.size) / 2)
			continue
		}
		l.line(p, "", l.size, l.ink, align)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render letter: %w", err)
	}
	return buf.Bytes(), nil
}
