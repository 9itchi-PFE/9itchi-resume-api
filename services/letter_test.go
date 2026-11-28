package services

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLetterSvc() (*letterService, *mocks.ResumeRepo) {
	r := &mocks.ResumeRepo{}
	s := NewLetterService(r).(*letterService)
	// 23:30 UTC on 30 November is already 1 December in Tunis.
	s.now = func() time.Time { return time.Date(2026, 11, 30, 23, 30, 0, 0, time.UTC) }
	return s, r
}

func TestLetterPDFWithResume(t *testing.T) {
	s, r := newLetterSvc()
	r.GetByIDFn = func(_ context.Context, id string) (*db.Resume, error) {
		assert.Equal(t, rid, id)
		return &db.Resume{ID: id, Content: []byte(`{"contact":{"name":"Ines Mansour","email":"cv@mail.tn","phone":"+216 20 000 000","location":"Sfax"},"style":{"accent":"#355C7D","body_font":"lato"}}`)}, nil
	}
	data, apiErr := s.LetterPDF(context.Background(), api.LetterRequest{ResumeID: rid, Language: "fr", Subject: "Stage PFE", Body: "Madame, Monsieur,\n\nJe vous écris.", SenderEmail: "ines@gmail.com"})
	require.Nil(t, apiErr)
	assert.True(t, bytes.HasPrefix(data, []byte("%PDF-")))
	text, err := core.ExtractText("pdf", data)
	require.NoError(t, err)
	for _, want := range []string{"Ines Mansour", "ines@gmail.com", "Sfax", "1 décembre 2026", "Objet", "Stage PFE", "Je vous écris."} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "cv@mail.tn", "the address the letter is sent from replaces the CV's")
	assert.NotContains(t, text, core.BrandLead, "the letter is not stamped")
}

func TestLetterPDFWithoutResume(t *testing.T) {
	s, r := newLetterSvc()
	data, apiErr := s.LetterPDF(context.Background(), api.LetterRequest{Language: "en", Body: "Dear team"})
	require.Nil(t, apiErr)
	text, err := core.ExtractText("pdf", data)
	require.NoError(t, err)
	assert.Contains(t, text, "Dear team")
	assert.Contains(t, text, "1 December 2026")

	// A CV deleted meanwhile drops the sender block, no 404.
	r.GetByIDFn = func(context.Context, string) (*db.Resume, error) { return nil, repositories.ErrNotFound }
	_, apiErr = s.LetterPDF(context.Background(), api.LetterRequest{ResumeID: rid, Language: "en", Body: "Dear team"})
	require.Nil(t, apiErr)
}

func TestLetterPDFErrors(t *testing.T) {
	s, r := newLetterSvc()
	_, apiErr := s.LetterPDF(context.Background(), api.LetterRequest{Language: "de", Body: "x"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	_, apiErr = s.LetterPDF(context.Background(), api.LetterRequest{ResumeID: "nope", Language: "fr", Body: "x"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	r.GetByIDFn = func(context.Context, string) (*db.Resume, error) { return nil, errors.New("db down") }
	_, apiErr = s.LetterPDF(context.Background(), api.LetterRequest{ResumeID: rid, Language: "fr", Body: "x"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestCleanStemAndCVStem(t *testing.T) {
	assert.Equal(t, "CV Data", cleanStem(`C:\docs\CV  Data.PDF`, "x"))
	assert.Equal(t, "Mon_CV", cleanStem("Mon__CV.docx", "x"))
	assert.Equal(t, "سيرة ذاتية", cleanStem("سيرة ذاتية", "x"))
	assert.Equal(t, "CV (v2)", cleanStem(" .CV (v2)- ", "x"))
	assert.Equal(t, "x", cleanStem(" ._- ", "x"))
	assert.Len(t, []rune(cleanStem(string(bytes.Repeat([]byte("é"), 100)), "x")), 80)
	assert.Equal(t, "My CV", cvStem("My CV", "Ines Mansour"))
	assert.Equal(t, "ines_mansour", cvStem("", "Inès Mansour"))
	assert.Equal(t, "CV", cvStem("", "محمد"))
	assert.Equal(t, ".pdf", originalExt("cv.PDF", ""))
	assert.Equal(t, ".docx", originalExt("upload", contentTypeDOCX))
}
