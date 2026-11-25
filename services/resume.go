package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/models/mapping"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100

	contentTypePDF  = "application/pdf"
	contentTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// ResumeService holds all business logic for resume. Handlers stay thin.
type ResumeService interface {
	// POST /resumes/upload: store the PDF/DOCX as the original and create an empty resume
	UploadResume(ctx context.Context, userID string, filename string, size int64, r io.Reader) (*api.UploadResumeResponse, *models.ApiError)
	// POST /resumes: create resume from structured content
	CreateResume(ctx context.Context, userID string, req api.CreateResumeRequest) (*api.ResumeResponse, *models.ApiError)
	// GET /resumes: list own resumes (no content), page/pageSize <= 0 use defaults
	ListResumes(ctx context.Context, userID string, page, pageSize int) (*api.ListResumesResponse, *models.ApiError)
	// GET /resumes/:id: resume detail
	GetResume(ctx context.Context, userID string, id string) (*api.ResumeResponse, *models.ApiError)
	// PUT /resumes/:id: edit sections / switch template
	UpdateResume(ctx context.Context, userID string, id string, req api.UpdateResumeRequest) (*api.ResumeResponse, *models.ApiError)
	// DELETE /resumes/:id: delete resume
	DeleteResume(ctx context.Context, userID string, id string) *models.ApiError
	// GET /resumes/:id/versions: list saved versions (no content)
	ListVersions(ctx context.Context, userID string, id string) (*api.ListVersionsResponse, *models.ApiError)
	// POST /resumes/:id/versions: snapshot a version
	CreateVersion(ctx context.Context, userID string, id string, req api.CreateVersionRequest) (*api.VersionResponse, *models.ApiError)
	// POST /resumes/:id/export: export PDF, or Word for subscribers
	ExportResume(ctx context.Context, userID string, id string, req api.ExportResumeRequest) (*api.FileResponse, *models.ApiError)
	// POST /resumes/:id/ats-check: the 24 CV rules
	ATSCheck(ctx context.Context, userID string, id string, req api.ATSCheckRequest) (*api.ATSCheckResponse, *models.ApiError)
	// GET /admin/users/:id/cv: where the CV a student changed last stands with the 24 rules, for the team
	UserCV(ctx context.Context, id string) (*api.UserCVResponse, *models.ApiError)
	// GET /resumes/:id/versions/:versionId: one saved version with its content
	GetVersion(ctx context.Context, userID, id, versionID string) (*api.VersionContentResponse, *models.ApiError)
	// POST /resumes/:id/versions/:versionId/restore: put a saved version back (the replaced content is saved first)
	RestoreVersion(ctx context.Context, userID, id, versionID string) (*api.ResumeResponse, *models.ApiError)
	// POST /resumes/:id/duplicate: copy a CV under a new title (counts against the CV limit)
	DuplicateResume(ctx context.Context, userID, id string, req api.DuplicateResumeRequest) (*api.ResumeResponse, *models.ApiError)
	// PUT /resumes/:id/photo: JPEG or PNG, 2 MB max, replaces the previous photo
	UploadPhoto(ctx context.Context, userID, id string, r io.Reader) *models.ApiError
	// GET /resumes/:id/photo
	GetPhoto(ctx context.Context, userID, id string) (*api.FileContent, *models.ApiError)
	// DELETE /resumes/:id/photo
	DeletePhoto(ctx context.Context, userID, id string) *models.ApiError
	// GET /me/downloads: how many CV downloads the user made (the free plan allows one)
	Downloads(ctx context.Context, userID string) (*api.DownloadsResponse, *models.ApiError)
	// GET /internal/resumes/:id/pdf: the CV as it is now, rendered as a PDF (email attachments), no user scoping
	InternalResumePDF(ctx context.Context, id string) (*api.FileContent, *models.ApiError)
	// GET /internal/resumes/:id/original (internal): the file the student imported, untouched
	InternalResumeOriginal(ctx context.Context, id string) (*api.FileContent, *models.ApiError)
}

type resumeService struct {
	repo      repositories.ResumeRepository
	templates repositories.TemplateRepository
	files     repositories.FileRepository
	tx        repositories.Transactor
	exporters map[string]core.Exporter
	// parser reads uploaded CVs (ai-api); nil leaves uploads empty with parse_status none.
	parser core.ResumeParser
	// quotas enforces the plan limits (payments-api); nil means no limits (tests, local runs without payments).
	quotas core.QuotaSource
	// async runs background work; tests replace it to run synchronously.
	async func(func())
}

