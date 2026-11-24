package core

import (
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
)

func TestLocationBesideDate(t *testing.T) {
	c := api.ResumeContent{
		Style:      &api.CVStyle{LocationPos: "date", MetaOrder: "location_first"},
		Experience: []api.Experience{{Company: "**Oliot**", Start: "02/2023", Location: "Tunis"}, {Company: "B", Start: "01/2022", End: "02/2022"}},
		Projects:   []api.Project{{Name: "P", Start: "12/2022", Location: "Sfax"}},
	}
	v := visibleEntries(c)
	assert.Equal(t, "Oliot", v.Experience[0].Company)
	assert.Equal(t, "Tunis · 02/2023", v.Experience[0].Start)
	assert.Empty(t, v.Experience[0].Location)
	assert.Equal(t, "01/2022", v.Experience[1].Start)
	assert.Equal(t, "Sfax · 12/2022", v.Projects[0].Year)
}
