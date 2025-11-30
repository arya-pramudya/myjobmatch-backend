package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"cloud.google.com/go/vertexai/genai"

	"github.com/myjobmatch/backend/config"
	"github.com/myjobmatch/backend/models"
)

// Client wraps the Vertex AI Gemini client
type Client struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	projectID string
	location  string
	modelName string
}

// NewClient creates a new Gemini client
func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	client, err := genai.NewClient(ctx, cfg.ProjectID, cfg.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	model := client.GenerativeModel(cfg.GeminiModel)

	// Configure model parameters
	model.SetTemperature(0.2) // Lower temperature for more consistent outputs
	model.SetTopP(0.8)
	model.SetMaxOutputTokens(8192)

	return &Client{
		client:    client,
		model:     model,
		projectID: cfg.ProjectID,
		location:  cfg.Location,
		modelName: cfg.GeminiModel,
	}, nil
}

// Close closes the Gemini client
func (c *Client) Close() error {
	return c.client.Close()
}

// ParseCVFromPDF extracts user profile from PDF bytes using Gemini's multimodal capability
func (c *Client) ParseCVFromPDF(ctx context.Context, pdfData []byte, filename string) (*models.UserProfile, error) {
	prompt := `Analyze this CV/resume document and extract structured information.
Return a JSON object with the following fields (use null for missing data):

{
  "name": "Full name",
  "email": "Email address",
  "phone": "Phone number",
  "summary": "Professional summary or objective",
  "title": "Current or desired job title",
  "experience_years": 0,
  "skills": ["skill1", "skill2"],
  "technical_stack": ["technology1", "technology2"],
  "languages": ["English", "Indonesian"],
  "preferred_roles": ["Backend Developer", "Software Engineer"],
  "preferred_locations": ["Jakarta", "Remote"],
  "preferred_remote_modes": ["WFH", "Hybrid"],
  "preferred_job_types": ["full_time"],
  "education": [
    {
      "degree": "Bachelor",
      "field": "Computer Science",
      "institution": "University Name",
      "year": 2020
    }
  ],
  "work_history": [
    {
      "title": "Software Engineer",
      "company": "Company Name",
      "location": "Jakarta",
      "start_date": "2020-01",
      "end_date": "2023-12",
      "description": "Brief description",
      "skills": ["Go", "Python"]
    }
  ],
  "certifications": ["AWS Certified", "GCP Professional"],
  "achievements": ["Led team of 5", "Increased performance by 50%"]
}

IMPORTANT for experience_years:
- Calculate TOTAL years of professional experience by looking at ALL work history entries
- Sum up all periods from earliest start date to latest end date (or current date if "Present")
- For example: if work history shows 2022-2025, that's approximately 3 years of experience
- Do NOT just count individual job durations, consider the overall career span

Infer preferred_roles based on experience and skills.
Infer preferred_remote_modes and preferred_locations from any mentioned preferences or recent work.

Return ONLY the JSON object, no markdown formatting, no explanation.`

	// Create PDF blob for Gemini multimodal
	pdfBlob := genai.Blob{
		MIMEType: "application/pdf",
		Data:     pdfData,
	}

	resp, err := c.model.GenerateContent(ctx, pdfBlob, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	text := extractText(resp)
	text = cleanJSON(text)

	var profile models.UserProfile
	if err := json.Unmarshal([]byte(text), &profile); err != nil {
		log.Printf("Failed to parse CV PDF response: %s", text)
		return nil, fmt.Errorf("failed to parse profile JSON: %w", err)
	}

	log.Printf("[Gemini] Parsed CV PDF '%s': name=%s, skills=%d, experience=%d years",
		filename, profile.Name, len(profile.Skills), profile.Experience)

	return &profile, nil
}

// ParseCV extracts user profile from CV text
func (c *Client) ParseCV(ctx context.Context, cvText string) (*models.UserProfile, error) {
	prompt := fmt.Sprintf(`Analyze the following CV/resume and extract structured information. 
Return a JSON object with the following fields (use null for missing data):

{
  "name": "Full name",
  "email": "Email address",
  "phone": "Phone number",
  "summary": "Professional summary or objective",
  "title": "Current or desired job title",
  "experience_years": 0,
  "skills": ["skill1", "skill2"],
  "technical_stack": ["technology1", "technology2"],
  "languages": ["English", "Indonesian"],
  "preferred_roles": ["Backend Developer", "Software Engineer"],
  "preferred_locations": ["Jakarta", "Remote"],
  "preferred_remote_modes": ["WFH", "Hybrid"],
  "preferred_job_types": ["full_time"],
  "education": [
    {
      "degree": "Bachelor",
      "field": "Computer Science",
      "institution": "University Name",
      "year": 2020
    }
  ],
  "work_history": [
    {
      "title": "Software Engineer",
      "company": "Company Name",
      "location": "Jakarta",
      "start_date": "2020-01",
      "end_date": "2023-12",
      "description": "Brief description",
      "skills": ["Go", "Python"]
    }
  ],
  "certifications": ["AWS Certified", "GCP Professional"],
  "achievements": ["Led team of 5", "Increased performance by 50%%"]
}

IMPORTANT for experience_years:
- Calculate TOTAL years of professional experience by looking at ALL work history entries
- Sum up all periods from earliest start date to latest end date (or current date if "Present")
- For example: if work history shows 2022-2025, that's approximately 3 years of experience
- Do NOT just count individual job durations, consider the overall career span

Infer preferred_roles based on experience and skills.
Infer preferred_remote_modes and preferred_locations from any mentioned preferences or recent work.

CV TEXT:
%s

Return ONLY the JSON object, no markdown formatting, no explanation.`, cvText)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	text := extractText(resp)
	text = cleanJSON(text)

	var profile models.UserProfile
	if err := json.Unmarshal([]byte(text), &profile); err != nil {
		log.Printf("Failed to parse CV response: %s", text)
		return nil, fmt.Errorf("failed to parse profile JSON: %w", err)
	}

	return &profile, nil
}

// ExtractJobFromHTML extracts job posting from HTML content
func (c *Client) ExtractJobFromHTML(ctx context.Context, html, url string) (*models.JobPosting, error) {
	// Truncate HTML if too long
	maxLen := 50000
	if len(html) > maxLen {
		html = html[:maxLen]
	}

	prompt := fmt.Sprintf(`Extract job posting information from this HTML content. 
Return a JSON object with the following fields:

{
  "title": "Job title",
  "company": "Company name",
  "description": "Job description (summarize if very long, max 500 chars)",
  "location": "Job location",
  "work_type": "full_time|part_time|contract|internship|freelance",
  "site_setting": "WFH|WFO|Hybrid|Unknown",
  "salary": "Salary range if mentioned",
  "date_posted": "Date posted if available",
  "requirements": "Key requirements (summarize, max 300 chars)",
  "benefits": "Benefits if mentioned",
  "experience_level": "entry|mid|senior|lead",
  "tags": ["relevant", "keywords", "technologies"]
}

URL: %s

HTML CONTENT:
%s

Return ONLY the JSON object. If this is not a job posting page, return {"error": "not_a_job_posting"}.`, url, html)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	text := extractText(resp)
	text = cleanJSON(text)

	// Check for error response
	var errResp map[string]string
	if err := json.Unmarshal([]byte(text), &errResp); err == nil {
		if errResp["error"] == "not_a_job_posting" {
			return nil, fmt.Errorf("not a job posting page")
		}
	}

	var job models.JobPosting
	if err := json.Unmarshal([]byte(text), &job); err != nil {
		log.Printf("Failed to parse job response: %s", text)
		return nil, fmt.Errorf("failed to parse job JSON: %w", err)
	}

	// Normalize fields
	job.URL = url
	job.Source = "web"
	job.WorkType = models.NormalizeWorkType(job.WorkType)
	job.SiteSetting = models.NormalizeSiteSetting(job.SiteSetting)

	return &job, nil
}

// ScoreJobMatch scores how well a job matches a user profile
func (c *Client) ScoreJobMatch(ctx context.Context, profile *models.UserProfile, job *models.JobPosting) (int, string, error) {
	profileJSON, _ := json.Marshal(profile)
	jobJSON, _ := json.Marshal(job)

	prompt := fmt.Sprintf(`Analyze how well this job matches the candidate's profile and return a match score.

CANDIDATE PROFILE:
%s

JOB POSTING:
%s

Return a JSON object with:
{
  "match_score": 0-100,
  "match_reason": "1-2 sentences explaining the match or mismatch"
}

Consider:
- Skills alignment (most important)
- Experience level match
- Location and remote preferences
- Job type preferences
- Industry/domain relevance

Return ONLY the JSON object.`, profileJSON, jobJSON)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate content: %w", err)
	}

	text := extractText(resp)
	text = cleanJSON(text)

	var result models.ScoreJobResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		log.Printf("Failed to parse score response: %s", text)
		return 0, "", fmt.Errorf("failed to parse score JSON: %w", err)
	}

	return result.MatchScore, result.MatchReason, nil
}

