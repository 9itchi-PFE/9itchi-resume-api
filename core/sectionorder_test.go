package core

import (
	"testing"

	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
)

func TestInferSectionOrder(t *testing.T) {
	text := "Ines Mansour\nPROFILE\nEngineer with skills in Go.\nPROFESSIONAL EXPERIENCE\nIngenico 07/2024\n" +
		"PROJECTS\nCoachify\nFormation\nENIS 2024\nCERTIFICATIONS AND PUBLICATIONS\nAZ-900\nSKILLS\nGo\nAwards\n1st\n" +
		"Extracurricular activities\nNASA\nLANGUAGES\nEnglish\nSKILLS\n"
	c := api.ResumeContent{Extra: []api.ExtraSection{{Title: "Extracurricular Activities"}, {Title: "Awards"}, {Title: "Publications"}}}
	assert.Equal(t, []string{"summary", "experience", "projects", "education", "certifications", "skills", "extra:1", "extra:0", "languages"},
		InferSectionOrder(text, c))
}

func TestInferSectionOrderArabicAndFrench(t *testing.T) {
	assert.Equal(t, []string{"skills", "education"}, InferSectionOrder("المهارات\nGo\nÉtudes\nENIS", api.ResumeContent{}))
	assert.Equal(t, []string{"experience", "languages"}, InferSectionOrder("Expériences professionnelles :\nLangues", api.ResumeContent{}))
}

func TestInferSectionOrderNeedsTwoHeadings(t *testing.T) {
	assert.Nil(t, InferSectionOrder("Ines\nSKILLS\nGo", api.ResumeContent{}))
	// Long lines, dates and e-mails are content, not headings.
	assert.Nil(t, InferSectionOrder("I gained experience with many skills over the years\nEducation 2021\nskills@x.tn", api.ResumeContent{}))
}
