package core

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/ledongthuc/pdf"
)

// MaxExtractedText caps the text sent to the AI parser (ai-api accepts 60 000 characters).
const MaxExtractedText = 60000

// maxDocxXML caps the decompressed word/document.xml (zip bomb guard).
const maxDocxXML = 20 << 20

// ErrNoText means the file holds no extractable text (a scanned PDF, for example).
var ErrNoText = errors.New("no text found in file")

// ExtractText returns the plain text of an uploaded CV ("pdf" or "docx"), cut to MaxExtractedText runes.
func ExtractText(format string, data []byte) (text string, err error) {
	switch format {
	case "pdf":
		text, err = pdfPlainText(data)
	case "docx":
		text, err = docxText(data)
	default:
		return "", fmt.Errorf("unsupported format %q", format)
	}
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ErrNoText
	}
	if r := []rune(text); len(r) > MaxExtractedText {
		text = string(r[:MaxExtractedText])
	}
	return text, nil
}

func pdfPlainText(data []byte) (text string, err error) {
	// The PDF library can panic on malformed files: turn that into an error.
	defer func() {
		if r := recover(); r != nil {
			text, err = "", errors.New("unreadable pdf")
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("unreadable pdf")
	}
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		writeRows(&sb, p.Content().Text)
		if sb.Len() > MaxExtractedText*4 {
			break
		}
	}
	return sb.String(), nil
}

// rowTolerance is how far apart (in points) two glyph baselines may be and still sit on one line.
const rowTolerance = 1.0

// writeRows writes the glyphs of one page in the order they are drawn, starting a new line when the baseline moves.
// CV generators draw in reading order, column by column, so this keeps two-column layouts apart; sorting by position
// (GetTextByRow) mixed the columns and, as it ignores the page transform, read flipped pages (FlowCV, Chrome) bottom
// line first. Glyphs often come one per Text, so a gap wider than a character is written as a space, a much wider one
// (a right-hand date column) or a jump back to the left as three.
func writeRows(sb *strings.Builder, texts []pdf.Text) {
	var line strings.Builder
	flush := func() {
		if s := strings.TrimSpace(line.String()); s != "" {
			sb.WriteString(s)
			sb.WriteByte('\n')
		}
		line.Reset()
	}
	run := 0 // glyphs drawn at the X of the previous one (a font without widths)
	for j, t := range texts {
		if j > 0 {
			prev := texts[j-1]
			switch {
			case math.Abs(t.Y-prev.Y) > rowTolerance:
				flush()
				run = 0
			case t.X == prev.X:
				run++
			case t.X < prev.X:
				line.WriteString("   ")
				run = 0
			default:
				line.WriteString(glyphGap(prev, t, run))
				run = 0
			}
		}
		line.WriteString(t.S)
	}
	flush()
}

// glyphGap returns the separator to write between two neighbouring glyphs of a line. When the width of prev is unknown,
// it and the run glyphs drawn at the same X before it are taken as half an em wide each.
func glyphGap(prev, cur pdf.Text, run int) string {
	if strings.HasSuffix(prev.S, " ") || strings.HasPrefix(cur.S, " ") {
		return ""
	}
	fs := max(prev.FontSize, 1)
	w, word := prev.W, 0.2*fs
	if w <= 0 {
		w, word = 0.5*fs*float64(run+1), 0.5*fs
	}
	switch gap := cur.X - prev.X - w; {
	case gap > 3*fs:
		return "   "
	case gap > word:
		return " "
	case run >= 3 && prev.W <= 0 && cur.S != "" && !strings.ContainsAny(cur.S[:1], ",.;:)"):
		// A string placed on its own after a run of unknown width: its true gap is unknown, keep the words apart.
		return " "
	}
	return ""
}

func docxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", errors.New("unreadable docx")
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", errors.New("unreadable docx")
		}
		defer rc.Close()
		return docxXMLText(io.LimitReader(rc, maxDocxXML))
	}
	return "", errors.New("unreadable docx")
}

// docxXMLText keeps the text runs (w:t), tabs and line breaks, one line per paragraph (w:p).
func docxXMLText(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var sb strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return sb.String(), nil
		}
		if err != nil {
			return "", errors.New("unreadable docx")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				sb.WriteByte('\t')
			case "br":
				sb.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				sb.WriteByte('\n')
			}
		case xml.CharData:
			if inText {
				sb.Write(t)
			}
		}
	}
}