// RefineProfileWithQuery uses query to refine/supplement profile
func (c *Client) RefineProfileWithQuery(ctx context.Context, profile *models.UserProfile, query string) (*models.UserProfile, error) {
	profileJSON, _ := json.Marshal(profile)

	prompt := fmt.Sprintf(`Given this user profile and their search query, update the profile to reflect their current job search intent.

EXISTING PROFILE:
%s

SEARCH QUERY: %s

Update the profile JSON with any new information from the query:
- Add any skills/technologies mentioned in query
- Update preferred_roles if query indicates specific roles
- Update preferred_locations if query mentions locations
- Update preferred_remote_modes if query mentions remote/WFH/hybrid
- Keep existing profile data that isn't contradicted by query

Return the UPDATED profile as a JSON object (same structure as input).
Return ONLY the JSON object.`, profileJSON, query)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	text := extractText(resp)
	text = cleanJSON(text)

	var updatedProfile models.UserProfile
	if err := json.Unmarshal([]byte(text), &updatedProfile); err != nil {
		log.Printf("Failed to parse refined profile: %s", text)
		return profile, nil // Return original on error
	}

	return &updatedProfile, nil
}

// DeriveProfileFromQuery creates a basic profile from just a search query
func (c *Client) DeriveProfileFromQuery(ctx context.Context, query string) (*models.UserProfile, error) {
	prompt := fmt.Sprintf(`Extract job search preferences from this search query and create a candidate profile.

SEARCH QUERY: %s

Return a JSON object with relevant fields:
{
  "title": "Inferred desired job title",
  "skills": ["extracted", "skills", "technologies"],
  "preferred_roles": ["inferred", "roles"],
  "preferred_locations": ["mentioned", "locations"],
  "preferred_remote_modes": ["WFH/WFO/Hybrid if mentioned"],
  "preferred_job_types": ["full_time/contract/etc if mentioned"],
  "experience_level": "entry/mid/senior if inferable"
}

Only include fields that can be reasonably inferred from the query.
Return ONLY the JSON object.`, query)

	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	text := extractText(resp)
	text = cleanJSON(text)

	var profile models.UserProfile
	if err := json.Unmarshal([]byte(text), &profile); err != nil {
		log.Printf("Failed to parse derived profile: %s", text)
		return &models.UserProfile{}, nil
	}

	return &profile, nil
}

// Helper functions

func extractText(resp *genai.GenerateContentResponse) string {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			sb.WriteString(string(textPart))
		}
	}
	return sb.String()
}

func cleanJSON(text string) string {
	// Remove markdown code blocks if present
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	return text
}
