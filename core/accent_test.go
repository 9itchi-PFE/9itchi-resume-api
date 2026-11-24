package core

import (
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
)

func TestAccentTargets(t *testing.T) {
	s := &api.CVStyle{Accent: "#ff0000", AccentOn: []string{"dates"}}
	l := newLook("classic", s)
	assert.Equal(t, rgb{255, 0, 0}, l.col("dates", cvInk))
	assert.Equal(t, cvInk, l.col("location", cvInk))
	assert.Equal(t, cvInk, l.col("headings", cvBlue))
	assert.Equal(t, cvMuted, l.col("location", cvMuted))
	assert.Equal(t, cvNavy, l.colKeep("rules", cvNavy))

	none := newLook("classic", &api.CVStyle{Accent: "#ff0000", AccentOn: []string{}})
	assert.Equal(t, cvInk, none.col("dates", cvInk))
}

func TestMarkedLocation(t *testing.T) {
	c := api.ResumeContent{
		Style:      &api.CVStyle{LocationPos: "date"},
		Experience: []api.Experience{{Company: "A", Start: "02/2023", Location: "Tunis"}},
	}
	v := visibleEntriesMark(c, true)
	assert.Equal(t, "02/2023 · "+locMark+"Tunis"+locMark, v.Experience[0].Start)
	assert.Equal(t, []metaPart{{"02/2023 · ", false}, {"Tunis", true}}, metaParts(v.Experience[0].Start))
}
