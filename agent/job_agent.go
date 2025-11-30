package agent

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/myjobmatch/backend/config"
	"github.com/myjobmatch/backend/gemini"
	"github.com/myjobmatch/backend/models"
	"github.com/myjobmatch/backend/tools"
)

// JobAgent orchestrates the job search process using MCP tools
type JobAgent struct {
	cfg           *config.Config
	geminiClient  *gemini.Client
	searchTool    *tools.SearchWebTool
	fetchTool     *tools.FetchPageTool
	extractTool   *tools.ExtractJobTool
	scoreTool     *tools.ScoreJobTool
	parseCVTool   *tools.ParseCVTool
	toolRegistry  *tools.ToolRegistry
	maxConcurrent int
}

// NewJobAgent creates a new job search agent
func NewJobAgent(ctx context.Context, cfg *config.Config) (*JobAgent, error) {
	// Initialize Gemini client
	geminiClient, err := gemini.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Initialize tools
	searchTool := tools.NewSearchWebTool(cfg)
	fetchTool := tools.NewFetchPageTool(cfg)
	extractTool := tools.NewExtractJobTool(geminiClient)
	scoreTool := tools.NewScoreJobTool(geminiClient)
	parseCVTool := tools.NewParseCVTool(geminiClient)

	// Register tools
	registry := tools.NewToolRegistry()
	registry.Register(searchTool)
	registry.Register(fetchTool)
	registry.Register(extractTool)
	registry.Register(scoreTool)
	registry.Register(parseCVTool)

	return &JobAgent{
		cfg:           cfg,
		geminiClient:  geminiClient,
		searchTool:    searchTool,
		fetchTool:     fetchTool,
		extractTool:   extractTool,
		scoreTool:     scoreTool,
		parseCVTool:   parseCVTool,
		toolRegistry:  registry,
		maxConcurrent: 5, // Max concurrent page fetches
	}, nil
}

// Close releases resources
func (a *JobAgent) Close() error {
	return a.geminiClient.Close()
}

// SearchJobsInput represents the input for the job search process
type SearchJobsInput struct {
	CVText     string                 `json:"cv_text,omitempty"`
	CVFileData []byte                 `json:"-"` // PDF/DOC file bytes
	CVFileName string                 `json:"-"` // Original filename
	Query      string                 `json:"query,omitempty"`
	Filters    models.JobSearchFilter `json:"filters,omitempty"`
}

// SearchJobsOutput represents the output of the job search process
type SearchJobsOutput struct {
	Results []models.RankedJob  `json:"results"`
	Profile *models.UserProfile `json:"profile,omitempty"`
	Stats   SearchStats         `json:"stats"`
}

// SearchStats provides statistics about the search
type SearchStats struct {
	URLsFound     int `json:"urls_found"`
	PagesFetched  int `json:"pages_fetched"`
	JobsExtracted int `json:"jobs_extracted"`
	JobsScored    int `json:"jobs_scored"`
	JobsReturned  int `json:"jobs_returned"`
	FetchErrors   int `json:"fetch_errors"`
	ExtractErrors int `json:"extract_errors"`
}

