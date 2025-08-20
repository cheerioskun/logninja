package scanner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cheerioskun/logninja/internal/models"
	"github.com/spf13/afero"
)

// BundleScanner handles bundle discovery and file enumeration
type BundleScanner struct {
	fs       afero.Fs
	maxDepth int
	logExts  map[string]bool
}

// NewBundleScanner creates a new BundleScanner with the given filesystem
func NewBundleScanner(fs afero.Fs) *BundleScanner {
	return &BundleScanner{
		fs:       fs,
		maxDepth: 10, // Default max depth
		logExts: map[string]bool{
			".log":  true,
			".txt":  true,
			".out":  true,
			".err":  true,
			".json": true,
		},
	}
}

// SetMaxDepth sets the maximum scanning depth
func (bs *BundleScanner) SetMaxDepth(depth int) {
	bs.maxDepth = depth
}

// AddLogExtension adds a file extension to be considered as log files
func (bs *BundleScanner) AddLogExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	bs.logExts[ext] = true
}

// ScanBundle scans a directory and returns a Bundle with all discovered files
func (bs *BundleScanner) ScanBundle(path string) (*models.Bundle, error) {
	// Verify path exists and is a directory
	info, err := bs.fs.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access path %s: %w", path, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	bundle := models.NewBundle(path, bs.fs)

	// Recursively scan the directory
	err = bs.scanDirectory(path, "", 0, bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to scan bundle: %w", err)
	}

	// Update bundle metadata
	bs.updateBundleMetadata(bundle)

	return bundle, nil
}

// scanDirectory recursively scans a directory and adds files to the bundle
func (bs *BundleScanner) scanDirectory(basePath, relativePath string, depth int, bundle *models.Bundle) error {
	if depth > bs.maxDepth {
		return nil // Skip if max depth exceeded
	}

	currentPath := filepath.Join(basePath, relativePath)

	entries, err := afero.ReadDir(bs.fs, currentPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", currentPath, err)
	}

	for _, entry := range entries {
		entryRelPath := filepath.Join(relativePath, entry.Name())
		entryFullPath := filepath.Join(basePath, entryRelPath)

		if entry.IsDir() {
			// Recursively scan subdirectory
			err := bs.scanDirectory(basePath, entryRelPath, depth+1, bundle)
			if err != nil {
				// Log warning but continue scanning
				fmt.Printf("Warning: failed to scan directory %s: %v\n", entryRelPath, err)
			}
		} else {
			// Process file
			fileInfo, err := bs.processFile(entryFullPath, entryRelPath, entry)
			if err != nil {
				// Log warning but continue scanning
				fmt.Printf("Warning: failed to process file %s: %v\n", entryRelPath, err)
				continue
			}

			bundle.AddFile(*fileInfo)
		}
	}

	return nil
}

// processFile processes a single file and returns FileInfo
func (bs *BundleScanner) processFile(fullPath, relativePath string, info os.FileInfo) (*models.FileInfo, error) {
	fileInfo := &models.FileInfo{
		Path:         relativePath,
		Size:         info.Size(),
		IsLogFile:    bs.isLogFile(relativePath),
		TimeRange:    nil, // Will be populated by log parser if needed
		LineCount:    0,   // Will be estimated if needed
		Selected:     false,
		LastModified: info.ModTime(),
	}

	// For log files, estimate line count and check for timestamps
	if fileInfo.IsLogFile {
		lineCount, err := bs.estimateLineCount(fullPath)
		if err == nil {
			fileInfo.LineCount = lineCount
		}

		// For now, we'll set IsLogFile to true for files with log extensions
		// Timestamp extraction will be handled by the parser in Phase 2
	}

	return fileInfo, nil
}

// isLogFile determines if a file should be considered a log file
func (bs *BundleScanner) isLogFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Check known log extensions
	if bs.logExts[ext] {
		return true
	}

	// Check common log file patterns
	filename := strings.ToLower(filepath.Base(path))
	logPatterns := []string{
		"log", "logs", "syslog", "messages", "access", "error", "debug",
		"trace", "audit", "event", "console", "output", "stderr", "stdout",
	}

	for _, pattern := range logPatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}

	return false
}

// estimateLineCount provides a rough estimate of lines in a file
func (bs *BundleScanner) estimateLineCount(path string) (int64, error) {
	file, err := bs.fs.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Sample first 64KB to estimate line count
	const sampleSize = 64 * 1024
	buffer := make([]byte, sampleSize)

	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return 0, err
	}

	if n == 0 {
		return 0, nil
	}

	// Count newlines in sample
	lines := int64(0)
	for i := 0; i < n; i++ {
		if buffer[i] == '\n' {
			lines++
		}
	}

	// Get file size for estimation
	stat, err := file.Stat()
	if err != nil {
		return lines, nil // Return sample count if we can't get file size
	}

	fileSize := stat.Size()

	// If we read the entire file, return exact count
	if int64(n) >= fileSize {
		return lines, nil
	}

	// Estimate total lines based on sample
	if lines > 0 {
		estimatedLines := (lines * fileSize) / int64(n)
		return estimatedLines, nil
	}

	return 0, nil
}

// updateBundleMetadata updates the bundle metadata after scanning
func (bs *BundleScanner) updateBundleMetadata(bundle *models.Bundle) {
	bundle.Metadata.ScanDepth = bs.maxDepth

	// Metadata is already updated by Bundle.AddFile() method
	// This method can be extended for additional metadata processing
}

// GetSupportedExtensions returns a list of supported log file extensions
func (bs *BundleScanner) GetSupportedExtensions() []string {
	var exts []string
	for ext := range bs.logExts {
		exts = append(exts, ext)
	}
	return exts
}

// QuickScan performs a quick scan to get basic bundle information without deep analysis
func (bs *BundleScanner) QuickScan(path string) (*models.BundleMetadata, error) {
	info, err := bs.fs.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access path %s: %w", path, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	metadata := &models.BundleMetadata{
		LogFileCount:   0,
		TotalFileCount: 0,
		CommonFormats:  make([]string, 0),
		ScanDepth:      1, // Quick scan only goes 1 level deep
	}

	entries, err := afero.ReadDir(bs.fs, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			metadata.TotalFileCount++
			if bs.isLogFile(entry.Name()) {
				metadata.LogFileCount++
			}
		}
	}

	return metadata, nil
}
