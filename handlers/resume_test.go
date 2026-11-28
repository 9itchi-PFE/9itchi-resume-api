package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubResumeSvc embeds the interface; tests set only the funcs they call.
type stubResumeSvc struct {
	atsReq api.ATSCheckRequest
	services.ResumeService
	upload func(ctx context.Context, userID, filename string, size int64, r io.Reader) (*api.UploadResumeResponse, *models.ApiError)
	create func(req api.CreateResumeRequest) (*api.ResumeResponse, *models.ApiError)
	export func(req api.ExportResumeRequest) (*api.FileResponse, *models.ApiError)
	pdf    func() (*api.FileContent, *models.ApiError)
}

func (s *stubResumeSvc) UploadResume(ctx context.Context, userID, filename string, size int64, r io.Reader) (*api.UploadResumeResponse, *models.ApiError) {
	return s.upload(ctx, userID, filename, size, r)
}

func (s *stubResumeSvc) CreateResume(_ context.Context, _ string, req api.CreateResumeRequest) (*api.ResumeResponse, *models.ApiError) {
	return s.create(req)
}

func (s *stubResumeSvc) ExportResume(_ context.Context, _, _ string, req api.ExportResumeRequest) (*api.FileResponse, *models.ApiError) {
	return s.export(req)
}

func (s *stubResumeSvc) InternalResumePDF(context.Context, string) (*api.FileContent, *models.ApiError) {
	return s.pdf()
}

func (s *stubResumeSvc) InternalResumeOriginal(context.Context, string) (*api.FileContent, *models.ApiError) {
	return s.pdf()
}

func (s *stubResumeSvc) ATSCheck(_ context.Context, _, _ string, req api.ATSCheckRequest) (*api.ATSCheckResponse, *models.ApiError) {
	s.atsReq = req
	return &api.ATSCheckResponse{Passed: 23, Total: 24, ToFix: 1, Rules: []api.ATSRuleResult{}}, nil
}

func newRouter(h gin.HandlerFunc, method, path string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "u1") })
	r.Handle(method, path, h)
	return r
}

