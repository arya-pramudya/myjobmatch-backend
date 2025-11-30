package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/myjobmatch/backend/gemini"
	"github.com/myjobmatch/backend/models"
)

// ScoreJobTool scores job-profile match using Gemini
type ScoreJobTool struct {
	geminiClient *gemini.Client
}

// NewScoreJobTool creates a new job scoring tool
func NewScoreJobTool(geminiClient *gemini.Client) *ScoreJobTool {
	return &ScoreJobTool{
		geminiClient: geminiClient,
	}
}

func (t *ScoreJobTool) Name() string {
	return "score_job_match"
}

func (t *ScoreJobTool) Description() string {
	return `Score how well a job posting matches a user's profile using AI.
Input should include the user profile and job posting.
Returns a match score (0-100) and a reason explaining the match.`
}

func (t *ScoreJobTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"profile": map[string]interface{}{
				"type":        "object",
				"description": "User profile with skills, preferences, experience",
			},
			"job": map[string]interface{}{
				"type":        "object",
				"description": "Job posting to score against the profile",
			},
		},
		"required": []string{"profile", "job"},
	}
}

// ScoreJobInput represents the input for job scoring
type ScoreJobInput struct {
	Profile models.UserProfile `json:"profile"`
	Job     models.JobPosting  `json:"job"`
}

func (t *ScoreJobTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var scoreInput ScoreJobInput
	if err := json.Unmarshal(input, &scoreInput); err != nil {
		return NewErrorResult(fmt.Sprintf("invalid input: %v", err))
	}

	score, reason, err := t.geminiClient.ScoreJobMatch(ctx, &scoreInput.Profile, &scoreInput.Job)
	if err != nil {
		return NewErrorResult(fmt.Sprintf("scoring failed: %v", err))
	}

	response := models.ScoreJobResponse{
		MatchScore:  score,
		MatchReason: reason,
	}

	return NewSuccessResult(response)
}

// ScoreJob is a direct method to score a job
func (t *ScoreJobTool) ScoreJob(ctx context.Context, profile *models.UserProfile, job *models.JobPosting) (int, string, error) {
	inputJSON, err := json.Marshal(ScoreJobInput{Profile: *profile, Job: *job})
	if err != nil {
		return 0, "", err
	}

	resultJSON, err := t.Execute(ctx, inputJSON)
	if err != nil {
		return 0, "", err
	}

	var result ToolResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return 0, "", err
	}

	if !result.Success {
		return 0, "", fmt.Errorf(result.Error)
	}

	var response models.ScoreJobResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		return 0, "", err
	}

	return response.MatchScore, response.MatchReason, nil
}
