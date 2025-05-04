package filesystem

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// FileSystemStorage implements the FileStorage interface using the local filesystem
type FileSystemStorage struct {
	rootDir     string
	baseURL     string
}

// NewFileSystemStorage creates a new file system storage
func NewFileSystemStorage(rootDir, baseURL string) (*FileSystemStorage, error) {
	// Ensure the root directory exists
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	return &FileSystemStorage{
		rootDir: rootDir,
		baseURL: baseURL,
	}, nil
}

// GenerateUploadURL creates a URL for uploading a file
// In a real implementation, this would be more sophisticated,
// possibly using signed URLs or a separate API endpoint
func (fs *FileSystemStorage) GenerateUploadURL(ctx context.Context, path string, contentType string, expiresIn time.Duration) (string, error) {
	// Create any necessary directories
	fullPath := filepath.Join(fs.rootDir, path)
	dir := filepath.Dir(fullPath)
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	
	// In a real implementation, we would use signed URLs or a separate API
	// For this implementation, we'll just return a URL pointing to our API endpoint
	// that would handle the upload
	return fmt.Sprintf("%s/upload?path=%s&contentType=%s", 
		fs.baseURL, 
		url.QueryEscape(path), 
		url.QueryEscape(contentType)), nil
}

// GenerateDownloadURL creates a URL for downloading a file
func (fs *FileSystemStorage) GenerateDownloadURL(ctx context.Context, path string, expiresIn time.Duration) (string, error) {
	// Check if the file exists
	fullPath := filepath.Join(fs.rootDir, path)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %w", err)
	}
	
	// In a real implementation, we would generate a signed URL
	// For this implementation, we'll just return a URL to serve the file
	return fmt.Sprintf("%s/download/%s", fs.baseURL, url.PathEscape(path)), nil
}

// DeleteFile removes a file from the filesystem
func (fs *FileSystemStorage) DeleteFile(ctx context.Context, path string) error {
	fullPath := filepath.Join(fs.rootDir, path)
	
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// File doesn't exist, nothing to delete
		return nil
	}
	
	return os.Remove(fullPath)
}

// DeleteObject implements the S3Storage interface for backward compatibility
func (fs *FileSystemStorage) DeleteObject(ctx context.Context, key string) error {
	return fs.DeleteFile(ctx, key)
}

// SaveFile saves data to a file
func (fs *FileSystemStorage) SaveFile(path string, data []byte) error {
	fullPath := filepath.Join(fs.rootDir, path)
	
	// Create any necessary directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	return os.WriteFile(fullPath, data, 0644)
}

// ReadFile reads data from a file
func (fs *FileSystemStorage) ReadFile(path string) ([]byte, error) {
	fullPath := filepath.Join(fs.rootDir, path)
	return os.ReadFile(fullPath)
}

// GetFilePath returns the full path to a file
func (fs *FileSystemStorage) GetFilePath(path string) string {
	return filepath.Join(fs.rootDir, path)
}