// Option configures the resume service.
type Option func(*resumeService)

// WithParser enables AI reading of uploaded CVs.
func WithParser(p core.ResumeParser) Option { return func(s *resumeService) { s.parser = p } }

// WithQuotas enables plan limits: CVs, downloads per period and the groups of the diagnosis shown.
func WithQuotas(q core.QuotaSource) Option { return func(s *resumeService) { s.quotas = q } }

func NewResumeService(
	repo repositories.ResumeRepository,
	templates repositories.TemplateRepository,
	files repositories.FileRepository,
	tx repositories.Transactor,
	exporters map[string]core.Exporter,
	opts ...Option,
) ResumeService {
	s := &resumeService{repo: repo, templates: templates, files: files, tx: tx, exporters: exporters,
		async: func(f func()) { go f() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// encodeContent validates and serialises content for the jsonb column.
func encodeContent(c api.ResumeContent) ([]byte, *models.ApiError) {
	if !c.Style.Valid() {
		return nil, newError(http.StatusBadRequest, "invalid style color (use #RRGGBB)")
	}
	if !api.ValidSectionOrder(c.SectionOrderKeys) || !api.ValidSectionOrder(c.HiddenSections) {
		return nil, newError(http.StatusBadRequest, "invalid section order")
	}
	raw, err := json.Marshal(mapping.NormalizeContent(c))
	if err != nil {
		return nil, newError(http.StatusBadRequest, "invalid content")
	}
	if len(raw) > api.MaxContentBytes {
		return nil, newError(http.StatusBadRequest, "content too large (max 200 KB)")
	}
	// PostgreSQL rejects NUL characters in jsonb strings.
	if bytes.Contains(raw, []byte(`\u0000`)) {
		return nil, newError(http.StatusBadRequest, "content contains invalid characters")
	}
	return raw, nil
}

func validateTitle(t string) *models.ApiError {
	if strings.TrimSpace(t) == "" {
		return newError(http.StatusBadRequest, "title is required")
	}
	if utf8.RuneCountInString(t) > api.MaxTitleLen {
		return newError(http.StatusBadRequest, "title too long (max 200 characters)")
	}
	if !utf8.ValidString(t) || strings.ContainsRune(t, 0) {
		return newError(http.StatusBadRequest, "title contains invalid characters")
	}
	return nil
}

// checkTemplate verifies that an optional template id exists and is active.
func (s *resumeService) checkTemplate(ctx context.Context, id *string) *models.ApiError {
	if id == nil {
		return nil
	}
	if !isUUID(*id) {
		return newError(http.StatusBadRequest, "invalid template id")
	}
	t, err := s.templates.Get(ctx, *id)
	if errors.Is(err, repositories.ErrNotFound) {
		return newError(http.StatusBadRequest, "template not found")
	}
	if err != nil {
		return repoError(err, "template not found")
	}
	if !t.Active {
		return newError(http.StatusBadRequest, "template is not active")
	}
	return nil
}

func (s *resumeService) UploadResume(ctx context.Context, userID string, filename string, size int64, r io.Reader) (*api.UploadResumeResponse, *models.ApiError) {
	if size > api.MaxUploadBytes {
		return nil, newError(http.StatusRequestEntityTooLarge, "file too large (max 10 MB)")
	}
	if size == 0 {
		return nil, newError(http.StatusBadRequest, "file is empty")
	}
	safeName := sanitizeFilename(filename)
	format, contentType, apiErr := detectUpload(safeName)
	if apiErr != nil {
		return nil, apiErr
	}
	if _, apiErr := s.checkResumeQuota(ctx, userID); apiErr != nil {
		return nil, apiErr
	}
	// Read at most limit+1 bytes: the declared size cannot be trusted.
	data, err := io.ReadAll(io.LimitReader(r, api.MaxUploadBytes+1))
	if err != nil {
		return nil, newError(http.StatusBadRequest, "could not read file")
	}
	if len(data) > api.MaxUploadBytes {
		return nil, newError(http.StatusRequestEntityTooLarge, "file too large (max 10 MB)")
	}
	if len(data) == 0 {
		return nil, newError(http.StatusBadRequest, "file is empty")
	}
	if !hasMagic(format, data) {
		return nil, newError(http.StatusBadRequest, "file content does not match its type")
	}

	title := strings.TrimSpace(strings.TrimSuffix(safeName, filepath.Ext(safeName)))
	if title == "" {
		title = "Untitled"
	}
	if r := []rune(title); len(r) > api.MaxTitleLen {
		title = string(r[:api.MaxTitleLen])
	}
	empty, apiErr := encodeContent(api.ResumeContent{})
	if apiErr != nil {
		return nil, apiErr
	}
	sum := sha256.Sum256(data)

	var resume *db.Resume
	var meta *db.ResumeFileMeta
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if resume, err = s.repo.Create(ctx, userID, title, nil, empty); err != nil {
			return err
		}
		meta, err = s.files.Create(ctx, &db.ResumeFile{
			ResumeID:    resume.ID,
			Kind:        "original",
			Format:      format,
			Filename:    safeName,
			ContentType: contentType,
			SizeBytes:   int64(len(data)),
			SHA256:      hex.EncodeToString(sum[:]),
			Data:        data,
		})
		return err
	})
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	if s.parser != nil {
		if err := s.repo.SetParseStatus(ctx, resume.ID, db.ParsePending); err != nil {
			utils.Logger.Error("upload: mark pending", zap.Error(err))
		} else {
			resume.ParseStatus = db.ParsePending
			id := resume.ID
			s.async(func() { s.readUpload(id, format, data) })
		}
	}
	return &api.UploadResumeResponse{
		Resume: mapping.ResumeToResponse(resume),
		File:   mapping.FileMetaToResponse(meta),
	}, nil
}

// detectUpload validates the extension and returns the format and content type.
func detectUpload(name string) (format, contentType string, apiErr *models.ApiError) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf":
		return "pdf", contentTypePDF, nil
	case ".docx":
		return "docx", contentTypeDOCX, nil
	case ".jpg", ".jpeg":
		return "jpeg", "image/jpeg", nil
	case ".png":
		return "png", "image/png", nil
	}
	return "", "", newError(http.StatusBadRequest, "only PDF, DOCX, JPG or PNG files are accepted")
}

