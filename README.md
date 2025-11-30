# MyJobMatch Backend

AI-powered job-hunting agent backend built with Go, ADK (Agent Development Kit), and MCP Tools.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Vue Frontend                                │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Go Backend (Cloud Run)                            │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                   /api/search-jobs Handler                    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                    │                                 │
│                                    ▼                                 │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                     ADK Job Agent                             │   │
│  │  ┌───────────────┐  ┌───────────────┐  ┌─────────────────┐  │   │
│  │  │ search_web    │  │ fetch_page    │  │ extract_job     │  │   │
│  │  │ (PSE Tool)    │  │ (HTTP Tool)   │  │ (Gemini Tool)   │  │   │
│  │  └───────────────┘  └───────────────┘  └─────────────────┘  │   │
│  │  ┌───────────────┐  ┌───────────────┐                       │   │
│  │  │ score_job     │  │ parse_cv      │                       │   │
│  │  │ (Gemini Tool) │  │ (Gemini Tool) │                       │   │
│  │  └───────────────┘  └───────────────┘                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

## Features

- **CV Analysis**: Upload PDF/Word or paste CV text for skill extraction
- **Smart Search**: Natural language job search with filters
- **Multi-mode Input**: Use CV only, search text only, or both combined
- **Job Ranking**: AI-powered matching scores based on user profile
- **MCP Tools**: Modular tools for web search, page fetching, and extraction
- **User Authentication**: Email/password and Google SSO login
- **CV Storage**: Upload and store CVs in Google Cloud Storage
- **API Documentation**: Swagger/OpenAPI documentation

## Project Structure

```
myjobmatch-backend/
├── main.go                 # Entry point
├── config/
│   └── config.go          # Configuration management
├── models/
│   ├── user.go            # UserProfile, Filters
│   ├── job.go             # JobPosting, RankedJob
│   └── request.go         # API request/response types
├── gemini/
│   └── client.go          # Vertex AI Gemini client
├── tools/
│   ├── base.go            # MCP tool interface
│   ├── search_web.go      # PSE job search tool
│   ├── fetch_page.go      # HTTP page fetcher tool
│   ├── extract_job.go     # Gemini job extraction tool
│   ├── score_job.go       # Gemini job scoring tool
│   └── parse_cv.go        # Gemini CV parsing tool
├── agent/
│   └── job_agent.go       # ADK agent orchestration
├── handlers/
│   └── search.go          # HTTP handlers
├── Dockerfile
├── .env.example
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.22+
- Google Cloud Project with Vertex AI API enabled
- Programmable Search Engine (PSE) API key
- Firestore database
- Cloud Storage bucket

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/myjobmatch-backend.git
cd myjobmatch-backend

# Install dependencies
go mod tidy

# Install swag CLI for Swagger documentation
go install github.com/swaggo/swag/cmd/swag@latest

# Generate Swagger documentation
swag init

# Copy environment template
cp .env.example .env
# Edit .env with your configuration

# Run the server
go run main.go
```

### Swagger Documentation

Once the server is running, access the API documentation at:
- **Swagger UI**: http://localhost:8080/swagger/index.html

To regenerate documentation after making changes to API annotations:
```bash
swag init
```

## Environment Variables

```env
# Google Cloud
PROJECT_ID=your-gcp-project-id
LOCATION=us-central1

# Programmable Search Engine
PSE_API_KEY=your-pse-api-key
PSE_ENGINE_ID=your-search-engine-id

# Server
PORT=8080

# Authentication
JWT_SECRET=your-secret-key
JWT_EXPIRY_HOURS=24
GOOGLE_CLIENT_ID=your-google-client-id

# Cloud Storage
CV_BUCKET_NAME=your-cv-bucket
```

## API Endpoints

### POST /api/search-jobs

Search for jobs based on CV, query, or both.

**Request (JSON):**
```json
{
  "cvText": "optional: pasted CV text",
  "query": "optional: job search text",
  "filters": {
    "locations": ["Jakarta"],
    "remote_modes": ["WFH", "Hybrid"],
    "job_types": ["full_time"]
  }
}
```

**Request (multipart/form-data):**
- `cv_file`: PDF or Word document
- `query`: Job search text
- `filters`: JSON string of filters

**Response:**
```json
{
  "results": [
    {
      "title": "Senior Golang Backend Engineer",
      "company": "TechCorp",
      "description": "We are looking for...",
      "location": "Jakarta / Remote",
      "work_type": "full_time",
      "site_setting": "Hybrid",
      "url": "https://example.com/job/123",
      "match_score": 92,
      "match_reason": "Strong match on Golang, microservices...",
      "source": "web",
      "tags": ["golang", "backend"]
    }
  ],
  "profile": {
    "skills": ["Go", "Python", "Kubernetes"],
    "preferred_locations": ["Jakarta"],
    "preferred_remote_modes": ["WFH", "Hybrid"]
  }
}
```

## Running Locally

```bash
# Install dependencies
go mod tidy

# Set environment variables
cp .env.example .env
# Edit .env with your credentials

# Run the server
go run main.go
```

## Deploying to Cloud Run

```bash
# Build and deploy
gcloud run deploy myjobmatch-backend \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars PROJECT_ID=your-project-id,LOCATION=us-central1
```

## MCP Tools

### 1. search_web_for_jobs
Uses Google Programmable Search Engine to find job posting URLs.

### 2. fetch_page_html
Fetches HTML content from job posting URLs.

### 3. extract_job_from_html
Uses Gemini to extract structured job data from HTML.

### 4. score_job_match
Uses Gemini to score job-profile compatibility (0-100).

### 5. parse_cv
Uses Gemini to extract structured profile from CV text.

## License

MIT
