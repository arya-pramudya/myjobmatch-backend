package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/myjobmatch/backend/agent"
	"github.com/myjobmatch/backend/auth"
	"github.com/myjobmatch/backend/models"
	"github.com/myjobmatch/backend/storage"
)

// SearchHandler handles job search requests
type SearchHandler struct {
	agent           *agent.JobAgent
	firestoreClient *storage.FirestoreClient
	storageClient   *storage.CloudStorageClient
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(
	jobAgent *agent.JobAgent,
	firestoreClient *storage.FirestoreClient,
	storageClient *storage.CloudStorageClient,
) *SearchHandler {
	return &SearchHandler{
		agent:           jobAgent,
		firestoreClient: firestoreClient,
		storageClient:   storageClient,
	}
}

// SearchJobs handles job search requests
// @Summary Search for jobs
// @Description Search for jobs using CV file/text and/or search query. Supports multipart/form-data for CV upload. Authentication optional - if authenticated and save_cv=true, CV will be saved to profile.
// @Tags Jobs
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param request body models.SearchJobsRequest false "Search request (JSON)"
// @Param cv_file formData file false "CV file (PDF, DOC, DOCX, TXT) - processed by AI"
// @Param cv_text formData string false "CV text content"
// @Param query formData string false "Search query"
// @Param save_cv formData bool false "Save CV to profile (requires authentication)"
// @Param locations formData []string false "Location filters"
// @Param remote_modes formData []string false "Remote mode filters (remote, hybrid, onsite)"
// @Param job_types formData []string false "Job type filters (full-time, part-time, contract)"
// @Success 200 {object} models.SearchJobsResponse "Search results"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /search-jobs [post]
func (h *SearchHandler) SearchJobs(c *gin.Context) {
	var cvText string
	var cvFileData []byte
	var cvFileName string
	var query string
	var filters models.JobSearchFilter
	var saveCV bool
	var useProfileCV bool

	contentType := c.ContentType()

	if strings.Contains(contentType, "multipart/form-data") {
		// Handle file upload
		cvText, cvFileData, cvFileName, query, filters, saveCV = h.parseMultipartRequest(c)
	} else {
		// Handle JSON request
		var req models.SearchJobsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error: "Invalid request body",
				Code:  http.StatusBadRequest,
			})
			return
		}
		cvText = req.CVText
		query = req.Query
		filters = req.Filters
		saveCV = req.SaveCV
	}

	// Check if user is authenticated
	claims := auth.GetAuthClaims(c)

	// If no CV provided, try to use saved CV from profile
	if claims != nil && cvText == "" && len(cvFileData) == 0 {
		user, err := h.firestoreClient.GetUserByEmail(c.Request.Context(), claims.Email)
		if err == nil && user.CVUrl != "" {
			// Download CV from Cloud Storage
			cvContent, err := h.storageClient.DownloadCV(c.Request.Context(), user.CVUrl)
			if err == nil {
				cvText = string(cvContent)
				useProfileCV = true
				log.Printf("[Handler] Using saved CV for user: %s", claims.Email)
			} else {
				log.Printf("[Handler] Failed to download saved CV: %v", err)
			}
		}
	}

	// Validate that at least one input is provided
	if cvText == "" && len(cvFileData) == 0 && query == "" {
		// If user is logged in but has no CV, provide helpful message
		if claims != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error: "Please provide a search query or upload your CV in your profile",
				Code:  http.StatusBadRequest,
			})
			return
		}
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Please provide a CV file, CV text, or search query",
			Code:  http.StatusBadRequest,
		})
		return
	}

	log.Printf("[Handler] SearchJobs request: query=%q, hasCVText=%v, hasCVFile=%v, useProfileCV=%v, saveCV=%v, filters=%+v",
		query, cvText != "", len(cvFileData) > 0, useProfileCV, saveCV, filters)

	// Execute job search - pass PDF data directly to agent for Gemini multimodal parsing
	input := agent.SearchJobsInput{
		CVText:     cvText,
		CVFileData: cvFileData,
		CVFileName: cvFileName,
		Query:      query,
		Filters:    filters,
	}

	output, err := h.agent.SearchJobs(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler] SearchJobs error: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Job search failed",
			Code:    http.StatusInternalServerError,
			Details: err.Error(),
		})
		return
	}

	// Save CV to profile if authenticated and requested
	var cvSaved bool
	if saveCV && claims != nil && len(cvFileData) > 0 && h.storageClient != nil {
		cvUrl, err := h.storageClient.UploadCVFromBytes(c.Request.Context(), claims.Email, cvFileData, cvFileName)
		if err != nil {
			log.Printf("[Handler] Failed to save CV to profile: %v", err)
		} else {
			// Update user's CV URL in Firestore
			if err := h.firestoreClient.UpdateUserCVUrl(c.Request.Context(), claims.Email, cvUrl); err != nil {
				log.Printf("[Handler] Failed to update CV URL in Firestore: %v", err)
			} else {
				cvSaved = true
				log.Printf("[Handler] CV saved to profile for user: %s", claims.Email)
			}
		}
	}

	response := models.SearchJobsResponse{
		Results:      output.Results,
		Profile:      output.Profile,
		TotalResults: len(output.Results),
		Message:      h.buildResultMessage(output.Stats),
		CVSaved:      cvSaved,
	}

	log.Printf("[Handler] SearchJobs success: returning %d results, cvSaved=%v", len(output.Results), cvSaved)
	c.JSON(http.StatusOK, response)
}