// SearchJobs performs the complete job search flow
func (a *JobAgent) SearchJobs(ctx context.Context, input SearchJobsInput) (*SearchJobsOutput, error) {
	log.Printf("[Agent] Starting job search with query=%q, hasCVText=%v, hasCVFile=%v",
		input.Query, input.CVText != "", len(input.CVFileData) > 0)

	var profile *models.UserProfile
	var err error

	// Step 1: Build user profile based on input mode
	profile, err = a.buildUserProfile(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to build user profile: %w", err)
	}
	log.Printf("[Agent] Built user profile: skills=%v, locations=%v", profile.Skills, profile.PreferredLocations)

	// Determine the effective search query
	effectiveQuery := input.Query
	if effectiveQuery == "" {
		effectiveQuery = profile.GenerateSearchQuery()
	}
	log.Printf("[Agent] Effective search query: %s", effectiveQuery)

	// Step 2: Search for job URLs using PSE
	searchResp, err := a.searchTool.SearchWithProfile(ctx, profile, effectiveQuery, input.Filters)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}
	log.Printf("[Agent] Found %d URLs from web search", len(searchResp.URLs))

	stats := SearchStats{
		URLsFound: len(searchResp.URLs),
	}

	if len(searchResp.URLs) == 0 {
		return &SearchJobsOutput{
			Results: []models.RankedJob{},
			Profile: profile,
			Stats:   stats,
		}, nil
	}

	// Step 3: Fetch pages concurrently
	fetchedPages := a.fetchPagesConcurrently(ctx, searchResp.URLs)
	stats.PagesFetched = len(fetchedPages)
	log.Printf("[Agent] Fetched %d pages", len(fetchedPages))

	// Count fetch errors
	for _, page := range fetchedPages {
		if page.Error != "" {
			stats.FetchErrors++
		}
	}

	// Step 4: Extract jobs from HTML concurrently
	jobs := a.extractJobsConcurrently(ctx, fetchedPages)
	stats.JobsExtracted = len(jobs)
	log.Printf("[Agent] Extracted %d jobs", len(jobs))

	if len(jobs) == 0 {
		return &SearchJobsOutput{
			Results: []models.RankedJob{},
			Profile: profile,
			Stats:   stats,
		}, nil
	}

	maxJobsToScore := 30
	if len(jobs) > maxJobsToScore {
		log.Printf("[Agent] Limiting jobs to score from %d to %d", len(jobs), maxJobsToScore)
		jobs = jobs[:maxJobsToScore]
	}

	// Step 5: Score jobs against profile concurrently
	rankedJobs := a.scoreJobsConcurrently(ctx, profile, jobs)
	stats.JobsScored = len(rankedJobs)
	log.Printf("[Agent] Scored %d jobs", len(rankedJobs))

	// Step 6: Filter jobs with match score >= 50
	filteredJobs := make([]models.RankedJob, 0, len(rankedJobs))
	for _, job := range rankedJobs {
		if job.MatchScore >= 50 {
			filteredJobs = append(filteredJobs, job)
		}
	}
	rankedJobs = filteredJobs

	// Sort by score (highest first)
	sort.Slice(rankedJobs, func(i, j int) bool {
		return rankedJobs[i].MatchScore > rankedJobs[j].MatchScore
	})

	// Limit to max results
	maxResults := a.cfg.MaxJobResults
	if len(rankedJobs) > maxResults {
		rankedJobs = rankedJobs[:maxResults]
	}
	stats.JobsReturned = len(rankedJobs)

	log.Printf("[Agent] Returning %d ranked jobs", len(rankedJobs))

	return &SearchJobsOutput{
		Results: rankedJobs,
		Profile: profile,
		Stats:   stats,
	}, nil
}

// buildUserProfile builds a user profile based on input mode
func (a *JobAgent) buildUserProfile(ctx context.Context, input SearchJobsInput) (*models.UserProfile, error) {
	var profile *models.UserProfile
	var err error

	// Mode 1: PDF file provided - use Gemini multimodal to parse
	if len(input.CVFileData) > 0 && isPDFFile(input.CVFileName) {
		log.Printf("[Agent] Parsing PDF CV using Gemini multimodal: %s", input.CVFileName)
		profile, err = a.geminiClient.ParseCVFromPDF(ctx, input.CVFileData, input.CVFileName)
		if err != nil {
			return nil, fmt.Errorf("CV PDF parsing failed: %w", err)
		}

		// If query is also provided, refine profile with query intent
		if input.Query != "" {
			log.Printf("[Agent] Refining profile with query intent")
			profile, err = a.geminiClient.RefineProfileWithQuery(ctx, profile, input.Query)
			if err != nil {
				log.Printf("[Agent] Warning: failed to refine profile with query: %v", err)
			}
		}
	} else if input.CVText != "" {
		// Mode 2: CV text provided
		log.Printf("[Agent] Parsing CV text to build profile")
		profile, err = a.parseCVTool.ParseCV(ctx, input.CVText)
		if err != nil {
			return nil, fmt.Errorf("CV parsing failed: %w", err)
		}

		// If query is also provided, refine profile with query intent
		if input.Query != "" {
			log.Printf("[Agent] Refining profile with query intent")
			profile, err = a.geminiClient.RefineProfileWithQuery(ctx, profile, input.Query)
			if err != nil {
				log.Printf("[Agent] Warning: failed to refine profile with query: %v", err)
			}
		}
	} else if input.Query != "" {
		// Mode 2: Only query provided
		log.Printf("[Agent] Deriving profile from query")
		profile, err = a.geminiClient.DeriveProfileFromQuery(ctx, input.Query)
		if err != nil {
			// Create minimal profile
			profile = &models.UserProfile{}
		}
	} else {
		// No CV and no query - create empty profile
		profile = &models.UserProfile{}
	}

	// Merge with explicit filters (filters take precedence)
	profile.MergeWithFilters(input.Filters)

	return profile, nil
}

