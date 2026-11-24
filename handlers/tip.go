package handlers

import (
	"net/http"

	"9itchi-resume-api/models"
	"9itchi-resume-api/services"

	"github.com/gin-gonic/gin"
)

// SendTips: POST /internal/tips/send (internal) raise the weekly advice on the CVs that still have points to correct.
func SendTips(svc services.TipService) gin.HandlerFunc {
	return func(c *gin.Context) {
		res, apiErr := svc.Send(c.Request.Context())
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}
