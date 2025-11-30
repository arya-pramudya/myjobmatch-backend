package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	// Google Cloud
	ProjectID string
	Location  string

	// Programmable Search Engine
	PSEAPIKey   string
	PSEEngineID string

	// Server
	Port  string
	Debug bool

	// Gemini Model
	GeminiModel string

	// Timeouts
	HTTPTimeoutSeconds int
	MaxJobResults      int

	// Authentication
	JWTSecret      string
	JWTExpiryHours int
	GoogleClientID string

	// Cloud Storage
	CVBucketName string
}

// Load loads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		// Google Cloud
		ProjectID: getEnv("PROJECT_ID", ""),
		Location:  getEnv("LOCATION", ""),

		// Programmable Search Engine
		PSEAPIKey:   getEnv("PSE_API_KEY", ""),
		PSEEngineID: getEnv("PSE_ENGINE_ID", ""), // Get from https://programmablesearchengine.google.com/

		// Server
		Port:  getEnv("PORT", "8080"),
		Debug: getEnvBool("DEBUG", false),

		// Gemini Model
		GeminiModel: getEnv("GEMINI_MODEL", "gemini-2.5-flash"),

		// Timeouts and limits
		HTTPTimeoutSeconds: getEnvInt("HTTP_TIMEOUT_SECONDS", 30),
		MaxJobResults:      getEnvInt("MAX_JOB_RESULTS", 50),

		// Authentication
		JWTSecret:      getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		JWTExpiryHours: getEnvInt("JWT_EXPIRY_HOURS", 24),
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),

		// Cloud Storage
		CVBucketName: getEnv("CV_BUCKET_NAME", ""),
	}

	return cfg
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	// ProjectID is required for Vertex AI
	if c.ProjectID == "" {
		return &ConfigError{Field: "PROJECT_ID", Message: "PROJECT_ID is required for Vertex AI"}
	}

	// PSE credentials are required for job search
	if c.PSEAPIKey == "" {
		return &ConfigError{Field: "PSE_API_KEY", Message: "PSE_API_KEY is required for job search"}
	}
	if c.PSEEngineID == "" {
		return &ConfigError{Field: "PSE_ENGINE_ID", Message: "PSE_ENGINE_ID is required for job search"}
	}

	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
