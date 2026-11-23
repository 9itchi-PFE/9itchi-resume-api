//go:build integration

package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"9itchi-resume-api/models/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	userB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func TestTemplatesSeededAndActiveOnly(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewTemplateRepository(store)

	items, err := repo.ListActive(ctx)
	require.NoError(t, err)
	var layouts []string
	for _, it := range items {
		layouts = append(layouts, it.Layout)
	}
	assert.ElementsMatch(t, []string{"classic", "modern", "compact"}, layouts)

	off := false
	_, err = repo.Update(ctx, items[0].ID, TemplatePatch{Active: &off})
	require.NoError(t, err)
	items2, err := repo.ListActive(ctx)
	require.NoError(t, err)
	assert.Len(t, items2, 2)

	_, err = repo.Create(ctx, items[1].Name, "classic", "", true)
	assert.ErrorIs(t, err, ErrConflict)
	_, err = repo.Get(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestResumeCRUDScopedByUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewResumeRepository(store)

	r, err := repo.Create(ctx, userA, "CV", nil, []byte(`{"summary":"hi"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"summary":"hi"}`, string(r.Content))

	_, err = repo.Get(ctx, userB, r.ID)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorIs(t, repo.Exists(ctx, userB, r.ID), ErrNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, userB, r.ID), ErrNotFound)
	_, err = repo.Update(ctx, userB, r.ID, ptr("x"), nil, nil)
	assert.ErrorIs(t, err, ErrNotFound)

	// partial update keeps content
	title := "New"
	u, err := repo.Update(ctx, userA, r.ID, &title, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "New", u.Title)
	assert.JSONEq(t, `{"summary":"hi"}`, string(u.Content))

	// template switch keeps content
	tpls, err := NewTemplateRepository(store).ListActive(ctx)
	require.NoError(t, err)
	u, err = repo.Update(ctx, userA, r.ID, nil, &tpls[0].ID, nil)
	require.NoError(t, err)
	require.NotNil(t, u.TemplateID)
	assert.JSONEq(t, `{"summary":"hi"}`, string(u.Content))

	u, err = repo.Update(ctx, userA, r.ID, nil, nil, []byte(`{"summary":"new"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"summary":"new"}`, string(u.Content))

	items, total, err := repo.List(ctx, userA, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, items, 1)
	_, total, err = repo.List(ctx, userB, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total)

	require.NoError(t, repo.Delete(ctx, userA, r.ID))
	_, err = repo.Get(ctx, userA, r.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestVersionsInTransaction(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewResumeRepository(store)
	r, err := repo.Create(ctx, userA, "CV", nil, []byte(`{"summary":"v"}`))
	require.NoError(t, err)

	snapshot := func() error {
		return store.WithTx(ctx, func(ctx context.Context) error {
			no, content, err := repo.BumpVersion(ctx, userA, r.ID)
			if err != nil {
				return err
			}
			_, err = repo.InsertVersion(ctx, r.ID, no, content)
			return err
		})
	}
	require.NoError(t, snapshot())
	require.NoError(t, snapshot())

	got, err := repo.Get(ctx, userA, r.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)

	vs, err := repo.ListVersions(ctx, r.ID)
	require.NoError(t, err)
	require.Len(t, vs, 2)
	assert.Equal(t, 2, vs[0].VersionNo)

	// wrong owner: nothing bumped, error returned
	err = store.WithTx(ctx, func(ctx context.Context) error {
		_, _, err := repo.BumpVersion(ctx, userB, r.ID)
		return err
	})
	assert.ErrorIs(t, err, ErrNotFound)

	// rollback: a failing insert (duplicate version_no) undoes the bump
	err = store.WithTx(ctx, func(ctx context.Context) error {
		if _, _, err := repo.BumpVersion(ctx, userA, r.ID); err != nil {
			return err
		}
		_, err := repo.InsertVersion(ctx, r.ID, 1, []byte(`{}`))
		return err
	})
	assert.ErrorIs(t, err, ErrConflict)
	got, err = repo.Get(ctx, userA, r.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)

	// cascade
	require.NoError(t, repo.Delete(ctx, userA, r.ID))
	vs, err = repo.ListVersions(ctx, r.ID)
	require.NoError(t, err)
	assert.Empty(t, vs)
}

func TestFileBlobRoundtripAndMetadataOnly(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	resumes := NewResumeRepository(store)
	files := NewFileRepository(store)
	r, err := resumes.Create(ctx, userA, "CV", nil, []byte(`{}`))
	require.NoError(t, err)

	data := append([]byte("%PDF-1.4\n"), 0x00, 0xff, 0x10, 0x80)
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	meta, err := files.Create(ctx, &db.ResumeFile{
		ResumeID: r.ID, Kind: "original", Format: "pdf", Filename: "cv.pdf",
		ContentType: "application/pdf", SizeBytes: int64(len(data)), SHA256: hexSum, Data: data,
	})
	require.NoError(t, err)
	assert.Equal(t, hexSum, meta.SHA256)

	got, err := files.Get(ctx, r.ID, meta.ID)
	require.NoError(t, err)
	assert.Equal(t, data, got.Data)
	back := sha256.Sum256(got.Data)
	assert.Equal(t, hexSum, hex.EncodeToString(back[:]))
	assert.Equal(t, int64(len(data)), got.SizeBytes)

	// list returns metadata only (the type has no data field); verify via SQL that data is intact
	// and that the list query text never selects it.
	list, err := files.ListByResume(ctx, r.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, meta.ID, list[0].ID)
	assert.NotContains(t, fileMetaColumns, "data")

	// latest export
	_, err = files.LatestExport(ctx, r.ID, "pdf")
	assert.ErrorIs(t, err, ErrNotFound)
	exp1 := &db.ResumeFile{ResumeID: r.ID, Kind: "export", Format: "pdf", Filename: "a.pdf", ContentType: "application/pdf", SizeBytes: 3, SHA256: hexSum, Data: []byte("one")}
	_, err = files.Create(ctx, exp1)
	require.NoError(t, err)
	exp2 := *exp1
	exp2.Data = []byte("two")
	_, err = files.Create(ctx, &exp2)
	require.NoError(t, err)
	latest, err := files.LatestExport(ctx, r.ID, "pdf")
	require.NoError(t, err)
	assert.Equal(t, []byte("two"), latest.Data)

	// wrong resume id cannot read another resume's file
	other, err := resumes.Create(ctx, userB, "other", nil, []byte(`{}`))
	require.NoError(t, err)
	_, err = files.Get(ctx, other.ID, meta.ID)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorIs(t, files.Delete(ctx, other.ID, meta.ID), ErrNotFound)

	// constraints
	_, err = files.Create(ctx, &db.ResumeFile{ResumeID: r.ID, Kind: "bogus", Format: "pdf", SizeBytes: 1, SHA256: hexSum, Data: []byte("x")})
	assert.Error(t, err)

	require.NoError(t, files.Delete(ctx, r.ID, meta.ID))
	_, err = files.Get(ctx, r.ID, meta.ID)
	assert.ErrorIs(t, err, ErrNotFound)

	// cascade on resume delete
	require.NoError(t, resumes.Delete(ctx, userA, r.ID))
	list, err = files.ListByResume(ctx, r.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestUploadStyleTransactionRollsBack(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	resumes := NewResumeRepository(store)
	files := NewFileRepository(store)

	err := store.WithTx(ctx, func(ctx context.Context) error {
		r, err := resumes.Create(ctx, userA, "CV", nil, []byte(`{}`))
		if err != nil {
			return err
		}
		// invalid sha256 length violates the CHECK constraint
		_, err = files.Create(ctx, &db.ResumeFile{ResumeID: r.ID, Kind: "original", Format: "pdf", SizeBytes: 1, SHA256: "short", Data: []byte("x")})
		return err
	})
	require.Error(t, err)
	_, total, err := resumes.List(ctx, userA, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, total, "resume must be rolled back with the failed file insert")
}

func ptr[T any](v T) *T { return &v }
