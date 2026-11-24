package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"9itchi-resume-api/models"
	"9itchi-resume-api/models/api"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
)

// multipartOverhead is allowed on top of the file limit for boundaries and headers.
const multipartOverhead = 1 << 20

// UploadResume: POST /resumes/upload (user) multipart field "file", PDF or DOCX, max 10 MB.
func UploadResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Cap the request body before the multipart parser reads anything.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, api.MaxUploadBytes+multipartOverhead)
		header, err := c.FormFile("file")
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				models.FormatError(c, &models.ApiError{Code: http.StatusRequestEntityTooLarge, Error: errors.New("file too large (max 10 MB)")})
				return
			}
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("multipart field 'file' is required")})
			return
		}
		f, err := header.Open()
		if err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("could not read file")})
			return
		}
		defer f.Close()
		result, apiErr := svc.UploadResume(c.Request.Context(), c.GetString("user_id"), header.Filename, header.Size, f)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// CreateResume: POST /resumes (user) create resume from structured content.
func CreateResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.CreateResumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.CreateResume(c.Request.Context(), c.GetString("user_id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// ListResumes: GET /resumes (user) list own resumes, paginated (page, page_size).
func ListResumes(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.Query("page"))
		pageSize, _ := strconv.Atoi(c.Query("page_size"))
		result, apiErr := svc.ListResumes(c.Request.Context(), c.GetString("user_id"), page, pageSize)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// GetResume: GET /resumes/:id (user) resume detail.
func GetResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// UpdateResume: PUT /resumes/:id (user) edit sections / switch template.
func UpdateResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.UpdateResumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.UpdateResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DeleteResume: DELETE /resumes/:id (user) delete resume.
func DeleteResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiErr := svc.DeleteResume(c.Request.Context(), c.GetString("user_id"), c.Param("id")); apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ListVersions: GET /resumes/:id/versions (user) list saved versions.
func ListVersions(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListVersions(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// CreateVersion: POST /resumes/:id/versions (user) snapshot a version.
func CreateVersion(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.CreateVersion(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// ExportResume: POST /resumes/:id/export (user) export PDF (docx is unsupported).
func ExportResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.ExportResumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.ExportResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// ATSCheck: POST /resumes/:id/ats-check (user) ATS score and suggestions. Not implemented yet.
func ATSCheck(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.ATSCheckRequest
		if c.Request.ContentLength != 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
				return
			}
		}
		result, apiErr := svc.ATSCheck(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// InternalResumePDF: GET /internal/resumes/:id/pdf (internal) latest exported PDF bytes (email attachments).
func InternalResumePDF(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.InternalResumePDF(c.Request.Context(), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		sendFile(c, result)
	}
}

// GetVersion: GET /resumes/:id/versions/:versionId (user) one saved version with its content.
func GetVersion(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetVersion(c.Request.Context(), c.GetString("user_id"), c.Param("id"), c.Param("versionId"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// RestoreVersion: POST /resumes/:id/versions/:versionId/restore (user) put a saved version back.
func RestoreVersion(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.RestoreVersion(c.Request.Context(), c.GetString("user_id"), c.Param("id"), c.Param("versionId"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// DuplicateResume: POST /resumes/:id/duplicate (user) copy a CV under a new title.
func DuplicateResume(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DuplicateResumeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.DuplicateResume(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}

// UploadPhoto: PUT /resumes/:id/photo (user) multipart field "file", JPEG or PNG, 2 MB max.
func UploadPhoto(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.MaxPhotoBytes+multipartOverhead)
		header, err := c.FormFile("file")
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				models.FormatError(c, &models.ApiError{Code: http.StatusRequestEntityTooLarge, Error: errors.New("photo too large (max 2 MB)")})
				return
			}
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("multipart field 'file' is required")})
			return
		}
		f, err := header.Open()
		if err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: errors.New("could not read file")})
			return
		}
		defer f.Close()
		if apiErr := svc.UploadPhoto(c.Request.Context(), c.GetString("user_id"), c.Param("id"), f); apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// GetPhoto: GET /resumes/:id/photo (user) the CV photo, inline.
func GetPhoto(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GetPhoto(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "private, no-cache")
		c.Data(http.StatusOK, result.ContentType, result.Data)
	}
}

// DeletePhoto: DELETE /resumes/:id/photo (user).
func DeletePhoto(svc services.ResumeService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiErr := svc.DeletePhoto(c.Request.Context(), c.GetString("user_id"), c.Param("id")); apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