// parseMultipartRequest parses a multipart/form-data request
// Returns: cvText, cvFileData, cvFileName, query, filters, saveCV
func (h *SearchHandler) parseMultipartRequest(c *gin.Context) (string, []byte, string, string, models.JobSearchFilter, bool) {
	var cvText, query, cvFileName string
	var cvFileData []byte
	var filters models.JobSearchFilter
	var saveCV bool

	// Get CV file if present
	file, header, err := c.Request.FormFile("cv_file")
	if err == nil && file != nil {
		defer file.Close()

		// Read file content into bytes
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, file); err == nil {
			cvFileData = buf.Bytes()
			cvFileName = header.Filename
			log.Printf("[Handler] Received CV file: %s, size: %d bytes", header.Filename, header.Size)
		}
	}

	// Get CV text if provided directly
	if textCV := c.PostForm("cv_text"); textCV != "" {
		cvText = textCV
	}

	// Get query
	query = c.PostForm("query")

	// Get save_cv flag
	if saveCVStr := c.PostForm("save_cv"); saveCVStr == "true" || saveCVStr == "1" {
		saveCV = true
	}

	// Parse filters from form
	if locations := c.PostFormArray("locations"); len(locations) > 0 {
		filters.Locations = locations
	} else if loc := c.PostForm("locations"); loc != "" {
		filters.Locations = strings.Split(loc, ",")
	}

	if remoteModes := c.PostFormArray("remote_modes"); len(remoteModes) > 0 {
		filters.RemoteModes = remoteModes
	} else if rm := c.PostForm("remote_modes"); rm != "" {
		filters.RemoteModes = strings.Split(rm, ",")
	}

	if jobTypes := c.PostFormArray("job_types"); len(jobTypes) > 0 {
		filters.JobTypes = jobTypes
	} else if jt := c.PostForm("job_types"); jt != "" {
		filters.JobTypes = strings.Split(jt, ",")
	}

	return cvText, cvFileData, cvFileName, query, filters, saveCV
}

// buildResultMessage creates a human-readable message about the search results
func (h *SearchHandler) buildResultMessage(stats agent.SearchStats) string {
	if stats.JobsReturned == 0 {
		return "No matching jobs found. Try adjusting your search criteria."
	}

	return ""
}

// HealthCheck returns server health status
// @Summary Health check
// @Description Check if the server is running and healthy
// @Tags System
// @Produce json
// @Success 200 {object} models.HealthResponse "Server is healthy"
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Timestamp: c.Request.Header.Get("Date"),
	})
}

// GetTools returns available MCP tools
// @Summary List available tools
// @Description Get a list of all available MCP tools for AI agents
// @Tags Tools
// @Produce json
// @Success 200 {object} map[string]interface{} "List of tools"
// @Router /tools [get]
func (h *SearchHandler) GetTools(c *gin.Context) {
	tools := h.agent.GetToolDefinitions()
	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
	})
}
