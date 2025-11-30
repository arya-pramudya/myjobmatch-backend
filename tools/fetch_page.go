package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/myjobmatch/backend/config"
	"github.com/myjobmatch/backend/models"
)

// FetchPageTool fetches HTML content from a URL
type FetchPageTool struct {
	client *http.Client
}

// NewFetchPageTool creates a new page fetcher tool
func NewFetchPageTool(cfg *config.Config) *FetchPageTool {
	return &FetchPageTool{
		client: &http.Client{
			Timeout: time.Duration(cfg.HTTPTimeoutSeconds) * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (t *FetchPageTool) Name() string {
	return "fetch_page_html"
}

func (t *FetchPageTool) Description() string {
	return `Fetch HTML content from a job posting URL.
Input should be a URL string.
Returns the HTML content of the page.`
}

func (t *FetchPageTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "The URL to fetch HTML content from",
			},
		},
		"required": []string{"url"},
	}
}

// FetchInput represents the input for the fetch tool
type FetchInput struct {
	URL string `json:"url"`
}

func (t *FetchPageTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var fetchInput FetchInput
	if err := json.Unmarshal(input, &fetchInput); err != nil {
		return NewErrorResult(fmt.Sprintf("invalid input: %v", err))
	}

	html, err := t.fetchPage(ctx, fetchInput.URL)
	if err != nil {
		return NewErrorResult(fmt.Sprintf("fetch failed: %v", err))
	}

	response := models.FetchPageResponse{
		HTML: html,
		URL:  fetchInput.URL,
	}

	return NewSuccessResult(response)
}

func (t *FetchPageTool) fetchPage(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page returned status %d", resp.StatusCode)
	}

	// Read body with limit
	maxBytes := int64(5 * 1024 * 1024) // 5MB limit
	limitedReader := io.LimitReader(resp.Body, maxBytes)

	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	html := string(body)

	// Basic HTML cleaning - remove scripts and styles for smaller payload
	html = t.cleanHTML(html)

	return html, nil
}

func (t *FetchPageTool) cleanHTML(html string) string {
	// Remove script tags and their content
	for {
		start := strings.Index(strings.ToLower(html), "<script")
		if start == -1 {
			break
		}
		end := strings.Index(strings.ToLower(html[start:]), "</script>")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+9:]
	}

	// Remove style tags and their content
	for {
		start := strings.Index(strings.ToLower(html), "<style")
		if start == -1 {
			break
		}
		end := strings.Index(strings.ToLower(html[start:]), "</style>")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+8:]
	}

	// Remove excessive whitespace
	for strings.Contains(html, "  ") {
		html = strings.ReplaceAll(html, "  ", " ")
	}
	for strings.Contains(html, "\n\n\n") {
		html = strings.ReplaceAll(html, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(html)
}

// FetchURL is a direct method to fetch a URL
func (t *FetchPageTool) FetchURL(ctx context.Context, url string) (*models.FetchPageResponse, error) {
	inputJSON, err := json.Marshal(FetchInput{URL: url})
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
		return &models.FetchPageResponse{URL: url, Error: result.Error}, nil
	}

	var response models.FetchPageResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
