//go:build integration

package repositories

import (
	"context"
	"strings"
	"testing"
	"time"

	"9itchi-resume-api/models/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteRepository(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewRewriteRepository(store)
	const user, other, cv = "00000000-0000-4000-8000-0000000000b1", "00000000-0000-4000-8000-0000000000b2", "00000000-0000-4000-8000-0000000000b3"
	hash := strings.Repeat("d", 64)

	_, err := repo.Find(ctx, user, hash)
	assert.ErrorIs(t, err, ErrNotFound)

	w, err := repo.Save(ctx, &db.AIRewrite{UserID: user, ResumeID: cv, InputHash: hash, Suggestions: []string{"Développé un portail en React."}, Warnings: []string{"new_numbers"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Développé un portail en React."}, w.Suggestions)
	assert.Equal(t, []string{"new_numbers"}, w.Warnings)

	again, err := repo.Save(ctx, &db.AIRewrite{UserID: user, ResumeID: cv, InputHash: hash, Suggestions: []string{"autre"}})
	require.NoError(t, err)
	assert.Equal(t, w.ID, again.ID, "the same text keeps the first proposal")
	assert.Equal(t, w.Suggestions, again.Suggestions)

	got, err := repo.Find(ctx, user, hash)
	require.NoError(t, err)
	assert.Equal(t, w.ID, got.ID)
	assert.Equal(t, []string{"new_numbers"}, got.Warnings, "warnings stored with the answer")
	_, err = repo.Find(ctx, other, hash)
	assert.ErrorIs(t, err, ErrNotFound, "another student sees nothing")

	none, err := repo.Save(ctx, &db.AIRewrite{UserID: other, ResumeID: cv, InputHash: hash, Suggestions: []string{"x"}})
	require.NoError(t, err)
	assert.Equal(t, []string{}, none.Warnings, "nil warnings stored as []")

	n, err := repo.CountSince(ctx, user, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = repo.CountSince(ctx, user, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Zero(t, n)
}
