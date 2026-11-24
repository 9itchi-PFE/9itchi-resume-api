package services

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"9itchi-resume-api/core"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const vid = "44444444-4444-4444-4444-444444444444"

func ownedOnly(f *fixture) {
	f.resumes.ExistsFn = func(_ context.Context, userID, _ string) error {
		if userID != uid {
			return repositories.ErrNotFound
		}
		return nil
	}
}

func TestGetVersion(t *testing.T) {
	f := newFixture()
	ownedOnly(f)
	f.resumes.GetVersionFn = func(_ context.Context, resumeID, versionID string) (*db.ResumeVersion, error) {
		return &db.ResumeVersion{ID: versionID, ResumeID: resumeID, VersionNo: 3, Content: []byte(`{"summary":"old"}`), CreatedAt: time.Unix(0, 0)}, nil
	}
	v, apiErr := f.svc.GetVersion(context.Background(), uid, rid, vid)
	require.Nil(t, apiErr)
	assert.Equal(t, 3, v.VersionNo)
	assert.Equal(t, "old", v.Content.Summary)

	_, apiErr = f.svc.GetVersion(context.Background(), "other", rid, vid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
	_, apiErr = f.svc.GetVersion(context.Background(), uid, rid, "x")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
}

func TestRestoreVersionSavesCurrentFirst(t *testing.T) {
	f := newFixture()
	var steps []string
	f.resumes.GetVersionFn = func(context.Context, string, string) (*db.ResumeVersion, error) {
		steps = append(steps, "get")
		return &db.ResumeVersion{ID: vid, Content: []byte(`{"summary":"old"}`)}, nil
	}
	f.resumes.BumpVersionFn = func(context.Context, string, string) (int, []byte, error) {
		steps = append(steps, "bump")
		return 7, []byte(`{"summary":"current"}`), nil
	}
	f.resumes.InsertVersionFn = func(_ context.Context, _ string, n int, label string, c []byte) (*db.ResumeVersionMeta, error) {
		assert.Equal(t, LabelBeforeRestore, label)
		steps = append(steps, "save:"+string(c))
		assert.Equal(t, 7, n)
		return &db.ResumeVersionMeta{}, nil
	}
	f.resumes.UpdateFn = func(_ context.Context, _, id string, _, _ *string, c []byte) (*db.Resume, error) {
		steps = append(steps, "update:"+string(c))
		return &db.Resume{ID: id, Content: c}, nil
	}
	res, apiErr := f.svc.RestoreVersion(context.Background(), uid, rid, vid)
	require.Nil(t, apiErr)
	assert.Equal(t, "old", res.Content.Summary)
	assert.Equal(t, []string{"get", "bump", `save:{"summary":"current"}`, `update:{"summary":"old"}`}, steps)

	f.resumes.GetVersionFn = func(context.Context, string, string) (*db.ResumeVersion, error) { return nil, repositories.ErrNotFound }
	_, apiErr = f.svc.RestoreVersion(context.Background(), uid, rid, vid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}

func TestDuplicateResume(t *testing.T) {
	tpl := tid
	f, _ := newSprint2Fixture(WithQuotas(fakeQuotas{q: &core.Quotas{PlanName: "Mensuel", Limits: map[string]int{}}}))
	f.resumes.GetFn = func(context.Context, string, string) (*db.Resume, error) {
		return &db.Resume{ID: rid, TemplateID: &tpl, Content: []byte(`{"summary":"web"}`)}, nil
	}
	var created []string
	f.resumes.CreateFn = func(_ context.Context, _, title string, templateID *string, c []byte) (*db.Resume, error) {
		created = append(created, title+"|"+*templateID+"|"+string(c))
		return &db.Resume{ID: "new", Title: title, Content: c}, nil
	}
	res, apiErr := f.svc.DuplicateResume(context.Background(), uid, rid, api.DuplicateResumeRequest{Title: "Data"})
	require.Nil(t, apiErr)
	assert.Equal(t, "Data", res.Title)
	assert.Equal(t, []string{`Data|` + tid + `|{"summary":"web"}`}, created)

	free, _ := newSprint2Fixture(WithQuotas(fakeQuotas{q: &core.Quotas{PlanName: "Free", Limits: map[string]int{"resumes": 1}}}))
	free.resumes.GetFn = f.resumes.GetFn
	free.resumes.CountByUserFn = func(context.Context, string) (int, error) { return 1, nil }
	_, apiErr = free.svc.DuplicateResume(context.Background(), uid, rid, api.DuplicateResumeRequest{Title: "Data"})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusPaymentRequired, apiErr.Code)
}

func TestPhoto(t *testing.T) {
	f := newFixture()
	ownedOnly(f)
	var stored *db.ResumeFile
	deleted := 0
	f.files.DeletePhotosFn = func(context.Context, string) error { deleted++; return nil }
	f.files.CreateFn = func(_ context.Context, fl *db.ResumeFile) (*db.ResumeFileMeta, error) {
		stored = fl
		return &db.ResumeFileMeta{}, nil
	}
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte("x"), 100)...)
	require.Nil(t, f.svc.UploadPhoto(context.Background(), uid, rid, bytes.NewReader(jpeg)))
	assert.Equal(t, "photo", stored.Kind)
	assert.Equal(t, "jpeg", stored.Format)
	assert.Equal(t, "image/jpeg", stored.ContentType)
	assert.Equal(t, 1, deleted, "the previous photo is replaced")

	cases := map[string]struct {
		user string
		data []byte
		code int
	}{
		"not an image": {uid, []byte("GIF89a......"), http.StatusBadRequest},
		"too large":    {uid, append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxPhotoBytes)...), http.StatusRequestEntityTooLarge},
		"not owner":    {"other", jpeg, http.StatusNotFound},
	}
	for name, tc := range cases {
		apiErr := f.svc.UploadPhoto(context.Background(), tc.user, rid, bytes.NewReader(tc.data))
		require.NotNil(t, apiErr, name)
		assert.Equal(t, tc.code, apiErr.Code, name)
	}

	f.files.LatestPhotoFn = func(context.Context, string) (*db.ResumeFile, error) {
		return &db.ResumeFile{Filename: "photo.jpeg", ContentType: "image/jpeg", Data: jpeg}, nil
	}
	p, apiErr := f.svc.GetPhoto(context.Background(), uid, rid)
	require.Nil(t, apiErr)
	assert.Equal(t, "image/jpeg", p.ContentType)
	_, apiErr = f.svc.GetPhoto(context.Background(), "other", rid)
	require.NotNil(t, apiErr)

	require.Nil(t, f.svc.DeletePhoto(context.Background(), uid, rid))
	assert.Equal(t, 2, deleted)
}

func TestGetResumeSaysWhetherThereIsAPhoto(t *testing.T) {
	f := newFixture()
	f.resumes.GetFn = func(_ context.Context, _, id string) (*db.Resume, error) {
		return &db.Resume{ID: id, Content: []byte(`{}`)}, nil
	}
	f.files.HasPhotoFn = func(context.Context, string) (bool, error) { return true, nil }
	res, apiErr := f.svc.GetResume(context.Background(), uid, rid)
	require.Nil(t, apiErr)
	assert.True(t, res.HasPhoto)
}

func TestDownloads(t *testing.T) {
	f := newFixture()
	f.files.CountExportsByUserFn = func(context.Context, string) (int, error) { return 2, nil }
	d, apiErr := f.svc.Downloads(context.Background(), uid)
	require.Nil(t, apiErr)
	assert.Equal(t, 2, d.Used)
	f.files.CountExportsByUserFn = func(context.Context, string) (int, error) { return 0, assert.AnError }
	_, apiErr = f.svc.Downloads(context.Background(), uid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}
