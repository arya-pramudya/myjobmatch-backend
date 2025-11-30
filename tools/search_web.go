package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/myjobmatch/backend/config"
	"github.com/myjobmatch/backend/models"
)

// SearchWebTool searches for job postings using Google Programmable Search Engine
type SearchWebTool struct {
	apiKey   string
	engineID string
	client   *http.Client
}

// NewSearchWebTool creates a new web search tool
func NewSearchWebTool(cfg *config.Config) *SearchWebTool {
	return &SearchWebTool{
		apiKey:   cfg.PSEAPIKey,
		engineID: cfg.PSEEngineID,
		client: &http.Client{
			Timeout: time.Duration(cfg.HTTPTimeoutSeconds) * time.Second,
		},
	}
}

func (t *SearchWebTool) Name() string {
	return "search_web_for_jobs"
}

func (t *SearchWebTool) Description() string {
	return `Search the web for job postings using Google Programmable Search Engine.
Input should include a query string and optional filters.
Returns a list of URLs to job postings.`
}

func (t *SearchWebTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query for finding jobs (e.g., 'Golang developer Jakarta remote')",
			},
			"locations": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "List of preferred locations",
			},
			"remote_modes": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Remote modes: WFH, WFO, Hybrid",
			},
		},
		"required": []string{"query"},
	}
}

// SearchInput represents the input for the search tool
type SearchInput struct {
	Query       string   `json:"query"`
	Locations   []string `json:"locations,omitempty"`
	RemoteModes []string `json:"remote_modes,omitempty"`
}

// PSEResponse represents the Google PSE API response
type PSEResponse struct {
	Items []PSEItem `json:"items"`
}

// PSEItem represents a single search result
type PSEItem struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

func (t *SearchWebTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var searchInput SearchInput
	if err := json.Unmarshal(input, &searchInput); err != nil {
		return NewErrorResult(fmt.Sprintf("invalid input: %v", err))
	}

	// Build the search query
	query := t.buildQuery(searchInput)

	// Call PSE API
	results, err := t.search(ctx, query)
	if err != nil {
		return NewErrorResult(fmt.Sprintf("search failed: %v", err))
	}

	// Extract URLs and convert to response
	urls := make([]string, 0, len(results))
	searchResults := make([]models.JobSearchResult, 0, len(results))

	for _, item := range results {
		urls = append(urls, item.Link)
		searchResults = append(searchResults, models.JobSearchResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: item.Snippet,
		})
	}

	response := models.WebSearchResponse{
		URLs:    urls,
		Results: searchResults,
	}

	return NewSuccessResult(response)
}

// Job portal site filters for Google PSE
var jobPortalSites = []string{
	"site:linkedin.com/jobs/view",
	"site:jobstreet.com",
	"site:jobstreet.co.id",
	"site:dealls.com/loker",
	"site:glints.com/opportunities",
	"site:kalibrr.com/c",
	"site:id.indeed.com",
}

func (t *SearchWebTool) buildQuery(input SearchInput) string {
	var parts []string

	// Add main query
	parts = append(parts, input.Query)

	// Add "job" keyword if not present
	if !strings.Contains(strings.ToLower(input.Query), "job") {
		parts = append(parts, "job")
	}

	// Add locations
	if len(input.Locations) > 0 {
		parts = append(parts, input.Locations[0])
	}

	// Add remote mode hints
	if len(input.RemoteModes) > 0 {
		mode := input.RemoteModes[0]
		switch mode {
		case "WFH":
			parts = append(parts, "remote")
		case "Hybrid":
			parts = append(parts, "hybrid")
		}
	}

	return strings.Join(parts, " ")
}

func (t *SearchWebTool) search(ctx context.Context, query string) ([]PSEItem, error) {
	var allItems []PSEItem
	seen := make(map[string]bool) // Deduplicate URLs

	log.Printf("[Search] Starting search with base query: %s", query)

	// Search each job portal separately for better results
	for _, siteFilter := range jobPortalSites {
		siteQuery := query + " " + siteFilter
		log.Printf("[Search] Searching: %s", siteQuery)

		// Get up to 50 results per site (multiple pages)
		for start := 1; start <= 50; start += 10 {
			items, err := t.searchPage(ctx, siteQuery, start, 10)
			if err != nil {
				log.Printf("[Search] Error for %s: %v", siteFilter, err)
				break
			}

			log.Printf("[Search] Got %d results from %s (page %d)", len(items), siteFilter, (start/10)+1)

			for _, item := range items {
				if !seen[item.Link] && isPreferredDetailURL(item.Link) {
					seen[item.Link] = true
					allItems = append(allItems, item)
				}
			}

			if len(items) < 10 {
				break
			}
		}
	}

	log.Printf("[Search] Total unique URLs found: %d", len(allItems))
	return allItems, nil
}

// isPreferredDetailURL filters URLs so that for certain sites we only keep detailed job pages
func isPreferredDetailURL(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		// If we can't parse the URL, keep it rather than risk dropping valid jobs
		return true
	}

	host := strings.ToLower(u.Host)
	q := u.Query()

	// For JobStreet, only keep URLs that have a concrete jobId parameter
	if strings.Contains(host, "jobstreet") {
		if q.Get("jobId") == "" {
			return false
		}
	}

	// For Indeed, only keep URLs that reference a specific job via vjk parameter
	if strings.Contains(host, "indeed") {
		if q.Get("vjk") == "" {
			return false
		}
	}

	return true
}

// searchPage fetches a single page of results
func (t *SearchWebTool) searchPage(ctx context.Context, query string, start, num int) ([]PSEItem, error) {
	baseURL := "https://www.googleapis.com/customsearch/v1"
	params := url.Values{}
	params.Set("key", t.apiKey)
	params.Set("cx", t.engineID)
	params.Set("q", query)
	params.Set("num", fmt.Sprintf("%d", num))
	params.Set("start", fmt.Sprintf("%d", start))

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PSE API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var pseResp PSEResponse
	if err := json.Unmarshal(body, &pseResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return pseResp.Items, nil
}

// SearchWithProfile performs a search using a user profile
func (t *SearchWebTool) SearchWithProfile(ctx context.Context, profile *models.UserProfile, query string, filters models.JobSearchFilter) (*models.WebSearchResponse, error) {
	// Build search input from profile and filters
	searchInput := SearchInput{
		Query:       query,
		Locations:   filters.Locations,
		RemoteModes: filters.RemoteModes,
	}

	// If query is empty, generate from profile
	if query == "" && profile != nil {
		searchInput.Query = profile.GenerateSearchQuery()
	}

	// If locations not in filters, use from profile
	if len(searchInput.Locations) == 0 && profile != nil {
		searchInput.Locations = profile.PreferredLocations
	}

	// If remote modes not in filters, use from profile
	if len(searchInput.RemoteModes) == 0 && profile != nil {
		searchInput.RemoteModes = profile.PreferredRemoteModes
	}

	inputJSON, err := json.Marshal(searchInput)
	if err != nil {
		return nil, err
	}

	resultJSON, err := t.Execute(ctx, inputJSON)
	if err != nil {
		return nil, err
	}

	var result ToolResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf(result.Error)
	}

	var response models.WebSearchResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
