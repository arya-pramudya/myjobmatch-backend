package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/myjobmatch/backend/auth"
	"github.com/myjobmatch/backend/models"
	"github.com/myjobmatch/backend/storage"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	firestoreClient *storage.FirestoreClient
	jwtService      *auth.JWTService
	googleAuth      *auth.GoogleAuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	firestoreClient *storage.FirestoreClient,
	jwtService *auth.JWTService,
	googleAuth *auth.GoogleAuthService,
) *AuthHandler {
	return &AuthHandler{
		firestoreClient: firestoreClient,
		jwtService:      jwtService,
		googleAuth:      googleAuth,
	}
}

// Register handles user registration with email/password
// @Summary Register a new user
// @Description Register a new user with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "Registration request"
// @Success 201 {object} models.AuthResponse "Registration successful"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 409 {object} models.ErrorResponse "User already exists"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Code:    http.StatusBadRequest,
			Details: err.Error(),
		})
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("[AuthHandler] Failed to hash password: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to process registration",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Create user
	user := &models.User{
		Email:    req.Email,
		Nama:     req.Nama,
		Password: hashedPassword,
		Provider: "email",
		CVUrl:    "",
	}

	if err := h.firestoreClient.CreateUser(c.Request.Context(), user); err != nil {
		log.Printf("[AuthHandler] Failed to create user: %v", err)
		c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "Registration failed",
			Code:    http.StatusConflict,
			Details: err.Error(),
		})
		return
	}

	// Generate JWT token
	token, err := h.jwtService.GenerateToken(user)
	if err != nil {
		log.Printf("[AuthHandler] Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to generate token",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	log.Printf("[AuthHandler] User registered: %s", user.Email)
	c.JSON(http.StatusCreated, models.AuthResponse{
		Token:   token,
		User:    user,
		Message: "Registration successful",
	})
}

// Login handles user login with email/password
// @Summary Login user
// @Description Login with email and password to get JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login request"
// @Success 200 {object} models.AuthResponse "Login successful"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Invalid credentials"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Code:    http.StatusBadRequest,
			Details: err.Error(),
		})
		return
	}

	// Get user by email
	user, err := h.firestoreClient.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid email or password",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	// Check if user registered with Google
	if user.Provider == "google" {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "This account uses Google Sign-In. Please login with Google.",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	// Verify password
	if !auth.CheckPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Invalid email or password",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	// Generate JWT token
	token, err := h.jwtService.GenerateToken(user)
	if err != nil {
		log.Printf("[AuthHandler] Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to generate token",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	log.Printf("[AuthHandler] User logged in: %s", user.Email)
	c.JSON(http.StatusOK, models.AuthResponse{
		Token:   token,
		User:    user,
		Message: "Login successful",
	})
}

// GoogleLogin handles Google SSO authentication
// @Summary Login with Google
// @Description Login or register using Google SSO ID token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body models.GoogleAuthRequest true "Google auth request"
// @Success 200 {object} models.AuthResponse "Login successful"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Invalid Google token"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/google [post]
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req models.GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Code:    http.StatusBadRequest,
			Details: err.Error(),
		})
		return
	}

	// Verify Google ID token
	googleUser, err := h.googleAuth.VerifyIDToken(c.Request.Context(), req.IDToken)
	if err != nil {
		log.Printf("[AuthHandler] Failed to verify Google token: %v", err)
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error:   "Invalid Google token",
			Code:    http.StatusUnauthorized,
			Details: err.Error(),
		})
		return
	}

	// Check if user exists
	user, err := h.firestoreClient.GetUserByEmail(c.Request.Context(), googleUser.Email)
	if err != nil {
		// User doesn't exist, create new user
		user = &models.User{
			Email:    googleUser.Email,
			Nama:     googleUser.Name,
			Password: "", // No password for Google users
			Provider: "google",
			GoogleID: googleUser.GoogleID,
			CVUrl:    "",
		}

		if err := h.firestoreClient.CreateUser(c.Request.Context(), user); err != nil {
			log.Printf("[AuthHandler] Failed to create Google user: %v", err)
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to create account",
				Code:    http.StatusInternalServerError,
				Details: err.Error(),
			})
			return
		}
		log.Printf("[AuthHandler] New Google user created: %s", user.Email)
	} else {
		// User exists, update Google ID if not set
		if user.GoogleID == "" {
			h.firestoreClient.UpdateUser(c.Request.Context(), user.Email, map[string]interface{}{
				"googleId": googleUser.GoogleID,
				"provider": "google",
			})
		}
	}

	// Generate JWT token
	token, err := h.jwtService.GenerateToken(user)
	if err != nil {
		log.Printf("[AuthHandler] Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to generate token",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	log.Printf("[AuthHandler] Google user logged in: %s", user.Email)
	c.JSON(http.StatusOK, models.AuthResponse{
		Token:   token,
		User:    user,
		Message: "Login successful",
	})
}

// GetProfile retrieves the current user's profile
// @Summary Get user profile
// @Description Get the authenticated user's profile information
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.ProfileResponse "User profile"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Router /auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	claims := auth.GetAuthClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	user, err := h.firestoreClient.GetUserByEmail(c.Request.Context(), claims.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "User not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	c.JSON(http.StatusOK, models.ProfileResponse{
		User: user,
	})
}

