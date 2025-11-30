package models

// SearchJobsRequest represents the API request for job search
// @Description Job search request with CV and/or query
type SearchJobsRequest struct {
	CVText  string          `json:"cvText,omitempty" form:"cv_text" example:"John Doe\nSoftware Engineer with 5 years experience..."`
	Query   string          `json:"query,omitempty" form:"query" example:"golang developer jakarta"`
	Filters JobSearchFilter `json:"filters,omitempty" form:"filters"`
	SaveCV  bool            `json:"saveCV,omitempty" form:"save_cv" example:"false"` // Save CV to profile if authenticated
}

// SearchJobsResponse represents the API response for job search
// @Description Job search results with ranked jobs and extracted profile
type SearchJobsResponse struct {
	Results      []RankedJob  `json:"results"`
	Profile      *UserProfile `json:"profile,omitempty"`
	TotalResults int          `json:"total_results" example:"10"`
	Message      string       `json:"message,omitempty" example:"Found 10 matching jobs"`
	CVSaved      bool         `json:"cvSaved,omitempty"` // True if CV was saved to profile
}

// ErrorResponse represents an API error response
// @Description Standard error response
type ErrorResponse struct {
	Error   string `json:"error" example:"Invalid request body"`
	Code    int    `json:"code" example:"400"`
	Details string `json:"details,omitempty" example:"email is required"`
}

// HealthResponse represents health check response
// @Description Server health status
type HealthResponse struct {
	Status    string `json:"status" example:"healthy"`
	Version   string `json:"version" example:"1.0.0"`
	Timestamp string `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// CVParseRequest represents request to parse CV
// @Description CV parsing request
type CVParseRequest struct {
	CVText string `json:"cv_text" example:"John Doe\nSoftware Engineer\nExperience: 5 years in Go, Python..."`
}

// CVParseResponse represents response from CV parsing
// @Description Parsed CV profile information
type CVParseResponse struct {
	Profile UserProfile `json:"profile"`
	Raw     string      `json:"raw,omitempty"` // Raw Gemini response for debugging
}

// WebSearchRequest represents request for web search tool
type WebSearchRequest struct {
	Query   string          `json:"query"`
	Profile UserProfile     `json:"profile,omitempty"`
	Filters JobSearchFilter `json:"filters,omitempty"`
}

// WebSearchResponse represents response from web search tool
type WebSearchResponse struct {
	URLs    []string          `json:"urls"`
	Results []JobSearchResult `json:"results,omitempty"`
}

// FetchPageRequest represents request to fetch a page
type FetchPageRequest struct {
	URL string `json:"url"`
}

// FetchPageResponse represents response from page fetch
type FetchPageResponse struct {
	HTML  string `json:"html"`
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

// ExtractJobRequest represents request to extract job from HTML
type ExtractJobRequest struct {
	HTML string `json:"html"`
	URL  string `json:"url"`
}

// ExtractJobResponse represents response from job extraction
type ExtractJobResponse struct {
	Job   *JobPosting `json:"job,omitempty"`
	Error string      `json:"error,omitempty"`
}

// ScoreJobRequest represents request to score a job match
type ScoreJobRequest struct {
	Profile UserProfile `json:"profile"`
	Job     JobPosting  `json:"job"`
}

// ScoreJobResponse represents response from job scoring
type ScoreJobResponse struct {
	MatchScore  int    `json:"match_score"`
	MatchReason string `json:"match_reason"`
}
