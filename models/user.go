package models

// UserProfile represents the extracted profile from CV or query
type UserProfile struct {
	// Personal Information
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`

	// Professional Summary
	Summary    string  `json:"summary,omitempty"`
	Title      string  `json:"title,omitempty"`
	Experience float64 `json:"experience_years,omitempty"`

	// Skills and Technologies
	Skills         []string `json:"skills,omitempty"`
	TechnicalStack []string `json:"technical_stack,omitempty"`
	Languages      []string `json:"languages,omitempty"`

	// Preferences
	PreferredRoles       []string `json:"preferred_roles,omitempty"`
	PreferredLocations   []string `json:"preferred_locations,omitempty"`
	PreferredRemoteModes []string `json:"preferred_remote_modes,omitempty"` // WFH, WFO, Hybrid
	PreferredJobTypes    []string `json:"preferred_job_types,omitempty"`    // full_time, contract, etc.
	MinSalary            int      `json:"min_salary,omitempty"`
	MaxSalary            int      `json:"max_salary,omitempty"`
	Currency             string   `json:"currency,omitempty"`

	// Education
	Education []Education `json:"education,omitempty"`

	// Work History
	WorkHistory []WorkExperience `json:"work_history,omitempty"`

	// Additional
	Certifications []string `json:"certifications,omitempty"`
	Achievements   []string `json:"achievements,omitempty"`
}

// Education represents educational background
type Education struct {
	Degree      string `json:"degree,omitempty"`
	Field       string `json:"field,omitempty"`
	Institution string `json:"institution,omitempty"`
	Year        int    `json:"year,omitempty"`
}

// WorkExperience represents past work experience
type WorkExperience struct {
	Title       string   `json:"title,omitempty"`
	Company     string   `json:"company,omitempty"`
	Location    string   `json:"location,omitempty"`
	StartDate   string   `json:"start_date,omitempty"`
	EndDate     string   `json:"end_date,omitempty"`
	Description string   `json:"description,omitempty"`
	Skills      []string `json:"skills,omitempty"`
}

// JobSearchFilter represents user-specified search filters
type JobSearchFilter struct {
	Locations   []string `json:"locations,omitempty"`
	RemoteModes []string `json:"remote_modes,omitempty"` // WFH, WFO, Hybrid
	JobTypes    []string `json:"job_types,omitempty"`    // full_time, part_time, contract, internship
	MinSalary   int      `json:"min_salary,omitempty"`
	MaxSalary   int      `json:"max_salary,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	DatePosted  string   `json:"date_posted,omitempty"` // last_24h, last_week, last_month
}

// SearchJobsInput is the unified input for the job search agent
type SearchJobsInput struct {
	Profile    UserProfile     `json:"profile"`
	Query      string          `json:"query,omitempty"`
	Filters    JobSearchFilter `json:"filters,omitempty"`
	UserIntent string          `json:"user_intent,omitempty"` // Additional intent from query when CV is also provided
}

// NewUserProfileFromQuery creates a lightweight profile from search query and filters
func NewUserProfileFromQuery(query string, filters JobSearchFilter) UserProfile {
	profile := UserProfile{
		PreferredLocations:   filters.Locations,
		PreferredRemoteModes: filters.RemoteModes,
		PreferredJobTypes:    filters.JobTypes,
		MinSalary:            filters.MinSalary,
		MaxSalary:            filters.MaxSalary,
		Currency:             filters.Currency,
	}

	// The query itself will be used to derive skills/roles via Gemini if needed
	return profile
}

// MergeWithFilters merges profile with explicit filters (filters take precedence)
func (p *UserProfile) MergeWithFilters(filters JobSearchFilter) {
	if len(filters.Locations) > 0 {
		p.PreferredLocations = filters.Locations
	}
	if len(filters.RemoteModes) > 0 {
		p.PreferredRemoteModes = filters.RemoteModes
	}
	if len(filters.JobTypes) > 0 {
		p.PreferredJobTypes = filters.JobTypes
	}
	if filters.MinSalary > 0 {
		p.MinSalary = filters.MinSalary
	}
	if filters.MaxSalary > 0 {
		p.MaxSalary = filters.MaxSalary
	}
	if filters.Currency != "" {
		p.Currency = filters.Currency
	}
}

// GenerateSearchQuery generates a search query string from the profile
func (p *UserProfile) GenerateSearchQuery() string {
	query := ""

	// Add title/role
	if p.Title != "" {
		query += p.Title + " "
	} else if len(p.PreferredRoles) > 0 {
		query += p.PreferredRoles[0] + " "
	}

	// Add top skills
	if len(p.Skills) > 0 {
		for i, skill := range p.Skills {
			if i >= 3 {
				break
			}
			query += skill + " "
		}
	}

	// Add location
	if len(p.PreferredLocations) > 0 {
		query += p.PreferredLocations[0] + " "
	}

	// Add remote mode
	if len(p.PreferredRemoteModes) > 0 {
		query += p.PreferredRemoteModes[0] + " "
	}

	query += "job"

	return query
}
