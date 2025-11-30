package models

import "encoding/json"

// FlexibleStringSlice can unmarshal from either a string or []string
type FlexibleStringSlice []string

func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as []string first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = arr
		return nil
	}

	// Try to unmarshal as string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		if str != "" {
			*f = []string{str}
		} else {
			*f = []string{}
		}
		return nil
	}

	// If both fail, return empty slice
	*f = []string{}
	return nil
}

// JobPosting represents a job posting extracted from a webpage
type JobPosting struct {
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	WorkType    string   `json:"work_type"`    // full_time, part_time, contract, internship
	SiteSetting string   `json:"site_setting"` // WFH, WFO, Hybrid, Unknown
	URL         string   `json:"url"`
	Source      string   `json:"source"` // web, linkedin, etc.
	Tags        []string `json:"tags,omitempty"`

	// Optional fields
	Salary          string              `json:"salary,omitempty"`
	DatePosted      string              `json:"date_posted,omitempty"`
	ApplicationURL  string              `json:"application_url,omitempty"`
	Requirements    string              `json:"requirements,omitempty"`
	Benefits        FlexibleStringSlice `json:"benefits,omitempty"`
	ExperienceLevel string              `json:"experience_level,omitempty"` // entry, mid, senior, lead
}

// RankedJob is a JobPosting with match scoring
type RankedJob struct {
	JobPosting
	MatchScore  int    `json:"match_score"`  // 0-100
	MatchReason string `json:"match_reason"` // 1-2 sentence explanation
}

// JobSearchResult represents a single search result from PSE
type JobSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WorkType constants
const (
	WorkTypeFullTime   = "full_time"
	WorkTypePartTime   = "part_time"
	WorkTypeContract   = "contract"
	WorkTypeInternship = "internship"
	WorkTypeFreelance  = "freelance"
)

// SiteSetting constants
const (
	SiteSettingWFH     = "WFH"
	SiteSettingWFO     = "WFO"
	SiteSettingHybrid  = "Hybrid"
	SiteSettingUnknown = "Unknown"
)

// ExperienceLevel constants
const (
	ExperienceLevelEntry  = "entry"
	ExperienceLevelMid    = "mid"
	ExperienceLevelSenior = "senior"
	ExperienceLevelLead   = "lead"
)

// NormalizeWorkType normalizes various work type strings to standard values
func NormalizeWorkType(raw string) string {
	switch raw {
	case "full-time", "Full-Time", "Full Time", "fulltime", "FULL_TIME":
		return WorkTypeFullTime
	case "part-time", "Part-Time", "Part Time", "parttime", "PART_TIME":
		return WorkTypePartTime
	case "contract", "Contract", "CONTRACT", "contractor":
		return WorkTypeContract
	case "internship", "Internship", "INTERNSHIP", "intern":
		return WorkTypeInternship
	case "freelance", "Freelance", "FREELANCE":
		return WorkTypeFreelance
	default:
		return raw
	}
}

// NormalizeSiteSetting normalizes various site setting strings to standard values
func NormalizeSiteSetting(raw string) string {
	switch raw {
	case "remote", "Remote", "REMOTE", "work from home", "Work From Home", "wfh":
		return SiteSettingWFH
	case "onsite", "Onsite", "ONSITE", "on-site", "On-Site", "office", "Office", "wfo", "work from office":
		return SiteSettingWFO
	case "hybrid", "Hybrid", "HYBRID", "flexible":
		return SiteSettingHybrid
	default:
		return SiteSettingUnknown
	}
}
