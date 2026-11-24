package core

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
		rows, err := p.GetTextByRow()
		if err != nil {
			continue
		}
		for _, row := range rows {
			for j, w := range row.Content {
				if j > 0 {
					sb.WriteByte(' ')
				}
				sb.WriteString(w.S)
			}
			sb.WriteByte('\n')
		}
		if sb.Len() > MaxExtractedText*4 {
			break
		}
	}
	return sb.String(), nil
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
