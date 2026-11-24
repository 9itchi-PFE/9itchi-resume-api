//go:build integration

package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimTipsOnceAWeek(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	resumes, tips := NewResumeRepository(store), NewTipRepository(store)

	add := func(user, title string, age time.Duration) string {
		r, err := resumes.Create(ctx, user, title, nil, []byte(`{}`))
		require.NoError(t, err)
		_, err = store.Conn(ctx).Exec(ctx, `UPDATE resumes SET updated_at = now() - $2::interval WHERE id = $1`, r.ID, age.String())
		require.NoError(t, err)
		return r.ID
	}
	add(userA, "older", 30*24*time.Hour)
	latestA := add(userA, "latest", 5*24*time.Hour)
	add(userB, "being worked on", time.Hour)
	const userC = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	reading := add(userC, "being read", 5*24*time.Hour)
	require.NoError(t, resumes.SetParseStatus(ctx, reading, "pending"))

	idle, weekAgo := time.Now().Add(-3*24*time.Hour), time.Now().Add(-7*24*time.Hour)
	got, err := tips.Claim(ctx, idle, weekAgo, 10)
	require.NoError(t, err)
	require.Len(t, got, 1, "the CV changed an hour ago and the one still being read are left alone")
	assert.Equal(t, userA, got[0].UserID)
	assert.Equal(t, latestA, got[0].ResumeID, "the CV changed last")

	again, err := tips.Claim(ctx, idle, weekAgo, 10)
	require.NoError(t, err)
	assert.Empty(t, again, "tipped a moment ago")

	// A week later the student is due again; meanwhile the other CV became the latest one.
	_, err = store.Conn(ctx).Exec(ctx, `UPDATE cv_tips SET sent_at = now() - interval '8 days'`)
	require.NoError(t, err)
	_, err = store.Conn(ctx).Exec(ctx, `UPDATE resumes SET updated_at = now() - interval '4 days' WHERE title = 'older'`)
	require.NoError(t, err)
	again, err = tips.Claim(ctx, idle, weekAgo, 10)
	require.NoError(t, err)
	require.Len(t, again, 1)
	assert.NotEqual(t, latestA, again[0].ResumeID)

	// A released claim is due at once; the limit holds.
	require.NoError(t, tips.Release(ctx, userA))
	_, err = store.Conn(ctx).Exec(ctx, `UPDATE resumes SET updated_at = now() - interval '4 days'`)
	require.NoError(t, err)
	again, err = tips.Claim(ctx, idle, weekAgo, 1)
	require.NoError(t, err)
	require.Len(t, again, 1)
	rest, err := tips.Claim(ctx, idle, weekAgo, 10)
	require.NoError(t, err)
	assert.Len(t, rest, 1, "the other student, once")
}