// fetchPagesConcurrently fetches multiple pages in parallel
func (a *JobAgent) fetchPagesConcurrently(ctx context.Context, urls []string) []models.FetchPageResponse {
	results := make([]models.FetchPageResponse, 0, len(urls))
	resultsChan := make(chan models.FetchPageResponse, len(urls))

	// Use semaphore to limit concurrency
	sem := make(chan struct{}, a.maxConcurrent)
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(pageURL string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := a.fetchTool.FetchURL(ctx, pageURL)
			if err != nil {
				resultsChan <- models.FetchPageResponse{URL: pageURL, Error: err.Error()}
				return
			}
			resultsChan <- *resp
		}(url)
	}

	// Wait for all fetches to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for resp := range resultsChan {
		results = append(results, resp)
	}

	return results
}

// extractJobsConcurrently extracts jobs from HTML pages in parallel (max 10 jobs)
func (a *JobAgent) extractJobsConcurrently(ctx context.Context, pages []models.FetchPageResponse) []models.JobPosting {
	const maxJobsToExtract = 10

	jobs := make([]models.JobPosting, 0, maxJobsToExtract)
	jobsChan := make(chan *models.JobPosting, len(pages))

	// Filter valid pages first
	validPages := make([]models.FetchPageResponse, 0)
	for _, page := range pages {
		if page.Error == "" && page.HTML != "" {
			validPages = append(validPages, page)
		}
	}

	// Limit pages to process for performance
	if len(validPages) > maxJobsToExtract {
		log.Printf("[Agent] Limiting pages to extract from %d to %d", len(validPages), maxJobsToExtract)
		validPages = validPages[:maxJobsToExtract]
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, a.maxConcurrent)

	for _, page := range validPages {
		wg.Add(1)
		go func(p models.FetchPageResponse) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			job, err := a.extractTool.ExtractFromHTML(ctx, p.HTML, p.URL)
			if err != nil {
				log.Printf("[Agent] Failed to extract job from %s: %v", p.URL, err)
				return
			}
			if job != nil && job.Title != "" {
				jobsChan <- job
			}
		}(page)
	}

	go func() {
		wg.Wait()
		close(jobsChan)
	}()

	for job := range jobsChan {
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	return jobs
}

// scoreJobsConcurrently scores jobs against profile in parallel
func (a *JobAgent) scoreJobsConcurrently(ctx context.Context, profile *models.UserProfile, jobs []models.JobPosting) []models.RankedJob {
	rankedJobs := make([]models.RankedJob, 0, len(jobs))
	rankedChan := make(chan models.RankedJob, len(jobs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, a.maxConcurrent)

	for _, job := range jobs {
		wg.Add(1)
		go func(j models.JobPosting) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			score, reason, err := a.scoreTool.ScoreJob(ctx, profile, &j)
			if err != nil {
				log.Printf("[Agent] Failed to score job %s: %v", j.Title, err)
				// Default score if scoring fails
				score = 50
				reason = "Unable to calculate match score"
			}

			rankedChan <- models.RankedJob{
				JobPosting:  j,
				MatchScore:  score,
				MatchReason: reason,
			}
		}(job)
	}

	go func() {
		wg.Wait()
		close(rankedChan)
	}()

	for ranked := range rankedChan {
		rankedJobs = append(rankedJobs, ranked)
	}

	return rankedJobs
}

// GetToolDefinitions returns the tool definitions for external use
func (a *JobAgent) GetToolDefinitions() []map[string]interface{} {
	return a.toolRegistry.GetToolDefinitions()
}

// isPDFFile checks if the filename indicates a PDF file
func isPDFFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".pdf")
}
