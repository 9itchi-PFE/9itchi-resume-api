package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeParser struct {
	out       *core.ParsedResume
	err       error
	text      string
	mediaType string
	image     []byte
}

func (p *fakeParser) ParseImage(_ context.Context, mediaType string, data []byte) (*core.ParsedResume, error) {
	p.mediaType, p.image = mediaType, data
	return p.out, p.err
}

func (p *fakeParser) Parse(_ context.Context, text string) (*core.ParsedResume, error) {
	p.text = text
	return p.out, p.err
}

type fakeQuotas struct {
	q   *core.Quotas
	err error
}

func (f fakeQuotas) Quotas(context.Context, string) (*core.Quotas, error) { return f.q, f.err }

func cvDocx(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	require.NoError(t, err)
	_, _ = w.Write([]byte(`<w:document xmlns:w="w"><w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body></w:document>`))
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// newSprint2Fixture runs background work synchronously.
func newSprint2Fixture(opts ...Option) (*fixture, *resumeService) {
	f := &fixture{resumes: &mocks.ResumeRepo{}, templates: &mocks.TemplateRepo{}, files: &mocks.FileRepo{}}
	svc := NewResumeService(f.resumes, f.templates, f.files, mocks.Tx{}, core.DefaultExporters(), opts...).(*resumeService)
	svc.async = func(fn func()) { fn() }
	f.svc = svc
	f.resumes.CreateFn = func(_ context.Context, userID, ti string, _ *string, c []byte) (*db.Resume, error) {
		return &db.Resume{ID: rid, UserID: userID, Title: ti, Content: c, ParseStatus: db.ParseNone}, nil
	}
	f.files.CreateFn = func(_ context.Context, fl *db.ResumeFile) (*db.ResumeFileMeta, error) {
		return &db.ResumeFileMeta{ID: "f", ResumeID: fl.ResumeID, Kind: fl.Kind, Format: fl.Format, Filename: fl.Filename}, nil
	}
	return f, svc
}

func TestUploadIsReadByAI(t *testing.T) {
	parser := &fakeParser{out: &core.ParsedResume{Language: "fr", Rewritten: 3, Removed: []string{"age", "religion", "full_address"}, Content: api.ResumeContent{Contact: api.Contact{Name: "Ines Mansour"}, Skills: []string{"Go"}}}}
	f, _ := newSprint2Fixture(WithParser(parser))
	var statuses []string
	var applied []byte
	var lang string
	var report []byte
	f.resumes.SetParseStatusFn = func(_ context.Context, id, st string) error { statuses = append(statuses, st); return nil }
	f.resumes.ApplyParsedFn = func(_ context.Context, id string, c []byte, l string, rep []byte) (bool, error) {
		applied, lang, report = c, l, rep
		return true, nil
	}
	data := cvDocx(t, "Ines Mansour, Go")
	res, apiErr := f.svc.UploadResume(context.Background(), uid, "cv.docx", int64(len(data)), bytes.NewReader(data))
	require.Nil(t, apiErr)
	assert.Equal(t, db.ParsePending, res.Resume.ParseStatus)
	assert.Equal(t, []string{db.ParsePending}, statuses)
	assert.Equal(t, "Ines Mansour, Go", parser.text)
	assert.Contains(t, string(applied), `"name":"Ines Mansour"`)
	assert.Contains(t, string(applied), `"certifications":[]`)
	assert.Equal(t, "fr", lang)
	assert.JSONEq(t, `{"pages":0,"seconds":0,"rewritten":3,"removed":["age","full_address"]}`, string(report))
}

func TestUploadPhotoOfCVIsReadAsImage(t *testing.T) {
	parser := &fakeParser{out: &core.ParsedResume{Language: "fr", Content: api.ResumeContent{Contact: api.Contact{Name: "Ines"}}}}
	f, _ := newSprint2Fixture(WithParser(parser))
	var stored *db.ResumeFile
	f.files.CreateFn = func(_ context.Context, fl *db.ResumeFile) (*db.ResumeFileMeta, error) {
		stored = fl
		return &db.ResumeFileMeta{ID: "f", Format: fl.Format}, nil
	}
	f.resumes.ApplyParsedFn = func(context.Context, string, []byte, string, []byte) (bool, error) { return true, nil }
	jpg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte("x"), 50)...)
	_, apiErr := f.svc.UploadResume(context.Background(), uid, "Scan_CV.JPG", int64(len(jpg)), bytes.NewReader(jpg))
	require.Nil(t, apiErr)
	assert.Equal(t, "jpeg", stored.Format)
	assert.Equal(t, "image/jpeg", stored.ContentType)
	assert.Equal(t, "image/jpeg", parser.mediaType)
	assert.Equal(t, jpg, parser.image)
	assert.Empty(t, parser.text, "no text extraction for a photo")

	png := []byte("\x89PNG\r\n\x1a\nrest")
	_, apiErr = f.svc.UploadResume(context.Background(), uid, "cv.png", int64(len(png)), bytes.NewReader(png))
	require.Nil(t, apiErr)
	assert.Equal(t, "image/png", parser.mediaType)

	_, apiErr = f.svc.UploadResume(context.Background(), uid, "cv.png", int64(len(jpg)), bytes.NewReader(jpg))
	require.NotNil(t, apiErr, "a JPEG named .png is refused")
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	_, apiErr = f.svc.UploadResume(context.Background(), uid, "cv.gif", 6, bytes.NewReader([]byte("GIF89a")))
	require.NotNil(t, apiErr)
}

