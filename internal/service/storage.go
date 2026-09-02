package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// FileStorage defines the interface for file storage services
type FileStorage interface {
	// Save for saving a file, returns the file key/path and size
	Save(ctx context.Context, file io.Reader, filename string) (string, int64, error)
	// Get for retrieving a file as a ReadCloser
	Get(ctx context.Context, path string) (io.ReadCloser, error)
	// Delete removes the physical file
	Delete(ctx context.Context, path string) error
	// GetPublicURL for getting a public URL to access the file
	// For local storage, it may return a relative path
	GetPublicURL(path string) string
}

// LocalStorage local filesystem implementation
type LocalStorage struct {
	baseDir string
}

func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &LocalStorage{baseDir: baseDir}, nil
}

func (s *LocalStorage) Save(ctx context.Context, file io.Reader, filename string) (string, int64, error) {
	// Prevent path traversal
	cleanName := filepath.Base(filename)
	fpath := filepath.Join(s.baseDir, cleanName)

	out, err := os.Create(fpath)
	if err != nil {
		return "", 0, err
	}
	defer out.Close()

	size, err := io.Copy(out, file)
	if err != nil {
		return "", 0, err
	}

	// Local storage returns filename as Key
	return cleanName, size, nil
}

func (s *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fpath := filepath.Join(s.baseDir, path)
	return os.Open(fpath)
}

func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fpath := filepath.Join(s.baseDir, filepath.Base(path))
	if err := os.Remove(fpath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *LocalStorage) GetPublicURL(path string) string {
	// For local storage, return relative path
	return "/uploads/" + path
}
