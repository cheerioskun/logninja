package export

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cheerioskun/logninja/internal/models"
	"github.com/spf13/afero"
)

// Service handles export operations with directory structure preservation
type Service struct {
	fs afero.Fs
}

// NewService creates a new export service
func NewService(fs afero.Fs) *Service {
	return &Service{
		fs: fs,
	}
}

// ExportOptions contains configuration for export operations
type ExportOptions struct {
	DestinationPath   string
	PreserveStructure bool
	Overwrite         bool
}

// ExportSummary contains information about the export operation
type ExportSummary struct {
	FileCount       int
	TotalSize       int64
	SourcePath      string
	DestinationPath string
}

// GetExportSummary calculates what would be exported without actually exporting
func (s *Service) GetExportSummary(ws *models.WorkingSet, destPath string) (*ExportSummary, error) {
	if ws == nil || ws.Bundle == nil {
		return nil, fmt.Errorf("invalid working set")
	}

	summary := &ExportSummary{
		SourcePath:      ws.Bundle.Path,
		DestinationPath: destPath,
	}

	// Count selected files and calculate total size
	for _, file := range ws.Bundle.Files {
		if ws.IsFileSelected(file.Path) {
			summary.FileCount++
			summary.TotalSize += file.Size
		}
	}

	return summary, nil
}

// ExportWorkingSet exports all selected files from the working set to the destination
func (s *Service) ExportWorkingSet(ws *models.WorkingSet, opts ExportOptions) error {
	if ws == nil || ws.Bundle == nil {
		return fmt.Errorf("invalid working set")
	}

	// Create destination directory if it doesn't exist
	if err := s.fs.MkdirAll(opts.DestinationPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Export each selected file
	for _, file := range ws.Bundle.Files {
		if ws.IsFileSelected(file.Path) {
			if err := s.exportFile(ws.Bundle.Path, file.Path, opts); err != nil {
				return fmt.Errorf("failed to export file %s: %w", file.Path, err)
			}
		}
	}

	return nil
}

// exportFile copies a single file preserving directory structure
func (s *Service) exportFile(bundlePath, relativePath string, opts ExportOptions) error {
	// Source file path
	sourcePath := filepath.Join(bundlePath, relativePath)

	// Destination file path (preserving directory structure)
	destPath := filepath.Join(opts.DestinationPath, relativePath)

	// Create destination directory
	destDir := filepath.Dir(destPath)
	if err := s.fs.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// Check if destination exists and handle overwrite
	if !opts.Overwrite {
		if exists, err := afero.Exists(s.fs, destPath); err != nil {
			return fmt.Errorf("failed to check if destination exists: %w", err)
		} else if exists {
			return fmt.Errorf("destination file exists and overwrite is disabled: %s", destPath)
		}
	}

	// Copy the file
	if err := s.copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// copyFile copies a file from source to destination, preserving attributes
func (s *Service) copyFile(sourcePath, destPath string) error {
	// Open source file
	srcFile, err := s.fs.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file info for permissions and timestamps
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Create destination file
	destFile, err := s.fs.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy contents
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Preserve permissions
	if err := s.fs.Chmod(destPath, srcInfo.Mode()); err != nil {
		// Log warning but don't fail the operation
		// TODO: Add proper logging
	}

	// Preserve timestamps if possible
	if _, ok := s.fs.(*afero.OsFs); ok {
		if err := os.Chtimes(destPath, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
			// Log warning but don't fail the operation
			// TODO: Add proper logging
		}
	}

	return nil
}

// GetDefaultExportPath generates a default export path based on current working directory
func GetDefaultExportPath(bundlePath string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Extract the last component of the path for the suffix
	baseName := filepath.Base(bundlePath)

	// Handle case where cwd is root
	if baseName == "/" || baseName == "." {
		baseName = "bundle"
	}

	return filepath.Join(cwd, baseName+"_refined"), nil
}

// ValidateExportPath performs basic validation on the export path
func ValidateExportPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("export path cannot be empty")
	}

	// Check if path is absolute or relative
	if !filepath.IsAbs(path) {
		// Convert to absolute path for validation
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		path = absPath
	}

	// Check if parent directory exists
	parentDir := filepath.Dir(path)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		return fmt.Errorf("parent directory does not exist: %s", parentDir)
	}

	return nil
}
