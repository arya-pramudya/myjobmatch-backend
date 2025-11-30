package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myjobmatch/backend/gemini"
	"github.com/myjobmatch/backend/models"
)

// ExtractJobTool extracts job posting information from HTML using Gemini
type ExtractJobTool struct {
	geminiClient *gemini.Client
}

// NewExtractJobTool creates a new job extraction tool
func NewExtractJobTool(geminiClient *gemini.Client) *ExtractJobTool {
	return &ExtractJobTool{
		geminiClient: geminiClient,
	}
}

func (t *ExtractJobTool) Name() string {
	return "extract_job_from_html"
}

func (t *ExtractJobTool) Description() string {
	return `Extract structured job posting information from HTML content using AI.
Input should include HTML content and the source URL.
Returns a structured JobPosting object with title, company, description, location, etc.`
}

func (t *ExtractJobTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"html": map[string]interface{}{
				"type":        "string",
				"description": "HTML content of the job posting page",
			},
			"url": map[string]interface{}{
				"type":        "string",
				"description": "Source URL of the job posting",
			},
		},
		"required": []string{"html", "url"},
	}
}

// ExtractJobInput represents the input for job extraction
type ExtractJobInput struct {
	HTML string `json:"html"`
	URL  string `json:"url"`
}

func (t *ExtractJobTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var extractInput ExtractJobInput
	if err := json.Unmarshal(input, &extractInput); err != nil {
		return NewErrorResult(fmt.Sprintf("invalid input: %v", err))
	}

	job, err := t.geminiClient.ExtractJobFromHTML(ctx, extractInput.HTML, extractInput.URL)
	if err != nil {
		return NewErrorResult(fmt.Sprintf("extraction failed: %v", err))
	}

	response := models.ExtractJobResponse{
		Job: job,
	}

	return NewSuccessResult(response)
}

// ExtractFromHTML is a direct method to extract job from HTML
func (t *ExtractJobTool) ExtractFromHTML(ctx context.Context, html, url string) (*models.JobPosting, error) {
	inputJSON, err := json.Marshal(ExtractJobInput{HTML: html, URL: url})
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

	var response models.ExtractJobResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		return nil, err
	}

	return response.Job, nil
}
