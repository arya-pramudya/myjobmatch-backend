package auth

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/api/idtoken"

	"github.com/myjobmatch/backend/config"
)

// GoogleAuthService handles Google SSO verification
type GoogleAuthService struct {
	clientID string
}

// GoogleUserInfo represents user info from Google token
type GoogleUserInfo struct {
	GoogleID string
	Email    string
	Name     string
	Picture  string
}

// NewGoogleAuthService creates a new Google auth service
func NewGoogleAuthService(cfg *config.Config) *GoogleAuthService {
	return &GoogleAuthService{
		clientID: cfg.GoogleClientID,
	}
}

// VerifyIDToken verifies a Google ID token and returns user info
func (s *GoogleAuthService) VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	if s.clientID == "" {
		return nil, errors.New("Google Client ID not configured")
	}

	payload, err := idtoken.Validate(ctx, idToken, s.clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract user info from payload
	userInfo := &GoogleUserInfo{
		GoogleID: payload.Subject,
	}

	if email, ok := payload.Claims["email"].(string); ok {
		userInfo.Email = email
	}
	if name, ok := payload.Claims["name"].(string); ok {
		userInfo.Name = name
	}
	if picture, ok := payload.Claims["picture"].(string); ok {
		userInfo.Picture = picture
	}

	if userInfo.Email == "" {
		return nil, errors.New("email not found in token")
	}

	return userInfo, nil
}
