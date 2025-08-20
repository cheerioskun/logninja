package scanner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/internal/parser"
	"github.com/cheerioskun/logninja/internal/utils"
	"github.com/spf13/afero"
)

const (
	// PeakLines is the number of lines we examine to determine if a file is a log file
	PeakLines = 10
)

// BundleScanner handles bundle discovery and file enumeration with a two-phase approach:
// Phase 1: Collect ALL files using WalkDir
// Phase 2: Filter files based on content peeking only
type BundleScanner struct {
	fs                 afero.Fs
	maxDepth           int
	timestampExtractor *parser.TimestampExtractor

	// Two-phase approach data
	allFiles   []string // Complete file set from WalkDir
	workingSet []string // Filtered log files after content analysis
}

// NewBundleScanner creates a new BundleScanner with the given filesystem
func NewBundleScanner(fs afero.Fs) *BundleScanner {
	return &BundleScanner{
		fs:                 fs,
		maxDepth:           10, // Default max depth
		timestampExtractor: parser.NewTimestampExtractor(fs),
		allFiles:           make([]string, 0),
		workingSet:         make([]string, 0),
	}
}

// SetMaxDepth sets the maximum scanning depth
func (bs *BundleScanner) SetMaxDepth(depth int) {
	bs.maxDepth = depth
}

// ScanBundle scans a directory using a two-phase approach and returns a Bundle with all discovered files
func (bs *BundleScanner) ScanBundle(path string) (*models.Bundle, error) {
	// Verify path exists and is a directory
	info, err := bs.fs.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access path %s: %w", path, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", path)
	}

	// Phase 1: Collect ALL files using WalkDir
	err = bs.collectAllFiles(path)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	// Phase 2: Filter files based on content peeking only
	err = bs.filterLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to filter log files: %w", err)
	}

	// Create bundle with working set
	bundle := models.NewBundle(path, bs.fs)
	err = bs.buildBundle(path, bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to build bundle: %w", err)
	}

	// Update bundle metadata
	bs.updateBundleMetadata(bundle)

	return bundle, nil
}

// collectAllFiles performs Phase 1: collect ALL files using WalkDir
func (bs *BundleScanner) collectAllFiles(basePath string) error {
	bs.allFiles = make([]string, 0) // Reset the file list

	// Use afero.Walk which is equivalent to filepath.WalkDir
	err := afero.Walk(bs.fs, basePath, func(fullPath string, info os.FileInfo, err error) error {
		if err != nil {
			// Log warning but continue scanning
			utils.Warning("failed to access %s: %v", fullPath, err)
			return nil // Continue walking
		}

		// Skip directories
		if info.IsDir() {
			// Check depth limit for directories
			relPath, err := filepath.Rel(basePath, fullPath)
			if err != nil {
				return nil // Continue on error
			}

			depth := len(filepath.SplitList(relPath))
			if relPath != "." && depth > bs.maxDepth {
				return filepath.SkipDir // Skip this directory and its contents
			}
			return nil // Continue into directory
		}

		// Add all regular files to our complete file set
		bs.allFiles = append(bs.allFiles, fullPath)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory %s: %w", basePath, err)
	}

	return nil
}

// filterLogFiles performs Phase 2: filter files based on content peeking only
func (bs *BundleScanner) filterLogFiles() error {
	bs.workingSet = make([]string, 0) // Reset the working set

	for _, filePath := range bs.allFiles {
		// Check if this file looks like a log file based on content only
		if bs.isLogFileByContent(filePath) {
			bs.workingSet = append(bs.workingSet, filePath)
		}
	}

	return nil
}

// buildBundle creates a Bundle from the working set
func (bs *BundleScanner) buildBundle(basePath string, bundle *models.Bundle) error {
	// Add all files (both log and non-log) to bundle for completeness
	for _, fullPath := range bs.allFiles {
		relPath, err := filepath.Rel(basePath, fullPath)
		if err != nil {
			utils.Warning("failed to get relative path for %s: %v", fullPath, err)
			continue
		}

		info, err := bs.fs.Stat(fullPath)
		if err != nil {
			utils.Warning("failed to stat file %s: %v", fullPath, err)
			continue
		}

		// Check if this file is in our working set (i.e., it's a log file)
		isLogFile := bs.isInWorkingSet(fullPath)

		fileInfo := &models.FileInfo{
			Path:         relPath,
			Size:         info.Size(),
			IsLogFile:    isLogFile,
			TimeRange:    nil, // Will be populated by log parser if needed
			Selected:     false,
			LastModified: info.ModTime(),
		}

		bundle.AddFile(*fileInfo)
	}

	return nil
}

// isInWorkingSet checks if a file path is in the working set
func (bs *BundleScanner) isInWorkingSet(filePath string) bool {
	for _, workingFile := range bs.workingSet {
		if workingFile == filePath {
			return true
		}
	}
	return false
}

// isLogFileByContent determines if a file is a log file based ONLY on content peeking
func (bs *BundleScanner) isLogFileByContent(filePath string) bool {
	if bs.timestampExtractor == nil {
		return false
	}

	// Use the timestamp detection logic to peek at file content
	result, err := bs.timestampExtractor.DetectBestPattern(filePath)
	if err != nil {
		// If we can't read the file, assume it's not a log file
		return false
	}

	// Require a minimum threshold for log file detection
	// For a file to be considered a log file, we need:
	// 1. At least 2 timestamp matches in the first 10 lines, AND
	// 2. At least 30% of lines must have timestamps (confidence >= 0.3)
	const minTimestampMatches = 2
	const minConfidence = 0.3

	return result.MatchCount >= minTimestampMatches && result.Confidence >= minConfidence
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
			// For quick scan, we check file content to determine if it's a log file
			entryFullPath := filepath.Join(path, entry.Name())
			if bs.isLogFileByContent(entryFullPath) {
				metadata.LogFileCount++
			}
		}
	}

	return metadata, nil
}

// GetAllFiles returns the complete file set collected during Phase 1
func (bs *BundleScanner) GetAllFiles() []string {
	return bs.allFiles
}

// GetWorkingSet returns the filtered log files from Phase 2
func (bs *BundleScanner) GetWorkingSet() []string {
	return bs.workingSet
}
