package api

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/assert"
)

func TestCVStyleEntryFieldsValidation(t *testing.T) {
	for name, tc := range map[string]struct {
		style CVStyle
		ok    bool
	}{
		"unset":                  {CVStyle{}, true},
		"stacked, title first":   {CVStyle{EntryLayout: "stacked", EntryOrder: "title_first"}, true},
		"inline, subtitle first": {CVStyle{EntryLayout: "inline", EntryOrder: "subtitle_first"}, true},
		"only a layout":          {CVStyle{EntryLayout: "inline"}, true},
		"bad layout":             {CVStyle{EntryLayout: "diagonal"}, false},
		"bad order":              {CVStyle{EntryOrder: "diagonal"}, false},
		"order in layout":        {CVStyle{EntryLayout: "title_first"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.ok, binding.Validator.ValidateStruct(&tc.style) == nil)
		})
	}
}

func TestDefaultCVStyleIsValid(t *testing.T) {
	s := DefaultCVStyle()
	assert.NoError(t, binding.Validator.ValidateStruct(s))
	assert.True(t, s.Valid())
	assert.True(t, s.Accents("dates"))
	assert.False(t, s.Accents("entry_title"))
	assert.NotSame(t, DefaultCVStyle().NameDelta, s.NameDelta, "each CV gets its own copy")
}

func TestCVStyleLookFieldsValidation(t *testing.T) {
	gap, far := 4.0, 13.0
	for name, tc := range map[string]struct {
		style CVStyle
		ok    bool
	}{
		"all set":             {CVStyle{EducationOrder: "subtitle_first", HeadingRule: "thick", SectionGap: &gap, SkillsStyle: "bar", LanguagesStyle: "inline"}, true},
		"thin, comma, rows":   {CVStyle{HeadingRule: "thin", SkillsStyle: "comma", LanguagesStyle: "rows"}, true},
		"bad education order": {CVStyle{EducationOrder: "school_first"}, false},
		"bad heading rule":    {CVStyle{HeadingRule: "dotted"}, false},
		"gap too big":         {CVStyle{SectionGap: &far}, false},
		"bad skills style":    {CVStyle{SkillsStyle: "tags"}, false},
		"bad languages style": {CVStyle{LanguagesStyle: "grid"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.ok, binding.Validator.ValidateStruct(&tc.style) == nil)
		})
	}
}
