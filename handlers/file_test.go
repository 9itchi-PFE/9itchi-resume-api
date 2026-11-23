package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/stretchr/testify/assert"
)

type stubFileSvc struct {
	services.FileService
	download func() (*api.FileContent, *models.ApiError)
	del      func() *models.ApiError
}

func (s *stubFileSvc) DownloadFile(context.Context, string, string, string) (*api.FileContent, *models.ApiError) {
	return s.download()
}

func (s *stubFileSvc) DeleteFile(context.Context, string, string, string) *models.ApiError {
	return s.del()
}

func TestDownloadFileHandler(t *testing.T) {
	svc := &stubFileSvc{download: func() (*api.FileContent, *models.ApiError) {
		return &api.FileContent{Filename: "my cv.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1")}, nil
	}}
	r := newRouter(DownloadFile(svc), http.MethodGet, "/resumes/:id/files/:fileId")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/resumes/a/files/b", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="my cv.pdf"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "%PDF-1", w.Body.String())

	svc.download = func() (*api.FileContent, *models.ApiError) {
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: assert.AnError}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/resumes/a/files/b", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`)
}

func TestDeleteFileHandler(t *testing.T) {
	svc := &stubFileSvc{del: func() *models.ApiError { return nil }}
	r := newRouter(DeleteFile(svc), http.MethodDelete, "/resumes/:id/files/:fileId")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/resumes/a/files/b", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)

	svc.del = func() *models.ApiError { return &models.ApiError{Code: http.StatusNotFound, Error: assert.AnError} }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/resumes/a/files/b", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
