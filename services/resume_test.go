package services

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	uid = "11111111-1111-1111-1111-111111111111"
	rid = "22222222-2222-2222-2222-222222222222"
	tid = "33333333-3333-3333-3333-333333333333"
)

type fixture struct {
	svc       ResumeService
	resumes   *mocks.ResumeRepo
	templates *mocks.TemplateRepo
	files     *mocks.FileRepo
}

func newFixture() *fixture {
	f := &fixture{resumes: &mocks.ResumeRepo{}, templates: &mocks.TemplateRepo{}, files: &mocks.FileRepo{}}
	f.svc = NewResumeService(f.resumes, f.templates, f.files, mocks.Tx{}, core.DefaultExporters())
	return f
}

func TestGetResumeOwnership(t *testing.T) {
	f := newFixture()
	f.resumes.GetFn = func(_ context.Context, userID, id string) (*db.Resume, error) {
		if userID != uid {
			return nil, repositories.ErrNotFound
		}
		return &db.Resume{ID: id, UserID: userID, Title: "cv", Content: []byte(`{"summary":"hi"}`)}, nil
	}

	got, apiErr := f.svc.GetResume(context.Background(), uid, rid)
	require.Nil(t, apiErr)
	assert.Equal(t, "hi", got.Content.Summary)
	assert.NotNil(t, got.Content.Skills, "nil slices are normalised to []")

	_, apiErr = f.svc.GetResume(context.Background(), "99999999-9999-9999-9999-999999999999", rid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	_, apiErr = f.svc.GetResume(context.Background(), uid, "not-a-uuid")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
}

func TestCreateResumeValidation(t *testing.T) {
	big := api.ResumeContent{Summary: strings.Repeat("x", 100)}
	for i := 0; i < 52; i++ {
		big.Experience = append(big.Experience, api.Experience{Description: []string{strings.Repeat("y", 1000), strings.Repeat("y", 1000), strings.Repeat("y", 1000), strings.Repeat("y", 1000)}})
	}
	tests := []struct {
		name string
		req  api.CreateResumeRequest
		want int
	}{
		{"ok", api.CreateResumeRequest{Title: "My CV"}, 0},
		{"title too long", api.CreateResumeRequest{Title: strings.Repeat("a", 201)}, http.StatusBadRequest},
		{"blank title", api.CreateResumeRequest{Title: "  "}, http.StatusBadRequest},
		{"nul in title", api.CreateResumeRequest{Title: "a\x00b"}, http.StatusBadRequest},
		{"nul in content", api.CreateResumeRequest{Title: "t", Content: api.ResumeContent{Summary: "a\x00b"}}, http.StatusBadRequest},
		{"content too large", api.CreateResumeRequest{Title: "t", Content: big}, http.StatusBadRequest},
		{"bad template id", api.CreateResumeRequest{Title: "t", TemplateID: ptr("zzz")}, http.StatusBadRequest},
		{"unknown template", api.CreateResumeRequest{Title: "t", TemplateID: ptr(tid)}, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			f.resumes.CreateFn = func(_ context.Context, userID, title string, _ *string, content []byte) (*db.Resume, error) {
				return &db.Resume{ID: rid, UserID: userID, Title: title, Content: content}, nil
			}
			_, apiErr := f.svc.CreateResume(context.Background(), uid, tc.req)
			if tc.want == 0 {
				assert.Nil(t, apiErr)
				return
			}
			require.NotNil(t, apiErr)
			assert.Equal(t, tc.want, apiErr.Code)
		})
	}
}

func TestCreateResumeInactiveTemplate(t *testing.T) {
	f := newFixture()
	f.templates.GetFn = func(context.Context, string) (*db.Template, error) { return &db.Template{ID: tid, Active: false}, nil }
	_, apiErr := f.svc.CreateResume(context.Background(), uid, api.CreateResumeRequest{Title: "t", TemplateID: ptr(tid)})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
}

func TestUpdateTemplateSwitchKeepsContent(t *testing.T) {
	f := newFixture()
	f.templates.GetFn = func(context.Context, string) (*db.Template, error) { return &db.Template{ID: tid, Active: true}, nil }
	gotContent := []byte("sentinel")
	f.resumes.UpdateFn = func(_ context.Context, _, id string, _, _ *string, content []byte) (*db.Resume, error) {
		gotContent = content
		return &db.Resume{ID: id, TemplateID: ptr(tid)}, nil
	}
	_, apiErr := f.svc.UpdateResume(context.Background(), uid, rid, api.UpdateResumeRequest{TemplateID: ptr(tid)})
	require.Nil(t, apiErr)
	assert.Nil(t, gotContent, "content must not be sent when only the template changes")
}

