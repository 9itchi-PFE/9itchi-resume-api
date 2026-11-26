package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwoColumnTemplatesMigrationEmbedded(t *testing.T) {
	b, err := FS.ReadFile("0008_two_column_templates.sql")
	require.NoError(t, err)
	sql := string(b)
	assert.Contains(t, sql, "-- +goose Up")
	assert.Contains(t, sql, "-- +goose Down")
	for _, layout := range []string{"'sidebar'", "'split'", "'banner'"} {
		assert.Contains(t, sql, layout)
	}
}
