package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"

	"github.com/myjobmatch/backend/config"
)

// CloudStorageClient wraps Google Cloud Storage operations
type CloudStorageClient struct {
	client     *storage.Client
	bucketName string
}

// NewCloudStorageClient creates a new Cloud Storage client
func NewCloudStorageClient(ctx context.Context, cfg *config.Config) (*CloudStorageClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloud Storage client: %w", err)
	}

	return &CloudStorageClient{
		client:     client,
		bucketName: cfg.CVBucketName,
	}, nil
}

// Close closes the Cloud Storage client
func (c *CloudStorageClient) Close() error {
	return c.client.Close()
}

// UploadCV uploads a CV file to Cloud Storage
func (c *CloudStorageClient) UploadCV(ctx context.Context, userEmail string, file multipart.File, header *multipart.FileHeader) (string, error) {
	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	timestamp := time.Now().Unix()

	// Sanitize email for use in path
	sanitizedEmail := strings.ReplaceAll(userEmail, "@", "_at_")
	sanitizedEmail = strings.ReplaceAll(sanitizedEmail, ".", "_")

	objectName := fmt.Sprintf("cvs/%s/%d%s", sanitizedEmail, timestamp, ext)

	// Get bucket handle
	bucket := c.client.Bucket(c.bucketName)
	obj := bucket.Object(objectName)

	// Create writer
	wc := obj.NewWriter(ctx)
	wc.ContentType = header.Header.Get("Content-Type")
	if wc.ContentType == "" {
		wc.ContentType = getContentType(ext)
	}

	// Copy file content
	if _, err := io.Copy(wc, file); err != nil {
		wc.Close()
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Close writer
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// Generate public URL or signed URL
	url := fmt.Sprintf("https://storage.googleapis.com/%s/%s", c.bucketName, objectName)

	return url, nil
}

// UploadCVFromBytes uploads CV content from bytes
func (c *CloudStorageClient) UploadCVFromBytes(ctx context.Context, userEmail string, content []byte, filename string) (string, error) {
	ext := filepath.Ext(filename)
	timestamp := time.Now().Unix()

	sanitizedEmail := strings.ReplaceAll(userEmail, "@", "_at_")
	sanitizedEmail = strings.ReplaceAll(sanitizedEmail, ".", "_")

	objectName := fmt.Sprintf("cvs/%s/%d%s", sanitizedEmail, timestamp, ext)

	bucket := c.client.Bucket(c.bucketName)
	obj := bucket.Object(objectName)

	wc := obj.NewWriter(ctx)
	wc.ContentType = getContentType(ext)

	if _, err := wc.Write(content); err != nil {
		wc.Close()
		return "", fmt.Errorf("failed to write content: %w", err)
	}

	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	url := fmt.Sprintf("https://storage.googleapis.com/%s/%s", c.bucketName, objectName)
	return url, nil
}

// DeleteCV deletes a CV file from Cloud Storage
func (c *CloudStorageClient) DeleteCV(ctx context.Context, cvUrl string) error {
	// Extract object name from URL
	prefix := fmt.Sprintf("https://storage.googleapis.com/%s/", c.bucketName)
	if !strings.HasPrefix(cvUrl, prefix) {
		return fmt.Errorf("invalid CV URL format")
	}

	objectName := strings.TrimPrefix(cvUrl, prefix)

	bucket := c.client.Bucket(c.bucketName)
	obj := bucket.Object(objectName)

	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete CV: %w", err)
	}

	return nil
}

// GetSignedURL generates a signed URL for temporary access
func (c *CloudStorageClient) GetSignedURL(ctx context.Context, objectName string, expiration time.Duration) (string, error) {
	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(expiration),
	}

	url, err := c.client.Bucket(c.bucketName).SignedURL(objectName, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}

// DownloadCV downloads CV content
func (c *CloudStorageClient) DownloadCV(ctx context.Context, cvUrl string) ([]byte, error) {
	prefix := fmt.Sprintf("https://storage.googleapis.com/%s/", c.bucketName)
	if !strings.HasPrefix(cvUrl, prefix) {
		return nil, fmt.Errorf("invalid CV URL format")
	}

	objectName := strings.TrimPrefix(cvUrl, prefix)

	bucket := c.client.Bucket(c.bucketName)
	obj := bucket.Object(objectName)

	rc, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read CV: %w", err)
	}

	return data, nil
}

func getContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
