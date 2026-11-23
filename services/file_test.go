package services

import (
	"context"
	"net/http"
	"testing"

	"9itchi-resume-api/models/db"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fid = "44444444-4444-4444-4444-444444444444"

func newFileSvc() (FileService, *mocks.ResumeRepo, *mocks.FileRepo) {
	r, f := &mocks.ResumeRepo{}, &mocks.FileRepo{}
	return NewFileService(f, r), r, f
}

func TestFileServiceOwnership(t *testing.T) {
	svc, resumes, files := newFileSvc()
	resumes.ExistsFn = func(context.Context, string, string) error { return repositories.ErrNotFound }
	files.GetFn = func(context.Context, string, string) (*db.ResumeFile, error) {
		t.Fatal("file repo must not be reached for a foreign resume")
		return nil, nil
	}

	_, apiErr := svc.ListFiles(context.Background(), uid, rid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	_, apiErr = svc.DownloadFile(context.Background(), uid, rid, fid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	apiErr = svc.DeleteFile(context.Background(), uid, rid, fid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}

func TestFileServiceInvalidIDs(t *testing.T) {
	svc, _, _ := newFileSvc()
	_, apiErr := svc.DownloadFile(context.Background(), uid, rid, "nope")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
}

func TestFileServiceDownloadSanitizesName(t *testing.T) {
	svc, _, files := newFileSvc()
	files.GetFn = func(context.Context, string, string) (*db.ResumeFile, error) {
		return &db.ResumeFile{Filename: "a\"b\r\n.pdf", ContentType: "application/pdf", Data: []byte("%PDF-")}, nil
	}
	got, apiErr := svc.DownloadFile(context.Background(), uid, rid, fid)
	require.Nil(t, apiErr)
	assert.Equal(t, "a_b__.pdf", got.Filename)
	assert.Equal(t, []byte("%PDF-"), got.Data)
}

func TestFileServiceListAndDelete(t *testing.T) {
	svc, _, files := newFileSvc()
	files.ListByResumeFn = func(context.Context, string) ([]db.ResumeFileMeta, error) {
		return []db.ResumeFileMeta{{ID: fid, Kind: "export"}}, nil
	}
	got, apiErr := svc.ListFiles(context.Background(), uid, rid)
	require.Nil(t, apiErr)
	require.Len(t, got.Items, 1)

	files.DeleteFn = func(context.Context, string, string) error { return repositories.ErrNotFound }
	apiErr = svc.DeleteFile(context.Background(), uid, rid, fid)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}
