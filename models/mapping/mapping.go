package mapping

import (
	"encoding/json"

	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
)

// NormalizeContent replaces nil slices by empty ones so JSON output has [] instead of null.
func NormalizeContent(c api.ResumeContent) api.ResumeContent {
	if c.Contact.Links == nil {
		c.Contact.Links = []string{}
	}
	if c.Experience == nil {
		c.Experience = []api.Experience{}
	}
	for i := range c.Experience {
		if c.Experience[i].Description == nil {
			c.Experience[i].Description = []string{}
		}
	}
	if c.Education == nil {
		c.Education = []api.Education{}
	}
	if c.Skills == nil {
		c.Skills = []string{}
	}
	if c.Languages == nil {
		c.Languages = []string{}
	}
	if c.Certifications == nil {
		c.Certifications = []api.Certification{}
	}
	return c
}

// ParseContent decodes stored jsonb into the typed schema. Empty or invalid JSON yields an empty document.
func ParseContent(raw []byte) api.ResumeContent {
	var c api.ResumeContent
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &c)
	}
	return NormalizeContent(c)
}

func ResumeToResponse(r *db.Resume) api.ResumeResponse {
	return api.ResumeResponse{
		ID:          r.ID,
		Title:       r.Title,
		TemplateID:  r.TemplateID,
		Version:     r.Version,
		ParseStatus: r.ParseStatus,
		Language:    r.Language,
		Content:     ParseContent(r.Content),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func ResumeSummaryToItem(r db.ResumeSummary) api.ResumeListItem {
	return api.ResumeListItem{
		ID:         r.ID,
		Title:      r.Title,
		TemplateID: r.TemplateID,
		Version:    r.Version,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func VersionToResponse(v db.ResumeVersionMeta) api.VersionResponse {
	return api.VersionResponse{ID: v.ID, VersionNo: v.VersionNo, CreatedAt: v.CreatedAt}
}

func TemplateToResponse(t *db.Template) api.TemplateResponse {
	return api.TemplateResponse{
		ID:         t.ID,
		Name:       t.Name,
		Layout:     t.Layout,
		PreviewURL: t.PreviewURL,
		Active:     t.Active,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

func FileMetaToResponse(f *db.ResumeFileMeta) api.FileResponse {
	return api.FileResponse{
		ID:          f.ID,
		ResumeID:    f.ResumeID,
		Kind:        f.Kind,
		Format:      f.Format,
		Filename:    f.Filename,
		ContentType: f.ContentType,
		SizeBytes:   f.SizeBytes,
		SHA256:      f.SHA256,
		CreatedAt:   f.CreatedAt,
	}
}
