package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/myjobmatch/backend/agent"
	"github.com/myjobmatch/backend/models"
)

// CVHandler handles CV parsing requests
type CVHandler struct {
	agent *agent.JobAgent
}

// NewCVHandler creates a new CV handler
func NewCVHandler(jobAgent *agent.JobAgent) *CVHandler {
	return &CVHandler{
		agent: jobAgent,
	}
}

// ParseCV parses a CV and extracts profile information
// @Summary Parse CV
// @Description Parse a CV file or text and extract structured profile information using AI
// @Tags CV
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Param request body models.CVParseRequest false "CV parse request (JSON)"
// @Param cv_file formData file false "CV file to parse"
// @Param cv_text formData string false "CV text content"
// @Success 200 {object} models.CVParseResponse "Parsed CV profile"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Parsing failed"
// @Router /parse-cv [post]
func (h *CVHandler) ParseCV(c *gin.Context) {
	var cvText string

	contentType := c.ContentType()

	if strings.Contains(contentType, "multipart/form-data") {
		// Handle file upload
		file, header, err := c.Request.FormFile("cv_file")
		if err != nil {
			// Try text field
			cvText = c.PostForm("cv_text")
		} else {
			defer file.Close()

			buf := new(bytes.Buffer)
			if _, err := io.Copy(buf, file); err != nil {
				c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error: "Failed to read CV file",
					Code:  http.StatusBadRequest,
				})
				return
			}
			cvText = buf.String()
			log.Printf("[CVHandler] Received CV file: %s", header.Filename)
		}
	} else {
		// Handle JSON request
		var req models.CVParseRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error: "Invalid request body",
				Code:  http.StatusBadRequest,
			})
			return
		}
		cvText = req.CVText
	}

	if cvText == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "CV text or file is required",
			Code:  http.StatusBadRequest,
		})
		return
	}

	// Parse CV using the agent
	input := agent.SearchJobsInput{
		CVText: cvText,
	}

	// We only need the profile parsing part
	output, err := h.agent.SearchJobs(c.Request.Context(), agent.SearchJobsInput{
		CVText: cvText,
		Query:  "any job", // Minimal query to trigger profile building
	})
	if err != nil {
		log.Printf("[CVHandler] ParseCV error: %v", err)

		// Try to return partial result if we have a profile
		if output != nil && output.Profile != nil {
			c.JSON(http.StatusOK, models.CVParseResponse{
				Profile: *output.Profile,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "CV parsing failed",
			Code:    http.StatusInternalServerError,
			Details: err.Error(),
		})
		return
	}

	_ = input // Suppress unused variable

	if output.Profile == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to extract profile from CV",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, models.CVParseResponse{
		Profile: *output.Profile,
	})
}
