package api

import (
	"regexp"
	"strconv"
	"strings"
)

// CVStyle is the optional look of a resume. A zero value means "the template's default", so documents that never
// opened the style panel are unchanged. The client preview and the PDF/DOCX exporters read the same fields.
type CVStyle struct {
	BaseSize     float64  `json:"base_size,omitempty" binding:"omitempty,min=7,max=14"`    // body text, pt
	NameDelta    *float64 `json:"name_delta,omitempty" binding:"omitempty,min=-3,max=20"`  // added to the base size, pt
	TitleDelta   *float64 `json:"title_delta,omitempty" binding:"omitempty,min=-3,max=20"` // professional title
	HeadingDelta *float64 `json:"heading_delta,omitempty" binding:"omitempty,min=-3,max=20"`
	EntryDelta   *float64 `json:"entry_delta,omitempty" binding:"omitempty,min=-3,max=10"` // entry header lines
	LineHeight   float64  `json:"line_height,omitempty" binding:"omitempty,min=1,max=2"`   // multiplier
	Gap          *float64 `json:"gap,omitempty" binding:"omitempty,min=0,max=10"`          // between entries, mm
	MarginX      float64  `json:"margin_x,omitempty" binding:"omitempty,min=5,max=30"`     // left and right, mm
	MarginY      float64  `json:"margin_y,omitempty" binding:"omitempty,min=5,max=30"`     // top and bottom, mm
	NameFont     string   `json:"name_font,omitempty" binding:"omitempty,oneof=sans serif"`
	BodyFont     string   `json:"body_font,omitempty" binding:"omitempty,oneof=sourcesans lato carlito tinos"` // applies to all body text
	HeadingCase  string   `json:"heading_case,omitempty" binding:"omitempty,oneof=capitalize uppercase"`
	DatePosition string   `json:"date_position,omitempty" binding:"omitempty,oneof=right below"`
	LocationPos   string `json:"location_position,omitempty" binding:"omitempty,oneof=line date"`
	MetaOrder     string `json:"meta_order,omitempty" binding:"omitempty,oneof=date_first location_first"`
	PhotoPos      string `json:"photo_position,omitempty" binding:"omitempty,oneof=left right"`
	ListStyle     string `json:"list_style,omitempty" binding:"omitempty,oneof=bullet hyphen"`
	IndentBody    *bool  `json:"indent_body,omitempty"`
	SubtitleStyle string `json:"subtitle_style,omitempty" binding:"omitempty,oneof=normal bold italic"`
	DateStyle     string `json:"date_style,omitempty" binding:"omitempty,oneof=normal bold italic"`
	Align        string   `json:"align,omitempty" binding:"omitempty,oneof=left center"`        // header text alignment
	Separator    string   `json:"separator,omitempty" binding:"omitempty,oneof=bullet bar"`     // between contact details
	ColorMode    string   `json:"color_mode,omitempty" binding:"omitempty,oneof=single multi"`  // single: accent on black text, white page
	Accent       string   `json:"accent,omitempty" binding:"omitempty,max=7"`
	Text         string   `json:"text,omitempty" binding:"omitempty,max=7"`
	Background   string   `json:"background,omitempty" binding:"omitempty,max=7"`
	AccentOn     []string `json:"accent_on,omitempty" binding:"max=8,dive,oneof=name title headings rules dates entry_title links"`
	ShowPhoto    *bool    `json:"show_photo,omitempty"`
}

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Valid reports whether every color is a #RRGGBB hex string (empty means default).
func (s *CVStyle) Valid() bool {
	if s == nil {
		return true
	}
	for _, c := range []string{s.Accent, s.Text, s.Background} {
		if c != "" && !hexColor.MatchString(c) {
			return false
		}
	}
	return true
}

// defaultAccentTargets are what the accent color paints when AccentOn is empty: the template's own accent spots.
var defaultAccentTargets = []string{"title", "headings", "rules", "entry_title", "links"}

// Accents reports whether the accent color applies to target.
func (s *CVStyle) Accents(target string) bool {
	if s == nil || s.Accent == "" {
		return false
	}
	list := s.AccentOn
	if len(list) == 0 {
		list = defaultAccentTargets
	}
	for _, t := range list {
		if t == target {
			return true
		}
	}
	return false
}

// SectionKeys are the sections of a resume, in the default order. Extra sections follow as "extra:N".
var SectionKeys = []string{"summary", "education", "experience", "projects", "skills", "certifications", "languages"}

// SectionOrder returns every visible section once, in the order the user chose: the keys of content.SectionOrder
// first (unknown or repeated ones dropped), then the remaining sections in the default order. Hidden sections are left out.
func (c ResumeContent) SectionOrder() []string {
	hidden := map[string]bool{}
	for _, k := range c.HiddenSections {
		hidden[k] = true
	}
	all := append([]string(nil), SectionKeys...)
	for i := range c.Extra {
		all = append(all, "extra:"+strconv.Itoa(i))
	}
	known := map[string]bool{}
	for _, k := range all {
		known[k] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(all))
	for _, k := range c.SectionOrderKeys {
		if known[k] && !seen[k] && !hidden[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, k := range all {
		if !seen[k] && !hidden[k] {
			out = append(out, k)
		}
	}
	return out
}

// ValidSectionOrder reports whether every key names a section; it is checked on save.
func ValidSectionOrder(keys []string) bool {
	for _, k := range keys {
		if strings.HasPrefix(k, "extra:") {
			if n, err := strconv.Atoi(k[6:]); err != nil || n < 0 || n >= 10 {
				return false
			}
			continue
		}
		ok := false
		for _, s := range SectionKeys {
			ok = ok || s == k
		}
		if !ok {
			return false
		}
	}
	return true
}
