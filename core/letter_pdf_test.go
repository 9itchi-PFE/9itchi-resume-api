package core

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"9itchi-resume-api/models/api"

	"github.com/ledongthuc/pdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var letterDay = time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)

func letterPages(t *testing.T, data []byte) int {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	return r.NumPage()
}

func TestRenderLetterLatin(t *testing.T) {
	contact := &api.Contact{Name: "Inès Mansour", Email: "ines@mail.tn", Phone: "+216 20 000 000", Location: "Tunis"}
	for lang, want := range map[string][]string{
		"fr": {"Objet", "Stage PFE", "28 novembre 2026", "Je vous écris."},
		"en": {"Subject:", "Stage PFE", "28 November 2026", "Je vous écris."},
	} {
		data, err := RenderLetter(context.Background(), LetterInput{Language: lang, Subject: "Stage PFE", Body: "Madame,\n\nJe vous écris.",
			Contact: contact, Style: api.DefaultCVStyle(), Now: letterDay})
		require.NoError(t, err)
		require.True(t, bytes.HasPrefix(data, []byte("%PDF-")))
		text, err := ExtractText("pdf", data)
		require.NoError(t, err)
		for _, w := range append(want, "Inès Mansour", "ines@mail.tn", "Tunis") {
			assert.Contains(t, text, w, lang)
		}
		assert.Equal(t, 1, letterPages(t, data))
	}
}

func TestRenderLetterLongBodyBreaksPages(t *testing.T) {
	body := strings.Repeat("Une phrase assez longue pour remplir la ligne de la lettre de motivation.\n", 120)
	data, err := RenderLetter(context.Background(), LetterInput{Language: "fr", Body: body, Now: letterDay})
	require.NoError(t, err)
	assert.Greater(t, letterPages(t, data), 1)

	ar := strings.Repeat("أنا طالب في السنة الأخيرة وأبحث عن تربص مشروع ختم الدراسات PFE 2026 في شركتكم.\n", 120)
	data, err = RenderLetter(context.Background(), LetterInput{Language: "ar", Body: ar, Now: letterDay})
	require.NoError(t, err)
	assert.Greater(t, letterPages(t, data), 1)
}

func TestRenderLetterArabic(t *testing.T) {
	data, err := RenderLetter(context.Background(), LetterInput{Language: "ar", Subject: "طلب تربص", Body: "السلام عليكم،\nأرغب في مشروع ختم الدراسات PFE 2026.",
		Contact: &api.Contact{Name: "محمد علي", Email: "m@mail.tn"}, Now: letterDay})
	require.NoError(t, err)
	text, err := ExtractText("pdf", data)
	require.NoError(t, err)
	assert.NotContains(t, text, "?", "Arabic is drawn, not replaced by '?'")
	assert.Contains(t, text, "PFE 2026")
	assert.Contains(t, text, "m@mail.tn")
	assert.Equal(t, "28 نوفمبر 2026", letterDate("ar", letterDay))
	assert.Equal(t, "5 جويلية 2026", letterDate("ar", time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)))
}

func TestRenderLetterCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RenderLetter(ctx, LetterInput{Language: "fr", Body: "x"})
	assert.Error(t, err)
}
