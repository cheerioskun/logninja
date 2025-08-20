package parser

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/afero"
)

// MmapFileSearcher provides binary search capabilities on memory-mapped files
// Uses direct byte position binary search without pre-indexing lines
type MmapFileSearcher struct {
	filePath           string
	timestampExtractor *TimestampExtractor
	bestPattern        *TimestampPattern
	fs                 afero.Fs

	// Memory mapping
	data   []byte
	file   *os.File
	mapped bool
}

// NewMmapFileSearcher creates a new memory-mapped file searcher
func NewMmapFileSearcher(fs afero.Fs, filePath string, timestampExtractor *TimestampExtractor, bestPattern *TimestampPattern) (*MmapFileSearcher, error) {
	searcher := &MmapFileSearcher{
		filePath:           filePath,
		timestampExtractor: timestampExtractor,
		bestPattern:        bestPattern,
		fs:                 fs,
		mapped:             false,
	}

	if err := searcher.mapFile(); err != nil {
		return nil, fmt.Errorf("failed to map file: %w", err)
	}

	return searcher, nil
}

// mapFile memory-maps the file for efficient random access
func (mfs *MmapFileSearcher) mapFile() error {
	// For afero compatibility, we need to get the real file path
	// This is a limitation - mmap works best with real OS files
	_, ok := mfs.fs.(*afero.OsFs)
	if !ok {
		return fmt.Errorf("mmap requires OsFs, got %T", mfs.fs)
	}

	file, err := os.Open(mfs.filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat file: %w", err)
	}

	size := stat.Size()
	if size == 0 {
		file.Close()
		return fmt.Errorf("file is empty")
	}

	// Memory map the file
	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to mmap file: %w", err)
	}

	mfs.file = file
	mfs.data = data
	mfs.mapped = true

	return nil
}

// findLineStart rolls back from a byte position to find the start of the current line
// Returns the byte offset of the line start
func (mfs *MmapFileSearcher) findLineStart(pos int64) int64 {
	if pos <= 0 {
		return 0
	}

	// Roll back to find the previous newline
	for i := pos - 1; i >= 0; i-- {
		if mfs.data[i] == '\n' {
			return i + 1 // Line starts after the newline
		}
	}

	return 0 // Beginning of file
}

// findLineEnd rolls forward from a byte position to find the end of the current line
// Returns the byte offset of the line end (excluding the newline)
func (mfs *MmapFileSearcher) findLineEnd(pos int64) int64 {
	maxPos := int64(len(mfs.data))

	for i := pos; i < maxPos; i++ {
		if mfs.data[i] == '\n' {
			return i
		}
	}

	return maxPos // End of file
}

// extractLineAt extracts the complete line containing the byte position
func (mfs *MmapFileSearcher) extractLineAt(pos int64) (string, int64, int64) {
	lineStart := mfs.findLineStart(pos)
	lineEnd := mfs.findLineEnd(lineStart)

	if lineStart >= lineEnd {
		return "", lineStart, lineEnd
	}

	line := string(mfs.data[lineStart:lineEnd])
	return line, lineStart, lineEnd
}

// parseTimestampAt extracts and parses timestamp from line at given position
func (mfs *MmapFileSearcher) parseTimestampAt(pos int64) (time.Time, error) {
	line, _, _ := mfs.extractLineAt(pos)
	if line == "" {
		return time.Time{}, fmt.Errorf("empty line")
	}

	return mfs.timestampExtractor.ParseTimestamp(line, mfs.bestPattern)
}

// BinarySearchTimeRange uses binary search on byte positions to find time range boundaries
// This is O(log n) without any O(n) preprocessing
func (mfs *MmapFileSearcher) BinarySearchTimeRange(startTime, endTime time.Time) (byteCount int64) {
	fileSize := int64(len(mfs.data))
	if fileSize == 0 {
		return 0
	}

	// Binary search for start position
	startPos := mfs.binarySearchTime(startTime, true)

	// Binary search for end position
	endPos := mfs.binarySearchTime(endTime, false)

	// Calculate byte count
	if endPos > startPos {
		byteCount = endPos - startPos
	}

	return byteCount
}

// binarySearchTime performs binary search to find byte position for a target time
// searchStart=true finds first position >= targetTime
// searchStart=false finds first position > targetTime
func (mfs *MmapFileSearcher) binarySearchTime(targetTime time.Time, searchStart bool) int64 {
	left := int64(0)
	right := int64(len(mfs.data))
	result := right

	for left < right {
		mid := left + (right-left)/2

		// Find the actual line containing mid position
		lineStart := mfs.findLineStart(mid)

		// Parse timestamp from this line
		timestamp, err := mfs.parseTimestampAt(lineStart)
		if err != nil {
			// No valid timestamp, try to move forward to find one
			nextValidPos := mfs.findNextValidTimestamp(lineStart)
			if nextValidPos == -1 {
				// No more valid timestamps, search in left half
				right = mid
				continue
			}
			lineStart = nextValidPos
			timestamp, err = mfs.parseTimestampAt(lineStart)
			if err != nil {
				right = mid
				continue
			}
		}

		// Compare timestamps
		var condition bool
		if searchStart {
			condition = !timestamp.Before(targetTime) // timestamp >= targetTime
		} else {
			condition = timestamp.After(targetTime) // timestamp > targetTime
		}

		if condition {
			result = lineStart
			right = mid
		} else {
			left = mfs.findLineEnd(lineStart) + 1 // Move to next line
		}
	}

	return result
}

// findNextValidTimestamp searches forward from a position to find the next line with a valid timestamp
func (mfs *MmapFileSearcher) findNextValidTimestamp(startPos int64) int64 {
	maxPos := int64(len(mfs.data))
	pos := startPos

	for pos < maxPos {
		lineStart := mfs.findLineStart(pos)
		lineEnd := mfs.findLineEnd(lineStart)

		if lineEnd > lineStart {
			line := string(mfs.data[lineStart:lineEnd])
			if _, err := mfs.timestampExtractor.ParseTimestamp(line, mfs.bestPattern); err == nil {
				return lineStart
			}
		}

		pos = lineEnd + 1 // Move to next line
	}

	return -1 // No valid timestamp found
}

// Close unmaps the file and releases resources
func (mfs *MmapFileSearcher) Close() error {
	var err error

	if mfs.mapped && mfs.data != nil {
		if unmapErr := syscall.Munmap(mfs.data); unmapErr != nil {
			err = fmt.Errorf("failed to munmap: %w", unmapErr)
		}
		mfs.data = nil
		mfs.mapped = false
	}

	if mfs.file != nil {
		if closeErr := mfs.file.Close(); closeErr != nil {
			if err != nil {
				err = fmt.Errorf("%w; also failed to close file: %v", err, closeErr)
			} else {
				err = fmt.Errorf("failed to close file: %w", closeErr)
			}
		}
		mfs.file = nil
	}

	return err
}

// GetFileStats returns statistics about the file
func (mfs *MmapFileSearcher) GetFileStats() (totalBytes int64) {
	return int64(len(mfs.data))
}
