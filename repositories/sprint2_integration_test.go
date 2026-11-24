//go:build integration

package repositories

import (
	"context"
	"testing"

	"9itchi-resume-api/models/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const u1 = "00000000-0000-0000-0000-000000000001"

func TestParseStatusLifecycle(t *testing.T) {
	store := newTestStore(t)
	repo := NewResumeRepository(store)
	ctx := context.Background()

	r, err := repo.Create(ctx, u1, "cv", nil, []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, db.ParseNone, r.ParseStatus)
	assert.Nil(t, r.Language)

	ok, err := repo.ApplyParsed(ctx, r.ID, []byte(`{"summary":"x"}`), "fr")
	require.NoError(t, err)
	assert.False(t, ok, "only a pending resume accepts a result")

	require.NoError(t, repo.SetParseStatus(ctx, r.ID, db.ParsePending))
	ok, err = repo.ApplyParsed(ctx, r.ID, []byte(`{"summary":"x"}`), "fr")
	require.NoError(t, err)
	assert.True(t, ok)
	got, err := repo.Get(ctx, u1, r.ID)
	require.NoError(t, err)
	assert.Equal(t, db.ParseDone, got.ParseStatus)
	assert.Equal(t, "fr", *got.Language)
	assert.JSONEq(t, `{"summary":"x"}`, string(got.Content))

	// the user edits while a second reading is pending: the late result is discarded
	require.NoError(t, repo.SetParseStatus(ctx, r.ID, db.ParsePending))
	_, err = repo.Update(ctx, u1, r.ID, nil, nil, []byte(`{"summary":"mine"}`))
	require.NoError(t, err)
	ok, err = repo.ApplyParsed(ctx, r.ID, []byte(`{"summary":"ai"}`), "fr")
	require.NoError(t, err)
	assert.False(t, ok)
	got, _ = repo.Get(ctx, u1, r.ID)
	assert.JSONEq(t, `{"summary":"mine"}`, string(got.Content))
	assert.Equal(t, db.ParseNone, got.ParseStatus)

	assert.Error(t, repo.SetParseStatus(ctx, r.ID, "bogus"), "CHECK constraint")
	assert.ErrorIs(t, repo.SetParseStatus(ctx, "00000000-0000-0000-0000-0000000000ff", db.ParseFailed), ErrNotFound)
}

func TestCountsForQuotas(t *testing.T) {
	store := newTestStore(t)
	resumes := NewResumeRepository(store)
	files := NewFileRepository(store)
	ctx := context.Background()

	n, err := resumes.CountByUser(ctx, u1)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	a, err := resumes.Create(ctx, u1, "a", nil, []byte(`{}`))
	require.NoError(t, err)
	b, err := resumes.Create(ctx, u1, "b", nil, []byte(`{}`))
	require.NoError(t, err)
	_, err = resumes.Create(ctx, "00000000-0000-0000-0000-000000000002", "other", nil, []byte(`{}`))
	require.NoError(t, err)
	n, _ = resumes.CountByUser(ctx, u1)
	assert.Equal(t, 2, n)

	add := func(resumeID, kind string) {
		_, err := files.Create(ctx, &db.ResumeFile{ResumeID: resumeID, Kind: kind, Format: "pdf", SizeBytes: 3,
			SHA256: "0000000000000000000000000000000000000000000000000000000000000000", Data: []byte("pdf")})
		require.NoError(t, err)
	}
	add(b.ID, "original")
	require.NoError(t, files.RecordDownload(ctx, u1, a.ID, "pdf"))
	require.NoError(t, files.RecordDownload(ctx, u1, b.ID, "pdf"))
	n, err = files.CountExportsByUser(ctx, u1)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	require.NoError(t, resumes.Delete(ctx, u1, a.ID))
	n, _ = files.CountExportsByUser(ctx, u1)
	assert.Equal(t, 2, n, "deleting a CV does not give the free download back")
}
