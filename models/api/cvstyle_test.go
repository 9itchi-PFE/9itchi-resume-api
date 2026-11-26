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