func TestUpdateResumeNotOwned(t *testing.T) {
	f := newFixture()
	f.resumes.UpdateFn = func(context.Context, string, string, *string, *string, []byte) (*db.Resume, error) {
		return nil, repositories.ErrNotFound
	}
	_, apiErr := f.svc.UpdateResume(context.Background(), uid, rid, api.UpdateResumeRequest{Title: ptr("x")})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}

func TestListResumesPaging(t *testing.T) {
	f := newFixture()
	var limit, offset int
	f.resumes.ListFn = func(_ context.Context, _ string, l, o int) ([]db.ResumeSummary, int, error) {
		limit, offset = l, o
		return []db.ResumeSummary{{ID: rid}}, 41, nil
	}
	got, apiErr := f.svc.ListResumes(context.Background(), uid, 3, 1000)
	require.Nil(t, apiErr)
	assert.Equal(t, 100, limit)
	assert.Equal(t, 200, offset)
	assert.Equal(t, 41, got.Total)
	assert.Equal(t, 3, got.Page)

	_, _ = f.svc.ListResumes(context.Background(), uid, 0, 0)
	assert.Equal(t, 20, limit)
	assert.Equal(t, 0, offset)
}

func TestCreateVersionUsesBumpedNumber(t *testing.T) {
	f := newFixture()
	f.resumes.BumpVersionFn = func(context.Context, string, string) (int, []byte, error) { return 3, []byte(`{}`), nil }
	var gotNo int
	f.resumes.InsertVersionFn = func(_ context.Context, id string, no int, _ []byte) (*db.ResumeVersionMeta, error) {
		gotNo = no
		return &db.ResumeVersionMeta{ID: "v", ResumeID: id, VersionNo: no}, nil
	}
	got, apiErr := f.svc.CreateVersion(context.Background(), uid, rid)
	require.Nil(t, apiErr)
	assert.Equal(t, 3, gotNo)
	assert.Equal(t, 3, got.VersionNo)

	f.resumes.BumpVersionFn = func(context.Context, string, string) (int, []byte, error) { return 0, nil, repositories.ErrNotFound }
	_, apiErr = f.svc.CreateVersion(context.Background(), "other", rid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}

func TestListVersionsOwnership(t *testing.T) {
	f := newFixture()
	f.resumes.ExistsFn = func(context.Context, string, string) error { return repositories.ErrNotFound }
	_, apiErr := f.svc.ListVersions(context.Background(), uid, rid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}

func pdfBytes() []byte { return append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 50)...) }

func TestUploadResume(t *testing.T) {
	zip := append([]byte("PK\x03\x04"), bytes.Repeat([]byte("x"), 20)...)
	tests := []struct {
		name     string
		filename string
		data     []byte
		size     int64
		want     int
	}{
		{"pdf ok", "My Resume.pdf", pdfBytes(), 0, 0},
		{"docx ok", "cv.docx", zip, 0, 0},
		{"wrong extension", "cv.txt", pdfBytes(), 0, http.StatusBadRequest},
		{"pdf ext with docx bytes", "cv.pdf", zip, 0, http.StatusBadRequest},
		{"bad magic", "cv.pdf", []byte("not a pdf at all"), 0, http.StatusBadRequest},
		{"declared too large", "cv.pdf", pdfBytes(), api.MaxUploadBytes + 1, http.StatusRequestEntityTooLarge},
		{"actual too large", "cv.pdf", append([]byte("%PDF-"), make([]byte, api.MaxUploadBytes)...), 100, http.StatusRequestEntityTooLarge},
		{"empty", "cv.pdf", nil, 0, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			var file *db.ResumeFile
			var title string
			f.resumes.CreateFn = func(_ context.Context, userID, ti string, _ *string, content []byte) (*db.Resume, error) {
				title = ti
				return &db.Resume{ID: rid, UserID: userID, Title: ti, Content: content}, nil
			}
			f.files.CreateFn = func(_ context.Context, fl *db.ResumeFile) (*db.ResumeFileMeta, error) {
				file = fl
				return &db.ResumeFileMeta{ID: "f", ResumeID: fl.ResumeID, Kind: fl.Kind, Format: fl.Format, SHA256: fl.SHA256}, nil
			}
			size := tc.size
			if size == 0 {
				size = int64(len(tc.data))
			}
			got, apiErr := f.svc.UploadResume(context.Background(), uid, tc.filename, size, bytes.NewReader(tc.data))
			if tc.want != 0 {
				require.NotNil(t, apiErr)
				assert.Equal(t, tc.want, apiErr.Code)
				assert.Nil(t, file, "nothing stored on failure")
				return
			}
			require.Nil(t, apiErr)
			assert.Equal(t, "original", file.Kind)
			assert.Len(t, file.SHA256, 64)
			assert.Equal(t, rid, file.ResumeID)
			assert.NotContains(t, title, ".")
			assert.Equal(t, got.File.Kind, "original")
		})
	}
}

