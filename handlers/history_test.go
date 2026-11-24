package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/stretchr/testify/assert"
)

type historySvc struct {
	services.ResumeService
	label     string
	photo     []byte
	photoErr  *models.ApiError
	dupTitle  string
	restored  string
	deleted   bool
	getPhoto  *api.FileContent
	versionID string
}

func (s *historySvc) CreateVersion(_ context.Context, _, _ string, req api.CreateVersionRequest) (*api.VersionResponse, *models.ApiError) {
	s.label = req.Label
	return &api.VersionResponse{ID: "v", Label: req.Label}, nil
}

func (s *historySvc) GetVersion(_ context.Context, _, _, versionID string) (*api.VersionContentResponse, *models.ApiError) {
	s.versionID = versionID
	return &api.VersionContentResponse{VersionResponse: api.VersionResponse{ID: versionID}}, nil
}

func (s *historySvc) RestoreVersion(_ context.Context, _, _, versionID string) (*api.ResumeResponse, *models.ApiError) {
	s.restored = versionID
	return &api.ResumeResponse{ID: "r"}, nil
}

func (s *historySvc) DuplicateResume(_ context.Context, _, _ string, req api.DuplicateResumeRequest) (*api.ResumeResponse, *models.ApiError) {
	s.dupTitle = req.Title
	return &api.ResumeResponse{ID: "copy", Title: req.Title}, nil
}

func (s *historySvc) UploadPhoto(_ context.Context, _, _ string, r io.Reader) *models.ApiError {
	s.photo, _ = io.ReadAll(r)
	return s.photoErr
}

func (s *historySvc) GetPhoto(context.Context, string, string) (*api.FileContent, *models.ApiError) {
	if s.getPhoto == nil {
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: assert.AnError}
	}
	return s.getPhoto, nil
}

func (s *historySvc) DeletePhoto(context.Context, string, string) *models.ApiError {
	s.deleted = true
	return nil
}

func do(r http.Handler, method, path, body, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateVersionLabel(t *testing.T) {
	svc := &historySvc{}
	r := newRouter(CreateVersion(svc), http.MethodPost, "/resumes/:id/versions")
	w := do(r, http.MethodPost, "/resumes/x/versions", `{"label":"template_changed"}`, "application/json")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "template_changed", svc.label)

	w = do(r, http.MethodPost, "/resumes/x/versions", "", "")
	assert.Equal(t, http.StatusCreated, w.Code, "the body is optional")

	w = do(r, http.MethodPost, "/resumes/x/versions", `{"label":"`+strings.Repeat("a", 101)+`"}`, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w = do(r, http.MethodPost, "/resumes/x/versions", `{"label":"<script>é"}`, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code, "labels are plain keys")
}

func TestVersionRestoreDuplicateHandlers(t *testing.T) {
	svc := &historySvc{}
	w := do(newRouter(GetVersion(svc), http.MethodGet, "/resumes/:id/versions/:versionId"), http.MethodGet, "/resumes/x/versions/v9", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v9", svc.versionID)

	w = do(newRouter(RestoreVersion(svc), http.MethodPost, "/resumes/:id/versions/:versionId/restore"), http.MethodPost, "/resumes/x/versions/v3/restore", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v3", svc.restored)

	dup := newRouter(DuplicateResume(svc), http.MethodPost, "/resumes/:id/duplicate")
	w = do(dup, http.MethodPost, "/resumes/x/duplicate", `{"title":"Data"}`, "application/json")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Data", svc.dupTitle)
	w = do(dup, http.MethodPost, "/resumes/x/duplicate", `{}`, "application/json")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPhotoHandlers(t *testing.T) {
	svc := &historySvc{}
	up := newRouter(UploadPhoto(svc), http.MethodPut, "/resumes/:id/photo")
	body, ct := multipartBody(t, "file", "me.jpg", []byte{0xFF, 0xD8, 0xFF, 1})
	w := do(up, http.MethodPut, "/resumes/x/photo", body.String(), ct)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, []byte{0xFF, 0xD8, 0xFF, 1}, svc.photo)

	body, ct = multipartBody(t, "other", "me.jpg", []byte{1})
	w = do(up, http.MethodPut, "/resumes/x/photo", body.String(), ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body, ct = multipartBody(t, "file", "big.jpg", bytes.Repeat([]byte{1}, services.MaxPhotoBytes+multipartOverhead+10))
	w = do(up, http.MethodPut, "/resumes/x/photo", body.String(), ct)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	svc.photoErr = &models.ApiError{Code: http.StatusBadRequest, Error: assert.AnError}
	body, ct = multipartBody(t, "file", "me.gif", []byte("GIF89a"))
	w = do(up, http.MethodPut, "/resumes/x/photo", body.String(), ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	get := newRouter(GetPhoto(svc), http.MethodGet, "/resumes/:id/photo")
	w = do(get, http.MethodGet, "/resumes/x/photo", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.getPhoto = &api.FileContent{ContentType: "image/png", Data: []byte("png")}
	w = do(get, http.MethodGet, "/resumes/x/photo", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))

	w = do(newRouter(DeletePhoto(svc), http.MethodDelete, "/resumes/:id/photo"), http.MethodDelete, "/resumes/x/photo", "", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, svc.deleted)
}
