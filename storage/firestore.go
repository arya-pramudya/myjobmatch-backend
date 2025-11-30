package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/myjobmatch/backend/config"
	"github.com/myjobmatch/backend/models"
)

const usersCollection = "users"

// FirestoreClient wraps Firestore operations
type FirestoreClient struct {
	client *firestore.Client
}

// NewFirestoreClient creates a new Firestore client
func NewFirestoreClient(ctx context.Context, cfg *config.Config) (*FirestoreClient, error) {
	client, err := firestore.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore client: %w", err)
	}

	return &FirestoreClient{client: client}, nil
}

// Close closes the Firestore client
func (f *FirestoreClient) Close() error {
	return f.client.Close()
}

// CreateUser creates a new user in Firestore
func (f *FirestoreClient) CreateUser(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	// Use email as document ID for uniqueness
	docRef := f.client.Collection(usersCollection).Doc(user.Email)

	// Check if user already exists
	_, err := docRef.Get(ctx)
	if err == nil {
		return errors.New("user with this email already exists")
	}
	if status.Code(err) != codes.NotFound {
		return fmt.Errorf("failed to check user existence: %w", err)
	}

	// Create user
	_, err = docRef.Set(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	user.ID = user.Email
	return nil
}

// GetUserByEmail retrieves a user by email
func (f *FirestoreClient) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	docRef := f.client.Collection(usersCollection).Doc(email)
	doc, err := docRef.Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	user.ID = doc.Ref.ID
	return &user, nil
}

// GetUserByGoogleID retrieves a user by Google ID
func (f *FirestoreClient) GetUserByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	iter := f.client.Collection(usersCollection).Where("googleId", "==", googleID).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	user.ID = doc.Ref.ID
	return &user, nil
}

// UpdateUser updates user data
func (f *FirestoreClient) UpdateUser(ctx context.Context, email string, updates map[string]interface{}) error {
	updates["updatedAt"] = time.Now()

	docRef := f.client.Collection(usersCollection).Doc(email)
	_, err := docRef.Set(ctx, updates, firestore.MergeAll)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdateUserCVUrl updates user's CV URL
func (f *FirestoreClient) UpdateUserCVUrl(ctx context.Context, email, cvUrl string) error {
	return f.UpdateUser(ctx, email, map[string]interface{}{
		"cvUrl": cvUrl,
	})
}

// UpdateUserProfile updates user's profile (nama)
func (f *FirestoreClient) UpdateUserProfile(ctx context.Context, email string, nama string) error {
	updates := map[string]interface{}{}
	if nama != "" {
		updates["nama"] = nama
	}

	if len(updates) == 0 {
		return nil
	}

	return f.UpdateUser(ctx, email, updates)
}

// DeleteUser deletes a user
func (f *FirestoreClient) DeleteUser(ctx context.Context, email string) error {
	docRef := f.client.Collection(usersCollection).Doc(email)
	_, err := docRef.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}