func TestUploadReadingFailures(t *testing.T) {
	cases := map[string]struct {
		parser *fakeParser
		data   func(t *testing.T) []byte
	}{
		"no text":    {&fakeParser{}, func(t *testing.T) []byte { return cvDocx(t, "") }},
		"ai fails":   {&fakeParser{err: errors.New("503")}, func(t *testing.T) []byte { return cvDocx(t, "cv") }},
		"unreadable": {&fakeParser{}, func(*testing.T) []byte { return append([]byte("PK\x03\x04"), bytes.Repeat([]byte("x"), 30)...) }},
	}
	for name, tc := range cases {
		f, _ := newSprint2Fixture(WithParser(tc.parser))
		var statuses []string
		f.resumes.SetParseStatusFn = func(_ context.Context, _, st string) error { statuses = append(statuses, st); return nil }
		f.resumes.ApplyParsedFn = func(context.Context, string, []byte, string, []byte) (bool, error) {
			t.Errorf("%s: nothing must be applied", name)
			return false, nil
		}
		data := tc.data(t)
		_, apiErr := f.svc.UploadResume(context.Background(), uid, "cv.docx", int64(len(data)), bytes.NewReader(data))
		require.Nil(t, apiErr, name)
		assert.Equal(t, []string{db.ParsePending, db.ParseFailed}, statuses, name)
	}
}

func TestUploadWithoutParserStaysNone(t *testing.T) {
	f, _ := newSprint2Fixture()
	f.resumes.SetParseStatusFn = func(context.Context, string, string) error {
		t.Error("no status change without a parser")
		return nil
	}
	data := cvDocx(t, "cv")
	res, apiErr := f.svc.UploadResume(context.Background(), uid, "cv.docx", int64(len(data)), bytes.NewReader(data))
	require.Nil(t, apiErr)
	assert.Equal(t, db.ParseNone, res.Resume.ParseStatus)
}