func TestUploadSanitizesFilename(t *testing.T) {
	f := newFixture()
	var file *db.ResumeFile
	f.resumes.CreateFn = func(_ context.Context, userID, ti string, _ *string, c []byte) (*db.Resume, error) {
		return &db.Resume{ID: rid, Title: ti, Content: c}, nil
	}
	f.files.CreateFn = func(_ context.Context, fl *db.ResumeFile) (*db.ResumeFileMeta, error) {
		file = fl
		return &db.ResumeFileMeta{}, nil
	}
	_, apiErr := f.svc.UploadResume(context.Background(), uid, "../../etc/pa\"ss\r\nwd.pdf", 60, bytes.NewReader(pdfBytes()))
	require.Nil(t, apiErr)
	assert.Equal(t, "pa_ss__wd.pdf", file.Filename)
}

func TestExportResume(t *testing.T) {
	newF := func() *fixture {
		f := newFixture()
		f.resumes.GetFn = func(_ context.Context, userID, id string) (*db.Resume, error) {
			if userID != uid {
				return nil, repositories.ErrNotFound
			}
			return &db.Resume{ID: id, Title: "My CV", Content: []byte(`{"summary":"hello"}`)}, nil
		}
		return f
	}

	t.Run("pdf stored as export", func(t *testing.T) {
		f := newF()
		var stored *db.ResumeFile
		f.files.CreateFn = func(_ context.Context, fl *db.ResumeFile) (*db.ResumeFileMeta, error) {
			stored = fl
			return &db.ResumeFileMeta{ID: "f1", Kind: fl.Kind, Format: fl.Format, Filename: fl.Filename}, nil
		}
		got, apiErr := f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "PDF"})
		require.Nil(t, apiErr)
		assert.Equal(t, "export", got.Kind)
		assert.Equal(t, "My CV.pdf", stored.Filename)
		assert.True(t, bytes.HasPrefix(stored.Data, []byte("%PDF-")))
		assert.Equal(t, int64(len(stored.Data)), stored.SizeBytes)
	})
	t.Run("docx unsupported", func(t *testing.T) {
		_, apiErr := newF().svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "docx"})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
		assert.Contains(t, apiErr.Error.Error(), "docx")
	})
	t.Run("unknown format", func(t *testing.T) {
		_, apiErr := newF().svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "odt"})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
	t.Run("other user", func(t *testing.T) {
		_, apiErr := newF().svc.ExportResume(context.Background(), "other", rid, api.ExportResumeRequest{Format: "pdf"})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
}

func TestExportRegistryOnlyPDF(t *testing.T) {
	reg := core.DefaultExporters()
	assert.Len(t, reg, 1)
	assert.Contains(t, reg, "pdf")
}

func TestATSCheckService(t *testing.T) {
	f := newFixture()
	f.resumes.GetFn = func(_ context.Context, userID, id string) (*db.Resume, error) {
		if userID != uid {
			return nil, repositories.ErrNotFound
		}
		lang := "fr"
		return &db.Resume{ID: id, Language: &lang, Content: []byte(`{"contact":{"name":"Ines Mansour","email":"i@x.tn","location":"Sfax"}}`)}, nil
	}
	res, apiErr := f.svc.ATSCheck(context.Background(), uid, rid, api.ATSCheckRequest{TargetLanguage: "en"})
	require.Nil(t, apiErr)
	assert.Equal(t, 24, res.Total)
	byID := map[string]api.ATSRuleResult{}
	for _, r := range res.Rules {
		byID[r.ID] = r
	}
	assert.Equal(t, "contact.phone", byID["contact_details"].Where)
	assert.Equal(t, "ines_mansour.pdf", byID["file_name"].Now)
	assert.Equal(t, "fix", byID["company_language"].Status)
	assert.Equal(t, "ok", byID["one_page"].Status)

	_, apiErr = f.svc.ATSCheck(context.Background(), "other", rid, api.ATSCheckRequest{})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
	_, apiErr = f.svc.ATSCheck(context.Background(), uid, "nope", api.ATSCheckRequest{})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
}

func TestInternalResumePDF(t *testing.T) {
	f := newFixture()
	_, apiErr := f.svc.InternalResumePDF(context.Background(), rid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	f.files.LatestExportFn = func(_ context.Context, _, format string) (*db.ResumeFile, error) {
		assert.Equal(t, "pdf", format)
		return &db.ResumeFile{Filename: "a.pdf", ContentType: "application/pdf", Data: pdfBytes()}, nil
	}
	got, apiErr := f.svc.InternalResumePDF(context.Background(), rid)
	require.Nil(t, apiErr)
	assert.Equal(t, "application/pdf", got.ContentType)
}

func ptr[T any](v T) *T { return &v }
