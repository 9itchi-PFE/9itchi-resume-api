package services

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"9itchi-resume-api/models"
	"9itchi-resume-api/repositories"
	"9itchi-resume-api/utils"

	"go.uber.org/zap"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool { return uuidRe.MatchString(s) }

func newError(code int, msg string) *models.ApiError {
	return &models.ApiError{Code: code, Error: errors.New(msg)}
}

// repoError maps repository errors to API errors: not found -> 404, conflict -> 409,
// anything else -> 500 with a generic message (details only in the log).
func repoError(err error, notFoundMsg string) *models.ApiError {
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		return newError(http.StatusNotFound, notFoundMsg)
	case errors.Is(err, repositories.ErrConflict):
		return newError(http.StatusConflict, "conflict")
	default:
		utils.Logger.Error("repository error", zap.Error(err))
		return newError(http.StatusInternalServerError, "internal error")
	}
}

// sanitizeFilename keeps letters, digits, space, '.', '_' and '-', replaces the rest with '_',
// drops any directory part and caps the length. It never returns an empty string.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ".")
	if r := []rune(out); len(r) > 150 {
		out = string(r[:150])
	}
	if out == "" {
		return "file"
	}
	return out
}

// cleanStem is a file name without extension made from user text (same rule as email-api's): directory parts
// and a trailing .pdf/.docx/.doc go, letters (any script), marks, digits, space and `_ - . ( )` stay, other runes
// become '_', runs of spaces and underscores shrink to their first rune, ` ._-` are trimmed at both ends and the
// result is cut to 80 runes. Empty gives fallback.
func cleanStem(in, fallback string) string {
	in = strings.ReplaceAll(in, "\\", "/")
	if i := strings.LastIndex(in, "/"); i >= 0 {
		in = in[i+1:]
	}
	in = strings.TrimSpace(in)
	lower := strings.ToLower(in)
	for _, ext := range []string{".pdf", ".docx", ".doc"} {
		if strings.HasSuffix(lower, ext) {
			in = in[:len(in)-len(ext)]
			break
		}
	}
	var b strings.Builder
	var last rune
	for _, r := range in {
		if !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsDigit(r) && !strings.ContainsRune(" _-.()", r) {
			r = '_'
		}
		if (r == ' ' || r == '_') && (last == ' ' || last == '_') {
			continue
		}
		b.WriteRune(r)
		last = r
	}
	const cut = " ._-"
	out := strings.Trim(b.String(), cut)
	if r := []rune(out); len(r) > 80 {
		out = strings.Trim(string(r[:80]), cut)
	}
	if out == "" {
		return fallback
	}
	return out
}

// cvStem names the CV attached to an application: its title as the student wrote it, else "prenom_nom", else "CV".
func cvStem(title, name string) string {
	return cleanStem(title, exportFilename(name, "CV"))
}

// originalExt is the extension of an imported file (".pdf", ".docx"...): its stored name's, else its content type's.
func originalExt(filename, contentType string) string {
	if i := strings.LastIndex(filename, "."); i >= 0 {
		ext := strings.ToLower(filename[i:])
		if len(ext) >= 2 && len(ext) <= 6 && strings.IndexFunc(ext[1:], func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9')
		}) < 0 {
			return ext
		}
	}
	switch contentType {
	case contentTypeDOCX:
		return ".docx"
	case "application/msword":
		return ".doc"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	}
	return ".pdf"
}