// hasMagic checks the leading bytes: "%PDF-" for pdf, a zip header ("PK") for docx, the JPEG and PNG signatures.
func hasMagic(format string, data []byte) bool {
	switch format {
	case "pdf":
		return bytes.HasPrefix(data, []byte("%PDF-"))
	case "docx":
		return bytes.HasPrefix(data, []byte("PK\x03\x04"))
	case "jpeg":
		return bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF})
	case "png":
		return bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	}
	return false
}

func (s *resumeService) CreateResume(ctx context.Context, userID string, req api.CreateResumeRequest) (*api.ResumeResponse, *models.ApiError) {
	if _, apiErr := s.checkResumeQuota(ctx, userID); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := validateTitle(req.Title); apiErr != nil {
		return nil, apiErr
	}
	content, apiErr := encodeContent(req.Content)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := s.checkTemplate(ctx, req.TemplateID); apiErr != nil {
		return nil, apiErr
	}
	res, err := s.repo.Create(ctx, userID, req.Title, req.TemplateID, content)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	resp := mapping.ResumeToResponse(res)
	return &resp, nil
}

func (s *resumeService) ListResumes(ctx context.Context, userID string, page, pageSize int) (*api.ListResumesResponse, *models.ApiError) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	rows, total, err := s.repo.List(ctx, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	out := &api.ListResumesResponse{
		Items: make([]api.ResumeListItem, 0, len(rows)), Total: total, Page: page, PageSize: pageSize,
	}
	for _, r := range rows {
		out.Items = append(out.Items, mapping.ResumeSummaryToItem(r))
	}
	return out, nil
}

func (s *resumeService) GetResume(ctx context.Context, userID string, id string) (*api.ResumeResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	res, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	return s.response(ctx, res), nil
}

