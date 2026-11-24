package core

import (
	"strconv"
	"strings"

	"9itchi-resume-api/models/api"
)

// look is the resolved style of one export: the template's defaults with the user's CVStyle laid over them.
type look struct {
	base, name, title, head, entry float64 // pt
	lineH                          float64 // multiplier
	gap                            float64 // mm between entries; < 0 keeps the template's own spacing
	mx, my                         float64 // mm
	nameSerif                      bool
	upper                          bool
	dateBelow, center              bool
	photoRight                     bool
	headerLayout                   string
	headerValign                   string
	layoutIsClassic                bool
	sep                            string
	showPhoto                      bool
	listChar                       string
	indent                         float64 // mm of the description text
	subStyle, dateStyle            string  // "" keeps the template's own style, "N" is explicitly normal
	style                          *api.CVStyle
	accent, text, bg               *rgb
}

type layoutLook struct {
	base, name, title, head, mx, my float64
	upper, nameSerif, center        bool
}

var layoutLooks = map[string]layoutLook{
	"classic": {base: 10.5, name: 23, title: 12, head: 13, mx: 14, my: 12, upper: true},
	"modern":  {base: 10, name: 22, title: 10.5, head: 13, mx: 9, my: 12, nameSerif: true},
	"compact": {base: 10, name: 17, title: 10, head: 11, mx: 15, my: 12, upper: true, center: true},
}

func hexRGB(s string) *rgb {
	if len(s) != 7 || s[0] != '#' {
		return nil
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return nil
	}
	return &rgb{int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff)}
}

func delta(p *float64, def float64) float64 {
	if p != nil {
		return *p
	}
	return def
}

func fontStyleOf(s string) string {
	switch s {
	case "bold":
		return "B"
	case "italic":
		return "I"
	case "normal":
		return "N"
	}
	return ""
}

// pick returns the user's choice for a text role, or def when the user kept the template's style.
func pick(chosen, def string) string {
	switch chosen {
	case "":
		return def
	case "N":
		return ""
	}
	return chosen
}

func newLook(layout string, s *api.CVStyle) look {
	def, ok := layoutLooks[layout]
	if !ok {
		def = layoutLooks["classic"]
	}
	if s == nil {
		s = &api.CVStyle{}
	}
	l := look{
		base: def.base, lineH: 1.38, gap: -1, mx: def.mx, my: def.my,
		nameSerif: def.nameSerif, upper: def.upper, center: def.center, sep: "bullet", showPhoto: s.ShowPhoto == nil || *s.ShowPhoto,
		style: s,
	}
	if s.BaseSize > 0 {
		l.base = s.BaseSize
	}
	l.name = l.base + delta(s.NameDelta, def.name-def.base)
	l.title = l.base + delta(s.TitleDelta, def.title-def.base)
	l.head = l.base + delta(s.HeadingDelta, def.head-def.base)
	l.entry = l.base + delta(s.EntryDelta, 0)
	if s.LineHeight > 0 {
		l.lineH = s.LineHeight
	}
	if s.Gap != nil {
		l.gap = *s.Gap
	}
	if s.MarginX > 0 {
		l.mx = s.MarginX
	}
	if s.MarginY > 0 {
		l.my = s.MarginY
	}
	switch s.NameFont {
	case "serif":
		l.nameSerif = true
	case "sans":
		l.nameSerif = false
	}
	switch s.HeadingCase {
	case "uppercase":
		l.upper = true
	case "capitalize":
		l.upper = false
	}
	l.dateBelow = s.DatePosition == "below"
	l.photoRight = s.PhotoPos == "right"
	l.headerLayout = s.HeaderLayout
	l.headerValign = s.HeaderValign
	l.layoutIsClassic = layout == "classic"
	l.listChar, l.indent = "•", 4
	if s.ListStyle == "hyphen" {
		l.listChar = "-"
	}
	if s.IndentBody != nil && !*s.IndentBody {
		l.indent = 2.6
	}
	l.subStyle, l.dateStyle = fontStyleOf(s.SubtitleStyle), fontStyleOf(s.DateStyle)
	switch s.Align {
	case "left":
		l.center = false
	case "center":
		l.center = true
	}
	switch {
	case s.Separator == "bar" || s.Separator == "icon":
		l.sep = s.Separator
	case s.Separator == "" && layout == "classic":
		l.sep = "icon"
	}
	l.accent = hexRGB(s.Accent)
	if s.ColorMode == "multi" {
		l.text, l.bg = hexRGB(s.Text), hexRGB(s.Background)
	}
	return l
}

// col is the color of target: the accent when the user picked one for it, else the template's native color.
func (l look) col(target string, native rgb) rgb {
	if l.accent != nil && l.style.Accents(target) {
		return *l.accent
	}
	return native
}

// ink is the body text color.
func (l look) ink(native rgb) rgb {
	if l.text != nil {
		return *l.text
	}
	return native
}

// gapOr is the space between entries in mm: the user's, else the template's.
func (l look) gapOr(def float64) float64 {
	if l.gap >= 0 {
		return l.gap
	}
	return def
}

// heading applies the heading case.
func (l look) heading(s string) string {
	if l.upper {
		return strings.ToUpper(s)
	}
	return s
}

func (l look) separator() string {
	if l.sep == "bar" {
		return " | "
	}
	if l.sep == "icon" {
		return "     "
	}
	return " · "
}

// extraAt returns the extra section an "extra:N" key points at.
func extraAt(c api.ResumeContent, key string) (api.ExtraSection, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(key, "extra:"))
	if !strings.HasPrefix(key, "extra:") || err != nil || n < 0 || n >= len(c.Extra) {
		return api.ExtraSection{}, false
	}
	return c.Extra[n], true
}

// inlineHeader puts the professional title on the name's line; it is the default of the classic header and ignored when the header is centred.
func (l look) inlineHeader() bool {
	return l.headerLayout != "stacked" && !l.center
}
