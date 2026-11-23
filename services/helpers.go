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