func (s *resumeService) UpdateResume(ctx context.Context, userID string, id string, req api.UpdateResumeRequest) (*api.ResumeResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	if req.Title != nil {
		if apiErr := validateTitle(*req.Title); apiErr != nil {
			return nil, apiErr
		}
	}
	var content []byte
	if req.Content != nil {
		var apiErr *models.ApiError
		if content, apiErr = encodeContent(*req.Content); apiErr != nil {
			return nil, apiErr
		}
	}
	if apiErr := s.checkTemplate(ctx, req.TemplateID); apiErr != nil {
		return nil, apiErr
	}
	// Content is only written when provided, so a template switch never alters it.
	res, err := s.repo.Update(ctx, userID, id, req.Title, req.TemplateID, content)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	resp := mapping.ResumeToResponse(res)
	return &resp, nil
}

func (s *resumeService) DeleteResume(ctx context.Context, userID string, id string) *models.ApiError {
	if !isUUID(id) {
		return newError(http.StatusBadRequest, "invalid resume id")
	}
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		return repoError(err, "resume not found")
	}
	return nil
}

func (s *resumeService) ListVersions(ctx context.Context, userID string, id string) (*api.ListVersionsResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	if err := s.repo.Exists(ctx, userID, id); err != nil {
		return nil, repoError(err, "resume not found")
	}
	rows, err := s.repo.ListVersions(ctx, id)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	out := &api.ListVersionsResponse{Items: make([]api.VersionResponse, 0, len(rows))}
	for _, v := range rows {
		out.Items = append(out.Items, mapping.VersionToResponse(v))
	}
	return out, nil
}

func (s *resumeService) CreateVersion(ctx context.Context, userID string, id string, req api.CreateVersionRequest) (*api.VersionResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	var meta *db.ResumeVersionMeta
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		versionNo, content, err := s.repo.BumpVersion(ctx, userID, id)
		if err != nil {
			return err
		}
		meta, err = s.repo.InsertVersion(ctx, id, versionNo, req.Label, content)
		return err
	})
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	resp := mapping.VersionToResponse(*meta)
	return &resp, nil
}

func (s *resumeService) ExportResume(ctx context.Context, userID string, id string, req api.ExportResumeRequest) (*api.FileResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	exporter, ok := s.exporters[format]
	if !ok {
		return nil, newError(http.StatusBadRequest, "unsupported export format, use pdf or docx")
	}
	res, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	branded, apiErr := s.checkDownloadQuota(ctx, userID)
	if apiErr != nil {
		return nil, apiErr
	}
	// The Word file is for subscribers (screen 4.1 download dialog); free downloads are branded PDFs.
	if format == "docx" && branded {
		return nil, newError(http.StatusPaymentRequired, errWordForSubscribers)
	}
	data, content, apiErr := s.render(ctx, res, exporter, branded)
	if apiErr != nil {
		return nil, apiErr
	}
	sum := sha256.Sum256(data)
	var meta *db.ResumeFileMeta
	err = s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if meta, err = s.files.Create(ctx, &db.ResumeFile{
			ResumeID:    res.ID,
			Kind:        "export",
			Format:      format,
			Filename:    exportFilename(content.Contact.Name, res.Title) + "." + format,
			ContentType: exportContentType(format),
			SizeBytes:   int64(len(data)),
			SHA256:      hex.EncodeToString(sum[:]),
			Data:        data,
		}); err != nil {
			return err
		}
		return s.files.RecordDownload(ctx, userID, res.ID, format)
	})
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	resp := mapping.FileMetaToResponse(meta)
	return &resp, nil
}

func (s *resumeService) ATSCheck(ctx context.Context, userID string, id string, req api.ATSCheckRequest) (*api.ATSCheckResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	res, err := s.repo.Get(ctx, userID, id)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	// The plan decides which groups of the diagnosis are shown; when it cannot be read nothing is shown (503).
	q, apiErr := s.plan(ctx, userID)
	if apiErr != nil {
		return nil, apiErr
	}
	out := core.LockGroups(checkResume(ctx, s.templates, res, req.TargetLanguage), q.Shows)
	return &out, nil
}

// UserCV is what the team sees of a student's CV: how many they have and how the one changed last does with the
// 24 rules. What the CV says is not returned.
func (s *resumeService) UserCV(ctx context.Context, id string) (*api.UserCVResponse, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid user id")
	}
	id = strings.ToLower(id)
	latest, total, err := s.repo.List(ctx, id, 1, 0)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	if len(latest) == 0 {
		return nil, newError(http.StatusNotFound, "resume not found")
	}
	res, err := s.repo.Get(ctx, id, latest[0].ID)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	check := checkResume(ctx, s.templates, res, "")
	return &api.UserCVResponse{
		ResumeID: res.ID, Title: res.Title, Resumes: total, Version: res.Version,
		Passed: check.Passed, Total: check.Total, ToFix: check.ToFix, UpdatedAt: res.UpdatedAt,
	}, nil
}

