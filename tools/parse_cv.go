package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myjobmatch/backend/gemini"
	"github.com/myjobmatch/backend/models"
)

// ParseCVTool parses CV text to extract user profile using Gemini
type ParseCVTool struct {
	geminiClient *gemini.Client
}

// NewParseCVTool creates a new CV parsing tool
func NewParseCVTool(geminiClient *gemini.Client) *ParseCVTool {
	return &ParseCVTool{
		geminiClient: geminiClient,
	}
}

func (t *ParseCVTool) Name() string {
	return "parse_cv"
}

func (t *ParseCVTool) Description() string {
	return `Parse CV/resume text to extract structured user profile using AI.
Input should be the CV text content.
Returns a structured UserProfile with skills, experience, preferences, etc.`
}

func (t *ParseCVTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"cv_text": map[string]interface{}{
				"type":        "string",
				"description": "The CV/resume text content to parse",
			},
		},
		"required": []string{"cv_text"},
	}
}

// ParseCVInput represents the input for CV parsing
type ParseCVInput struct {
	CVText string `json:"cv_text"`
}

func (t *ParseCVTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var parseInput ParseCVInput
	if err := json.Unmarshal(input, &parseInput); err != nil {
		return NewErrorResult(fmt.Sprintf("invalid input: %v", err))
	}

	profile, err := t.geminiClient.ParseCV(ctx, parseInput.CVText)
	if err != nil {
		return NewErrorResult(fmt.Sprintf("CV parsing failed: %v", err))
	}

	response := models.CVParseResponse{
		Profile: *profile,
	}

	return NewSuccessResult(response)
}

// ParseCV is a direct method to parse CV text
func (t *ParseCVTool) ParseCV(ctx context.Context, cvText string) (*models.UserProfile, error) {
	inputJSON, err := json.Marshal(ParseCVInput{CVText: cvText})
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

	var response models.CVParseResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		return nil, err
	}

	return &response.Profile, nil
}
