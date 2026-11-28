package core

import (
	"testing"

	"github.com/go-pdf/fpdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmiriHasPresentationForms(t *testing.T) {
	for _, file := range []string{"fonts/Amiri-Regular.ttf", "fonts/Amiri-Bold.ttf"} {
		ttf, err := fpdf.TtfParse(file)
		require.NoError(t, err)
		for _, forms := range arabicForms {
			for _, r := range forms {
				if r != 0 {
					assert.NotZero(t, ttf.Chars[uint16(r)], "%s lacks %U", file, r)
				}
			}
		}
		for _, lig := range lamAlef {
			assert.NotZero(t, ttf.Chars[uint16(lig[0])], "%s lacks %U", file, lig[0])
			assert.NotZero(t, ttf.Chars[uint16(lig[1])], "%s lacks %U", file, lig[1])
		}
	}
}

func TestShapeArabic(t *testing.T) {
	// باب: initial ba, final alef (alef never joins forward), isolated ba.
	assert.Equal(t, "\uFE91\uFE8E\uFE8F", shapeArabic("باب"))
	// بيت: initial, medial, final.
	assert.Equal(t, "\uFE91\uFEF4\uFE96", shapeArabic("بيت"))
	// Non-joining letters break the word: ورد = isolated waw, isolated ra, isolated dal.
	assert.Equal(t, "\uFEED\uFEAD\uFEA9", shapeArabic("ورد"))
	// لا isolated, and after a joining letter (سلام) the final lam-alef.
	assert.Equal(t, "\uFEFB", shapeArabic("لا"))
	assert.Equal(t, "\uFEB3\uFEFC\uFEE1", shapeArabic("سلام"))
	assert.Equal(t, "\uFEF7", shapeArabic("لأ"))
	// ة and ى are final when joined, ء stays isolated.
	assert.Equal(t, "\uFEDF\uFEE4\uFE94", shapeArabic("لمة"))
	assert.Equal(t, "\uFE80", shapeArabic("ء"))
	// Harakat are transparent: the letters around them still join.
	assert.Equal(t, "\uFE91\u064E\uFE8E\uFE8F", shapeArabic("بَاب"))
	assert.Equal(t, "PFE 2026", shapeArabic("PFE 2026"))
}

func TestVisualLine(t *testing.T) {
	assert.Equal(t, "تبا", visualLine("ابت"))
	got := visualLine("مشروع PFE 2026 هنا")
	assert.Contains(t, got, "PFE 2026", "a Latin run keeps its order inside Arabic")
	assert.Equal(t, "انه PFE 2026 عورشم", got)
	assert.Equal(t, "(ب) ا", visualLine("ا (ب)"), "brackets are mirrored in a right-to-left run")
	assert.True(t, hasArabic("Stage مرحبا"))
	assert.False(t, hasArabic("Stage PFE"))
}
