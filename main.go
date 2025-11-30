package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/myjobmatch/backend/agent"
	"github.com/myjobmatch/backend/auth"
	"github.com/myjobmatch/backend/config"
	_ "github.com/myjobmatch/backend/docs"
	"github.com/myjobmatch/backend/gemini"
	"github.com/myjobmatch/backend/handlers"
	"github.com/myjobmatch/backend/mcp"
	"github.com/myjobmatch/backend/storage"
	"github.com/myjobmatch/backend/tools"
)

// @title MyJobMatch API
// @version 1.0
// @description AI-powered job matching backend with CV parsing, job search, and ranking capabilities.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@myjobmatch.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load .env file if present (for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Set Gin mode based on debug setting
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create context for initialization
	ctx := context.Background()

	// Initialize Firestore client
	log.Println("Initializing Firestore client...")
	firestoreClient, err := storage.NewFirestoreClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Firestore client: %v", err)
	}
	defer firestoreClient.Close()
	log.Println("Firestore client initialized successfully")

	// Initialize Cloud Storage client
	log.Println("Initializing Cloud Storage client...")
	storageClient, err := storage.NewCloudStorageClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Cloud Storage client: %v", err)
	}
	defer storageClient.Close()
	log.Println("Cloud Storage client initialized successfully")

	// Initialize auth services
	jwtService := auth.NewJWTService(cfg)
	googleAuthService := auth.NewGoogleAuthService(cfg)

	// Initialize the job agent
	log.Println("Initializing job agent...")
	jobAgent, err := agent.NewJobAgent(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize job agent: %v", err)
	}
	defer jobAgent.Close()
	log.Println("Job agent initialized successfully")

	// Create handlers
	searchHandler := handlers.NewSearchHandler(jobAgent, firestoreClient, storageClient)
	cvHandler := handlers.NewCVHandler(jobAgent)
	authHandler := handlers.NewAuthHandler(firestoreClient, jwtService, googleAuthService)

	// Create MCP server with tool registry
	geminiClient, err := gemini.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create Gemini client for MCP: %v", err)
	}
	defer geminiClient.Close()

	toolRegistry := tools.NewToolRegistry()
	toolRegistry.Register(tools.NewSearchWebTool(cfg))
	toolRegistry.Register(tools.NewFetchPageTool(cfg))
	toolRegistry.Register(tools.NewExtractJobTool(geminiClient))
	toolRegistry.Register(tools.NewScoreJobTool(geminiClient))
	toolRegistry.Register(tools.NewParseCVTool(geminiClient))

	mcpServer := mcp.NewServer(toolRegistry)

	// Create Gin router
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Configure CORS for Vue frontend
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173", "*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Register routes
	router.GET("/health", handlers.HealthCheck)

	api := router.Group("/api")
	{
		// Auth endpoints (public)
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/google", authHandler.GoogleLogin)
		}

		// Protected auth endpoints (require authentication)
		authProtected := api.Group("/auth")
		authProtected.Use(auth.AuthMiddleware(jwtService))
		{
			authProtected.GET("/profile", authHandler.GetProfile)
			authProtected.PUT("/profile", authHandler.UpdateProfile)
			authProtected.POST("/cv", func(c *gin.Context) {
				authHandler.UploadCV(c, storageClient)
			})
		}

		// Job search endpoint (optional auth - uses saved CV if authenticated)
		api.POST("/search-jobs", auth.OptionalAuthMiddleware(jwtService), searchHandler.SearchJobs)

		// CV parsing endpoint
		api.POST("/parse-cv", cvHandler.ParseCV)

		// Tools introspection endpoint
		api.GET("/tools", searchHandler.GetTools)

		// MCP endpoints for external AI agents
		mcpServer.RegisterRoutes(api)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on port %s...", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