func TestFreePlanAllowsOneCV(t *testing.T) {
	free := fakeQuotas{q: &core.Quotas{PlanName: "Free", Limits: map[string]int{"resumes": 1, "cv_downloads": 1}}}
	f, _ := newSprint2Fixture(WithQuotas(free))
	count := 0
	f.resumes.CountByUserFn = func(context.Context, string) (int, error) { return count, nil }

	_, apiErr := f.svc.CreateResume(context.Background(), uid, api.CreateResumeRequest{Title: "CV"})
	require.Nil(t, apiErr, "the first CV is free")

	count = 1
	_, apiErr = f.svc.CreateResume(context.Background(), uid, api.CreateResumeRequest{Title: "CV 2"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
	assert.Equal(t, errResumeLimit, apiErr.Error.Error())

	data := cvDocx(t, "cv")
	_, apiErr = f.svc.UploadResume(context.Background(), uid, "cv.docx", int64(len(data)), bytes.NewReader(data))
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)

	paid, _ := newSprint2Fixture(WithQuotas(fakeQuotas{q: &core.Quotas{PlanName: "Mensuel", Limits: map[string]int{}}}))
	paid.resumes.CountByUserFn = func(context.Context, string) (int, error) { return 5, nil }
	_, apiErr = paid.svc.CreateResume(context.Background(), uid, api.CreateResumeRequest{Title: "CV 6"})
	assert.Nil(t, apiErr, "no resumes key means no limit")
}

func TestDownloadQuotaAndBranding(t *testing.T) {
	setup := func(q core.QuotaSource, exports int) (*fixture, *[]byte) {
		f, _ := newSprint2Fixture(WithQuotas(q))
		f.resumes.GetFn = func(_ context.Context, _, id string) (*db.Resume, error) {
			return &db.Resume{ID: id, Title: "Mon CV", Content: []byte(`{"contact":{"name":"Inès Ben Mansour"}}`)}, nil
		}
		f.files.CountExportsByUserFn = func(context.Context, string) (int, error) { return exports, nil }
		var data []byte
		f.files.CreateFn = func(_ context.Context, fl *db.ResumeFile) (*db.ResumeFileMeta, error) {
			data = fl.Data
			return &db.ResumeFileMeta{ID: "f", Kind: fl.Kind, Format: fl.Format, Filename: fl.Filename}, nil
		}
		return f, &data
	}
	free := fakeQuotas{q: &core.Quotas{PlanName: "Free", Limits: map[string]int{"cv_downloads": 1}}}

	f, data := setup(free, 0)
	res, apiErr := f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "pdf"})
	require.Nil(t, apiErr)
	assert.Equal(t, "ines_ben_mansour.pdf", res.Filename, "rule 22: prenom_nom.pdf")
	text, err := core.ExtractText("pdf", *data)
	require.NoError(t, err)
	assert.Contains(t, text, core.BrandLead, "the free download carries the mark")

	f, _ = setup(free, 0)
	_, apiErr = f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "docx"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
	assert.Equal(t, errWordForSubscribers, apiErr.Error.Error())

	f, _ = setup(free, 1)
	_, apiErr = f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "pdf"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
	assert.Equal(t, errDownloadLimit, apiErr.Error.Error())

	f, data = setup(fakeQuotas{q: &core.Quotas{PlanName: "Mensuel", Limits: map[string]int{}}}, 12)
	_, apiErr = f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "pdf"})
	require.Nil(t, apiErr)
	text, _ = core.ExtractText("pdf", *data)
	assert.NotContains(t, text, core.BrandLead)
	res, apiErr = f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "docx"})
	require.Nil(t, apiErr, "subscribers get the Word file")
	assert.Equal(t, "ines_ben_mansour.docx", res.Filename)
	text, err = core.ExtractText("docx", *data)
	require.NoError(t, err)
	assert.Contains(t, text, "Inès Ben Mansour")

	f, _ = setup(fakeQuotas{err: core.ErrPeerUnavailable}, 0)
	_, apiErr = f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "pdf"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.Code, "fails closed when the plan cannot be checked")
}

func TestExportFilename(t *testing.T) {
	assert.Equal(t, "ines_mansour", exportFilename(" Inès  Mansour ", "x"))
	assert.Equal(t, "mohamed_ali_ben_salah", exportFilename("Mohamed-Ali Ben Salah", "x"))
	assert.Equal(t, "My CV", exportFilename("", "My CV"))
	assert.Equal(t, "My CV", exportFilename("محمد", "My CV"), "falls back to the title without latin letters")
}

func TestExportRecordsTheDownload(t *testing.T) {
	f, _ := newSprint2Fixture()
	f.resumes.GetFn = func(_ context.Context, _, id string) (*db.Resume, error) {
		return &db.Resume{ID: id, Title: "CV", Content: []byte(`{}`)}, nil
	}
	var recorded []string
	f.files.RecordDownloadFn = func(_ context.Context, userID, resumeID, format string) error {
		recorded = append(recorded, userID+"/"+resumeID+"/"+format)
		return nil
	}
	_, apiErr := f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "pdf"})
	require.Nil(t, apiErr)
	assert.Equal(t, []string{uid + "/" + rid + "/pdf"}, recorded)

	f.files.RecordDownloadFn = func(context.Context, string, string, string) error { return errors.New("db down") }
	_, apiErr = f.svc.ExportResume(context.Background(), uid, rid, api.ExportResumeRequest{Format: "pdf"})
	require.NotNil(t, apiErr, "no download without its record")
}
