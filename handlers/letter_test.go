package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"

	"github.com/stretchr/testify/assert"
)

type stubLetterSvc struct {
	got api.LetterRequest
	err *models.ApiError
}

func (s *stubLetterSvc) LetterPDF(_ context.Context, req api.LetterRequest) ([]byte, *models.ApiError) {
	s.got = req
	if s.err != nil {
		return nil, s.err
	}
	return []byte("%PDF-letter"), nil
}

func TestLetterPDFHandler(t *testing.T) {
	svc := &stubLetterSvc{}
	r := newRouter(LetterPDF(svc), http.MethodPost, "/internal/letters/pdf")
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/internal/letters/pdf", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	w := post(`{"resume_id":"11111111-1111-1111-1111-111111111111","language":"fr","subject":"Stage PFE","body":"Bonjour","sender_email":"a@b.tn"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="letter.pdf"; filename*=UTF-8''letter.pdf`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "%PDF-letter", w.Body.String())
	assert.Equal(t, "Stage PFE", svc.got.Subject)
	assert.Equal(t, "a@b.tn", svc.got.SenderEmail)

	assert.Equal(t, http.StatusOK, post(`{"language":"ar","body":"مرحبا"}`).Code, "resume_id, subject and sender are optional")

	for _, body := range []string{
		`{"language":"de","body":"x"}`,
		`{"language":"fr"}`,
		`{"language":"fr","body":"x","resume_id":"nope"}`,
		`{"language":"fr","body":"x","sender_email":"not-an-email"}`,
		`{"language":"fr","body":"x","subject":"` + strings.Repeat("a", 201) + `"}`,
		`{"language":"fr","body":"` + strings.Repeat("a", 10001) + `"}`,
		`{`,
	} {
		w := post(body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Contains(t, w.Body.String(), `"error"`)
	}

	svc.err = &models.ApiError{Code: http.StatusInternalServerError, Error: assert.AnError}
	assert.Equal(t, http.StatusInternalServerError, post(`{"language":"en","body":"Hello"}`).Code)
}

func TestAttachmentDisposition(t *testing.T) {
	assert.Equal(t, `attachment; filename="Mon CV Ines.pdf"; filename*=UTF-8''Mon%20CV%20In%C3%A8s.pdf`, attachmentDisposition("Mon CV Inès.pdf"))
	assert.Equal(t, `attachment; filename="CV.pdf"; filename*=UTF-8''%D8%B3%D9%8A%D8%B1%D8%A9.pdf`, attachmentDisposition("سيرة.pdf"),
		"an Arabic title keeps its name in filename*, the ASCII fallback is CV")
	assert.Equal(t, `attachment; filename="a_b_c.pdf"; filename*=UTF-8''a%22b%5Cc.pdf`, attachmentDisposition(`a"b\c.pdf`))
}
