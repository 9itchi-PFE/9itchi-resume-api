package handlers

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"unicode"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ListFiles: GET /resumes/:id/files (user) file metadata, never the blob.
func ListFiles(svc services.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListFiles(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DownloadFile: GET /resumes/:id/files/:fileId (user) stream the stored PDF/DOCX blob.
func DownloadFile(svc services.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.DownloadFile(c.Request.Context(), c.GetString("user_id"), c.Param("id"), c.Param("fileId"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		sendFile(c, result)
	}
}

// DeleteFile: DELETE /resumes/:id/files/:fileId (user) delete a stored file.
func DeleteFile(svc services.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiErr := svc.DeleteFile(c.Request.Context(), c.GetString("user_id"), c.Param("id"), c.Param("fileId")); apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// sendFile streams a blob as an attachment. The filename was sanitized by the service.
func sendFile(c *gin.Context, f *api.FileContent) {
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Filename}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, f.ContentType, f.Data)
}

// sendNamedFile streams a blob whose name may hold any script (a CV title in Arabic, accents): the UTF-8 name in
// filename* (RFC 6266/5987) and an ASCII filename fallback for clients that only read that one.
func sendNamedFile(c *gin.Context, f *api.FileContent) {
	c.Header("Content-Disposition", attachmentDisposition(f.Filename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, f.ContentType, f.Data)
}

// attachmentDisposition is: attachment; filename="<ascii>"; filename*=UTF-8 and two quotes, then the percent-encoded name.
func attachmentDisposition(name string) string {
	var enc strings.Builder
	for _, b := range []byte(name) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.IndexByte("!#$&+-.^_`|~", b) >= 0 {
			enc.WriteByte(b)
		} else {
			fmt.Fprintf(&enc, "%%%02X", b)
		}
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiFilename(name), enc.String())
}

var stripMarks = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// asciiFilename drops accents and turns other non-ASCII runes, quotes and backslashes into '_' (one per run).
// A stem left without any ASCII letter or digit (an Arabic title) becomes "CV".
func asciiFilename(name string) string {
	plain, _, _ := transform.String(stripMarks, name)
	var b strings.Builder
	alnum := false
	for _, r := range plain {
		ok := r >= 0x20 && r < 0x7f && r != '"' && r != '\\'
		switch {
		case ok:
			b.WriteRune(r)
		case !strings.HasSuffix(b.String(), "_"):
			b.WriteByte('_')
		}
	}
	out := b.String()
	ext := path.Ext(out)
	for _, r := range strings.TrimSuffix(out, ext) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			alnum = true
			break
		}
	}
	if !alnum {
		return "CV" + ext
	}
	return out
}