func multipartBody(t *testing.T, field, filename string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

func TestUploadResumeHandler(t *testing.T) {
	var gotName string
	var gotData []byte
	svc := &stubResumeSvc{upload: func(_ context.Context, userID, filename string, _ int64, r io.Reader) (*api.UploadResumeResponse, *models.ApiError) {
		gotName = filename
		gotData, _ = io.ReadAll(r)
		assert.Equal(t, "u1", userID)
		return &api.UploadResumeResponse{Resume: api.ResumeResponse{ID: "r1"}, File: api.FileResponse{ID: "f1"}}, nil
	}}
	r := newRouter(UploadResume(svc), http.MethodPost, "/resumes/upload")

	t.Run("ok", func(t *testing.T) {
		body, ct := multipartBody(t, "file", "cv.pdf", []byte("%PDF-1.4 data"))
		req := httptest.NewRequest(http.MethodPost, "/resumes/upload", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, "cv.pdf", gotName)
		assert.Equal(t, []byte("%PDF-1.4 data"), gotData)
		assert.Contains(t, w.Body.String(), `"resume"`)
		assert.Contains(t, w.Body.String(), `"file"`)
	})

	t.Run("missing field", func(t *testing.T) {
		body, ct := multipartBody(t, "other", "cv.pdf", []byte("x"))
		req := httptest.NewRequest(http.MethodPost, "/resumes/upload", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), `"error"`)
	})

	t.Run("body over limit is rejected before the service", func(t *testing.T) {
		big := bytes.Repeat([]byte("a"), api.MaxUploadBytes+multipartOverhead+10)
		body, ct := multipartBody(t, "file", "cv.pdf", big)
		req := httptest.NewRequest(http.MethodPost, "/resumes/upload", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc2 := &stubResumeSvc{upload: func(context.Context, string, string, int64, io.Reader) (*api.UploadResumeResponse, *models.ApiError) {
			return nil, &models.ApiError{Code: http.StatusBadRequest, Error: assert.AnError}
		}}
		r2 := newRouter(UploadResume(svc2), http.MethodPost, "/resumes/upload")
		body, ct := multipartBody(t, "file", "cv.pdf", []byte("x"))
		req := httptest.NewRequest(http.MethodPost, "/resumes/upload", body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r2.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestCreateResumeHandlerBinding(t *testing.T) {
	svc := &stubResumeSvc{create: func(req api.CreateResumeRequest) (*api.ResumeResponse, *models.ApiError) {
		return &api.ResumeResponse{ID: "r1", Title: req.Title}, nil
	}}
	r := newRouter(CreateResume(svc), http.MethodPost, "/resumes")

	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"ok":             {`{"title":"CV","content":{"summary":"hi","skills":["Go"]}}`, http.StatusCreated},
		"missing title":  {`{"content":{}}`, http.StatusBadRequest},
		"title too long": {`{"title":"` + strings.Repeat("a", 201) + `"}`, http.StatusBadRequest},
		"bad template":   {`{"title":"CV","template_id":"nope"}`, http.StatusBadRequest},
		"nested too big": {`{"title":"CV","content":{"contact":{"name":"` + strings.Repeat("a", 201) + `"}}}`, http.StatusBadRequest},
		"bad json":       {`{`, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/resumes", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

func TestExportHandler(t *testing.T) {
	svc := &stubResumeSvc{export: func(req api.ExportResumeRequest) (*api.FileResponse, *models.ApiError) {
		if req.Format != "pdf" {
			return nil, &models.ApiError{Code: http.StatusBadRequest, Error: assert.AnError}
		}
		return &api.FileResponse{ID: "f1", Kind: "export"}, nil
	}}
	r := newRouter(ExportResume(svc), http.MethodPost, "/resumes/:id/export")

	req := httptest.NewRequest(http.MethodPost, "/resumes/x/export", strings.NewReader(`{"format":"pdf"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"f1"`)

	req = httptest.NewRequest(http.MethodPost, "/resumes/x/export", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInternalResumePDFHandler(t *testing.T) {
	svc := &stubResumeSvc{pdf: func() (*api.FileContent, *models.ApiError) {
		return &api.FileContent{Filename: "cv.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1")}, nil
	}}
	r := newRouter(InternalResumePDF(svc), http.MethodGet, "/internal/resumes/:id/pdf")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/resumes/x/pdf", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="cv.pdf"; filename*=UTF-8''cv.pdf`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "%PDF-1", w.Body.String())

	svc.pdf = func() (*api.FileContent, *models.ApiError) {
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: assert.AnError}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/resumes/x/pdf", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestATSCheckHandler(t *testing.T) {
	svc := &stubResumeSvc{}
	r := newRouter(ATSCheck(svc), http.MethodPost, "/resumes/:id/ats-check")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/resumes/x/ats-check", nil))
	assert.Equal(t, http.StatusOK, w.Code, "the body is optional")
	assert.Contains(t, w.Body.String(), `"to_fix":1`)

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/resumes/x/ats-check", strings.NewReader(`{"target_language":"en"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "en", svc.atsReq.TargetLanguage)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/resumes/x/ats-check", strings.NewReader(`{"target_language":"de"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInternalResumeOriginalHandler(t *testing.T) {
	svc := &stubResumeSvc{pdf: func() (*api.FileContent, *models.ApiError) {
		return &api.FileContent{Filename: "mon cv.docx", ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Data: []byte("PK")}, nil
	}}
	r := newRouter(InternalResumeOriginal(svc), http.MethodGet, "/internal/resumes/:id/original")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/resumes/x/original", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="mon cv.docx"; filename*=UTF-8''mon%20cv.docx`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "PK", w.Body.String())

	svc.pdf = func() (*api.FileContent, *models.ApiError) {
		return nil, &models.ApiError{Code: http.StatusNotFound, Error: assert.AnError}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/resumes/x/original", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
