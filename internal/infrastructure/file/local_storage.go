package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type LocalStorageProvider struct {
	basePath string
}

func NewLocalStorageProvider(basePath string) *LocalStorageProvider {
	// Create the uploads directory if it doesn't exist
	_ = os.MkdirAll(basePath, 0755)
	
	return &LocalStorageProvider{
		basePath: basePath,
	}
}

func (s *LocalStorageProvider) SaveFile(ctx context.Context, fileBytes []byte, ext string) (string, error) {
	// Generate secure and unguessable UUID for the file
	fileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	fullPath := filepath.Join(s.basePath, fileName)

	// Save to disk
	err := os.WriteFile(fullPath, fileBytes, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file to disk: %w", err)
	}

	// Return a secure URL-friendly path, e.g. /uploads/uuid.jpg
	return fmt.Sprintf("/uploads/%s", fileName), nil
}
