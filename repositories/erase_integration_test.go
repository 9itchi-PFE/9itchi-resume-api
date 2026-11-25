//go:build integration

package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Everything of the deleted user goes, in one transaction, and nothing of anyone else.
func TestEraseUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewEraseRepository(store)
	const gone, kept = "00000000-0000-4000-8000-0000000000e1", "00000000-0000-4000-8000-0000000000e2"
	const count = `SELECT (SELECT count(*) FROM resumes WHERE user_id = $1) + (SELECT count(*) FROM resume_downloads WHERE user_id = $1) + (SELECT count(*) FROM cv_tips WHERE user_id = $1) + (SELECT count(*) FROM resume_files f JOIN resumes r ON r.id = f.resume_id WHERE r.user_id = $1) + (SELECT count(*) FROM ai_rewrites WHERE user_id = $1)`
	left := func(id string) int {
		var n int
		require.NoError(t, store.Conn(ctx).QueryRow(ctx, count, id).Scan(&n))
		return n
	}
	for _, id := range []string{gone, kept} {
		for _, q := range []string{
			`WITH r AS (INSERT INTO resumes (user_id) VALUES ($1) RETURNING id) INSERT INTO resume_files (resume_id, kind, format, size_bytes, sha256, data) SELECT id, 'original', 'pdf', 3, repeat('a', 64), 'abc' FROM r`,
			`INSERT INTO resume_downloads (user_id, format) VALUES ($1, 'pdf')`,
			`INSERT INTO cv_tips (user_id, resume_id) SELECT $1, id FROM resumes WHERE user_id = $1`,
			`INSERT INTO ai_rewrites (user_id, resume_id, input_hash) SELECT $1, id, repeat('b', 64) FROM resumes WHERE user_id = $1`,
		} {
			_, err := store.Conn(ctx).Exec(ctx, q, id)
			require.NoError(t, err, q)
		}
	}
	require.Equal(t, 5, left(gone))

	rows, err := repo.EraseUser(ctx, gone)
	require.NoError(t, err)
	assert.Positive(t, rows)
	assert.Zero(t, left(gone))
	assert.Equal(t, 5, left(kept), "another student keeps everything")

	rows, err = repo.EraseUser(ctx, gone)
	require.NoError(t, err, "erasing again is not an error")
	assert.Zero(t, rows)
}
