package parser

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/internal/utils"
	"github.com/spf13/afero"
)

const (
	// MaxLinesToCheckForTimestamp is the maximum number of lines we check when extracting bounds
	// This is higher than PeakLines since we really need to find valid timestamps for bounds
	MaxLinesToCheckForTimestamp = 100
)

// TimeBounds represents the earliest and latest timestamps found in a file
type TimeBounds struct {
	Earliest     time.Time
	Latest       time.Time
	FilePath     string
	Valid        bool              // false if no valid timestamps found
	BestPattern  *TimestampPattern // The pattern that worked best for this file
	EarliestLine string            // The actual line containing earliest timestamp
	LatestLine   string            // The actual line containing latest timestamp
}

// BoundsExtractor efficiently finds earliest and latest timestamps in log files
type BoundsExtractor struct {
	timestampExtractor *TimestampExtractor
	fs                 afero.Fs
}

// NewBoundsExtractor creates a new bounds extractor
func NewBoundsExtractor(fs afero.Fs) *BoundsExtractor {
	return &BoundsExtractor{
		timestampExtractor: NewTimestampExtractor(fs),
		fs:                 fs,
	}
}

// ExtractBounds finds the earliest and latest timestamps in a file using efficient linear search
func (be *BoundsExtractor) ExtractBounds(filePath string) (*TimeBounds, error) {
	bounds := &TimeBounds{
		FilePath: filePath,
		Valid:    false,
	}

	// First, detect the best timestamp pattern for this file
	patternResult, err := be.timestampExtractor.DetectBestPattern(filePath)
	if err != nil {
		return bounds, fmt.Errorf("failed to detect timestamp pattern for %s: %w", filePath, err)
	}

	if patternResult.BestPattern == nil {
		// No timestamp patterns found
		return bounds, nil
	}

	bounds.BestPattern = patternResult.BestPattern

	// Find earliest timestamp (linear search from top)
	earliest, earliestLine, err := be.findEarliestTimestamp(filePath, bounds.BestPattern)
	if err != nil {
		return bounds, fmt.Errorf("failed to find earliest timestamp in %s: %w", filePath, err)
	}

	// Find latest timestamp (linear search from bottom)
	latest, latestLine, err := be.findLatestTimestamp(filePath, bounds.BestPattern)
	if err != nil {
		return bounds, fmt.Errorf("failed to find latest timestamp in %s: %w", filePath, err)
	}

	bounds.Earliest = earliest
	bounds.Latest = latest
	bounds.EarliestLine = earliestLine
	bounds.LatestLine = latestLine
	bounds.Valid = true

	return bounds, nil
}

// findEarliestTimestamp performs linear search from top of file to find first valid timestamp
func (be *BoundsExtractor) findEarliestTimestamp(filePath string, bestPattern *TimestampPattern) (time.Time, string, error) {
	file, err := be.fs.Open(filePath)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return be.timestampExtractor.FindLineWithTimestamp(file, bestPattern, MaxLinesToCheckForTimestamp)
}

// findLatestTimestamp performs linear search from bottom of file to find last valid timestamp
func (be *BoundsExtractor) findLatestTimestamp(filePath string, bestPattern *TimestampPattern) (time.Time, string, error) {
	file, err := be.fs.Open(filePath)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// For reverse reading, we need to seek to end and read backwards
	// For large files, we'll read the last few KB and scan backwards
	stat, err := file.Stat()
	if err != nil {
		return time.Time{}, "", fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := stat.Size()

	// Read last 64KB or entire file if smaller
	readSize := int64(64 * 1024) // 64KB
	if fileSize < readSize {
		readSize = fileSize
	}

	// Seek to position for reading the tail
	seekPos := fileSize - readSize
	if seekPos < 0 {
		seekPos = 0
	}

	_, err = file.Seek(seekPos, io.SeekStart)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("failed to seek in file: %w", err)
	}

	// Read the tail into memory and scan backwards
	buffer := make([]byte, readSize)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return time.Time{}, "", fmt.Errorf("failed to read file tail: %w", err)
	}

	return be.findLatestTimestampInBuffer(buffer[:n], bestPattern)
}

// findLatestTimestampInBuffer scans a buffer backwards to find the latest timestamp
func (be *BoundsExtractor) findLatestTimestampInBuffer(buffer []byte, bestPattern *TimestampPattern) (time.Time, string, error) {
	// Split buffer into lines and scan from bottom up
	lines := splitLinesReverse(buffer)

	for _, line := range lines {
		if timestamp, err := be.timestampExtractor.ParseTimestamp(line, bestPattern); err == nil {
			return timestamp, line, nil
		}
	}

	return time.Time{}, "", fmt.Errorf("no valid timestamp found in file tail")
}

// splitLinesReverse splits a buffer into lines and returns them in reverse order
func splitLinesReverse(buffer []byte) []string {
	// Convert to string and split by newlines
	content := string(buffer)
	lines := make([]string, 0)

	// Split by newlines
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	// Reverse the slice
	for i := 0; i < len(lines)/2; i++ {
		j := len(lines) - 1 - i
		lines[i], lines[j] = lines[j], lines[i]
	}

	return lines
}

// ExtractBoundsFromWorkingSet extracts time bounds from all selected files in a working set
func (be *BoundsExtractor) ExtractBoundsFromWorkingSet(workingSet *models.WorkingSet) ([]TimeBounds, error) {
	if workingSet == nil || workingSet.Bundle == nil {
		return nil, fmt.Errorf("invalid working set")
	}

	var bounds []TimeBounds

	// Process only selected log files
	for _, file := range workingSet.Bundle.Files {
		if !workingSet.SelectedFiles[file.Path] || !file.IsLogFile {
			continue
		}

		// Convert relative path to absolute path
		absolutePath := workingSet.Bundle.GetAbsolutePath(file.Path)

		fileBounds, err := be.ExtractBounds(absolutePath)
		if err != nil {
			// Log warning but continue with other files
			utils.Warning("failed to extract bounds from %s: %v", absolutePath, err)
			continue
		}

		if fileBounds.Valid {
			bounds = append(bounds, *fileBounds)
		}
	}

	return bounds, nil
}

// FindGlobalTimeBounds finds the overall earliest and latest timestamps across all files
func (be *BoundsExtractor) FindGlobalTimeBounds(bounds []TimeBounds) *models.TimeRange {
	if len(bounds) == 0 {
		return nil
	}

	var earliest, latest time.Time
	initialized := false

	for _, bound := range bounds {
		if !bound.Valid {
			continue
		}

		if !initialized {
			earliest = bound.Earliest
			latest = bound.Latest
			initialized = true
		} else {
			if bound.Earliest.Before(earliest) {
				earliest = bound.Earliest
			}
			if bound.Latest.After(latest) {
				latest = bound.Latest
			}
		}
	}

	if !initialized {
		return nil
	}

	timeRange, _ := models.NewTimeRange(earliest, latest)
	return timeRange
}