// checkResume runs the 24 rules on a CV as it is now, rendered with its template. targetLanguage is the language
// of the company the student applies to, "" when not said.
func checkResume(ctx context.Context, templates repositories.TemplateRepository, res *db.Resume, targetLanguage string) api.ATSCheckResponse {
	content := mapping.ParseContent(res.Content)
	layout := ""
	if res.TemplateID != nil {
		if t, err := templates.Get(ctx, *res.TemplateID); err == nil {
			layout = t.Layout
		}
	}
	lang := ""
	if res.Language != nil {
		lang = *res.Language
	}
	headings := "fr"
	if lang == "en" {
		headings = "en"
	}
	filename := ""
	if name := exportFilename(content.Contact.Name, ""); name != "" {
		filename = name + ".pdf"
	}
	return core.CheckATS(core.ATSInput{
		Content:        content,
		Language:       lang,
		TargetLanguage: targetLanguage,
		ExportFilename: filename,
		Rendered:       core.RenderForCheck(ctx, content, layout, headings),
	})
}

// render draws the resume as it is now (template, language, photo) with the given exporter. branded adds the
// "CV créé avec 9itchi" mark of the free download.
func (s *resumeService) render(ctx context.Context, res *db.Resume, exporter core.Exporter, branded bool) ([]byte, api.ResumeContent, *models.ApiError) {
	layout := ""
	if res.TemplateID != nil {
		if t, err := s.templates.Get(ctx, *res.TemplateID); err == nil {
			layout = t.Layout
		}
	}
	content := mapping.ParseContent(res.Content)
	lang := "fr"
	if res.Language != nil && *res.Language == "en" {
		lang = "en"
	}
	opts := core.RenderOptions{Language: lang, Branded: branded}
	if layout != "compact" {
		if p, err := s.files.LatestPhoto(ctx, res.ID); err == nil {
			opts.Photo, opts.PhotoFormat = p.Data, p.Format
		} else if !errors.Is(err, repositories.ErrNotFound) {
			return nil, content, repoError(err, "resume not found")
		}
	}
	data, err := exporter.Render(ctx, content, layout, opts)
	if err != nil {
		utils.Logger.Error("export render failed", zap.Error(err))
		return nil, content, newError(http.StatusInternalServerError, "export failed")
	}
	return data, content, nil
}

// InternalResumePDF renders the CV as it is now, so an application always carries the current CV and not an
// older download. Nothing is stored and it does not count as a download of the student.
func (s *resumeService) InternalResumePDF(ctx context.Context, id string) (*api.FileContent, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	res, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, repoError(err, "resume not found")
	}
	if res == nil {
		return nil, newError(http.StatusNotFound, "resume not found")
	}
	// Never branded: 9itchi only sends applications for subscribers.
	data, content, apiErr := s.render(ctx, res, s.exporters["pdf"], false)
	if apiErr != nil {
		return nil, apiErr
	}
	return &api.FileContent{Filename: exportFilename(content.Contact.Name, res.Title) + ".pdf", ContentType: contentTypePDF, Data: data}, nil
}

// InternalResumeOriginal returns the file the student imported for this CV, byte for byte: an application sent
// without a subscription carries it, not the CV built in 9itchi. 404 when the CV was not started from a file.
func (s *resumeService) InternalResumeOriginal(ctx context.Context, id string) (*api.FileContent, *models.ApiError) {
	if !isUUID(id) {
		return nil, newError(http.StatusBadRequest, "invalid resume id")
	}
	f, err := s.files.Original(ctx, id)
	if err != nil {
		return nil, repoError(err, "imported file not found")
	}
	if f == nil {
		return nil, newError(http.StatusNotFound, "imported file not found")
	}
	return &api.FileContent{Filename: f.Filename, ContentType: f.ContentType, Data: f.Data}, nil
}

func exportContentType(format string) string {
	if format == "docx" {
		return contentTypeDOCX
	}
	return contentTypePDF
}
