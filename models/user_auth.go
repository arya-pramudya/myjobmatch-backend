package models

import "time"

// User represents a user in Firestore
// @Description User account information
type User struct {
	ID        string    `json:"id" firestore:"-" example:"user@example.com"`
	Email     string    `json:"email" firestore:"email" example:"user@example.com"`
	Nama      string    `json:"nama" firestore:"nama" example:"John Doe"`
	Password  string    `json:"-" firestore:"password"` // Hashed password, never sent to client
	CVUrl     string    `json:"cvUrl" firestore:"cvUrl" example:"gs://bucket/cvs/user@example.com/resume.pdf"`
	Provider  string    `json:"provider" firestore:"provider" example:"email"` // "email" or "google"
	GoogleID  string    `json:"-" firestore:"googleId,omitempty"`
	CreatedAt time.Time `json:"createdAt" firestore:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" firestore:"updatedAt"`
}

// RegisterRequest represents registration request
// @Description User registration request
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required,min=6" example:"password123"`
	Nama     string `json:"nama" binding:"required" example:"John Doe"`
}

// LoginRequest represents login request
// @Description User login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"password123"`
}

// GoogleAuthRequest represents Google SSO authentication request
// @Description Google SSO authentication request
type GoogleAuthRequest struct {
	IDToken string `json:"idToken" binding:"required" example:"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// UpdateProfileRequest represents profile update request
// @Description Profile update request
type UpdateProfileRequest struct {
	Nama string `json:"nama,omitempty" example:"John Smith"`
}

// AuthResponse represents authentication response
// @Description Authentication response with JWT token
type AuthResponse struct {
	Token   string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User    *User  `json:"user"`
	Message string `json:"message,omitempty" example:"Login successful"`
}

// ProfileResponse represents user profile response
// @Description User profile response
type ProfileResponse struct {
	User    *User  `json:"user"`
	Message string `json:"message,omitempty" example:"Profile updated successfully"`
}

// CVUploadResponse represents CV upload response
// @Description CV upload response
type CVUploadResponse struct {
	CVUrl   string `json:"cvUrl" example:"gs://bucket/cvs/user@example.com/resume.pdf"`
	Message string `json:"message" example:"CV uploaded successfully"`
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Nama   string `json:"nama"`
}