// UpdateProfile updates the current user's profile
// @Summary Update user profile
// @Description Update the authenticated user's profile (name)
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.UpdateProfileRequest true "Update profile request"
// @Success 200 {object} models.ProfileResponse "Profile updated"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/profile [put]
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	claims := auth.GetAuthClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid request body",
			Code:    http.StatusBadRequest,
			Details: err.Error(),
		})
		return
	}

	// Update profile
	if err := h.firestoreClient.UpdateUserProfile(c.Request.Context(), claims.Email, req.Nama); err != nil {
		log.Printf("[AuthHandler] Failed to update profile: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update profile",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	// Get updated user
	user, err := h.firestoreClient.GetUserByEmail(c.Request.Context(), claims.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "User not found",
			Code:  http.StatusNotFound,
		})
		return
	}

	log.Printf("[AuthHandler] Profile updated: %s", claims.Email)
	c.JSON(http.StatusOK, models.ProfileResponse{
		User:    user,
		Message: "Profile updated successfully",
	})
}

// UploadCV uploads a CV file for the authenticated user
// @Summary Upload CV
// @Description Upload a CV file (PDF, DOC, DOCX) to user's profile
// @Tags Auth
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param cv_file formData file true "CV file (PDF, DOC, DOCX)"
// @Success 200 {object} models.CVUploadResponse "CV uploaded successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid file"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/cv [post]
func (h *AuthHandler) UploadCV(c *gin.Context, storageClient *storage.CloudStorageClient) {
	claims := auth.GetAuthClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "Unauthorized",
			Code:  http.StatusUnauthorized,
		})
		return
	}

	// Get file from form
	file, header, err := c.Request.FormFile("cv_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "CV file is required",
			Code:    http.StatusBadRequest,
			Details: err.Error(),
		})
		return
	}
	defer file.Close()

	// Upload to Cloud Storage
	cvUrl, err := storageClient.UploadCV(c.Request.Context(), claims.Email, file, header)
	if err != nil {
		log.Printf("[AuthHandler] Failed to upload CV: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Failed to upload CV",
			Code:    http.StatusInternalServerError,
			Details: err.Error(),
		})
		return
	}

	// Update user's CV URL in Firestore
	if err := h.firestoreClient.UpdateUserCVUrl(c.Request.Context(), claims.Email, cvUrl); err != nil {
		log.Printf("[AuthHandler] Failed to update CV URL: %v", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to save CV reference",
			Code:  http.StatusInternalServerError,
		})
		return
	}

	log.Printf("[AuthHandler] CV uploaded for user: %s", claims.Email)
	c.JSON(http.StatusOK, models.CVUploadResponse{
		CVUrl:   cvUrl,
		Message: "CV uploaded successfully",
	})
}
