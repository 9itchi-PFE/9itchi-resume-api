package handlers

import (
	"mime"
	"net/http"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
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
