package parser

import (
	"bufio"
	"fmt"
	"time"

	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/internal/utils"
	"github.com/spf13/afero"
)

// HistogramBuilder creates log volume histograms from working sets
type HistogramBuilder struct {
	boundsExtractor *BoundsExtractor
	fs              afero.Fs
}

// NewHistogramBuilder creates a new histogram builder
func NewHistogramBuilder(fs afero.Fs) *HistogramBuilder {
	return &HistogramBuilder{
		boundsExtractor: NewBoundsExtractor(fs),
		fs:              fs,
	}
}

// BuildHistogram creates a log volume histogram from a working set
func (hb *HistogramBuilder) BuildHistogram(workingSet *models.WorkingSet, binCount int) ([]models.VolumePoint, error) {
	if binCount <= 0 {
		binCount = 20 // Default
	}

	// Step 1: Extract time bounds from all selected files
	fileBounds, err := hb.boundsExtractor.ExtractBoundsFromWorkingSet(workingSet)
	if err != nil {
		return nil, fmt.Errorf("failed to extract file bounds: %w", err)
	}

	if len(fileBounds) == 0 {
		return nil, fmt.Errorf("no valid timestamps found in selected files")
	}

	// Step 2: Find global time range
	globalRange := hb.boundsExtractor.FindGlobalTimeBounds(fileBounds)
	if globalRange == nil {
		return nil, fmt.Errorf("failed to determine global time range")
	}

	// Step 3: Create time bins
	bins := hb.createTimeBins(globalRange, binCount)

	// Step 4: Populate histogram by binary searching each file for each bin
	volumePoints := make([]models.VolumePoint, len(bins))

	for i, bin := range bins {
		volumePoints[i] = models.VolumePoint{
			BinStart:  bin.Start,
			BinEnd:    bin.End,
			Count:     0,
			Size:      0,
			FileCount: 0,
		}

		// For each file, binary search to find bytes within this time bin
		for _, fileBound := range fileBounds {
			if !fileBound.Valid {
				continue
			}

			// Skip files that don't overlap with this bin
			if fileBound.Latest.Before(bin.Start) || fileBound.Earliest.After(bin.End) {
				continue
			}

			byteCount, err := hb.countBytesInTimeRange(fileBound.FilePath, fileBound.BestPattern, bin.Start, bin.End)
			if err != nil {
				// Log warning but continue
				utils.Warning("failed to count bytes in %s for bin %v-%v: %v",
					fileBound.FilePath, bin.Start.Format(time.RFC3339), bin.End.Format(time.RFC3339), err)
				continue
			}

			if byteCount > 0 {
				volumePoints[i].Size += byteCount
				volumePoints[i].FileCount++
			}
		}
	}

	return volumePoints, nil
}

// TimeBin represents a time range for histogram binning
type TimeBin struct {
	Start time.Time
	End   time.Time
}

// createTimeBins divides the global time range into the specified number of bins
func (hb *HistogramBuilder) createTimeBins(globalRange *models.TimeRange, binCount int) []TimeBin {
	duration := globalRange.Duration()
	binDuration := duration / time.Duration(binCount)

	bins := make([]TimeBin, binCount)
	currentTime := globalRange.Start

	for i := 0; i < binCount; i++ {
		bins[i].Start = currentTime
		if i == binCount-1 {
			// Last bin goes to the exact end
			bins[i].End = globalRange.End
		} else {
			bins[i].End = currentTime.Add(binDuration)
		}
		currentTime = bins[i].End
	}

	return bins
}

// countBytesInTimeRange counts bytes within a time range using mmap + binary search
func (hb *HistogramBuilder) countBytesInTimeRange(filePath string, bestPattern *TimestampPattern, startTime, endTime time.Time) (int64, error) {
	// Create memory-mapped file searcher for binary search
	searcher, err := NewMmapFileSearcher(hb.fs, filePath, hb.boundsExtractor.timestampExtractor, bestPattern)
	if err != nil {
		// Fallback to linear scan for files that can't be mmapped (e.g., non-OsFs)
		return hb.countBytesInTimeRangeLinear(filePath, bestPattern, startTime, endTime)
	}
	defer searcher.Close()

	// Use binary search to find byte boundaries for the time range
	byteCount := searcher.BinarySearchTimeRange(startTime, endTime)

	return byteCount, nil
}

// countBytesInTimeRangeLinear provides fallback linear scanning for non-mmap filesystems
func (hb *HistogramBuilder) countBytesInTimeRangeLinear(filePath string, bestPattern *TimestampPattern, startTime, endTime time.Time) (int64, error) {
	file, err := hb.fs.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var byteCount int64

	scanner := bufio.NewScanner(file)
	inRange := false

	for scanner.Scan() {
		line := scanner.Text()
		lineBytes := int64(len(line) + 1) // +1 for newline

		// Try to parse timestamp
		if timestamp, err := hb.boundsExtractor.timestampExtractor.ParseTimestamp(line, bestPattern); err == nil {
			// Line has a timestamp - check if it's in range
			if !timestamp.Before(startTime) && !timestamp.After(endTime) {
				inRange = true
				byteCount += lineBytes
			} else if timestamp.After(endTime) {
				// We've gone past the end time, stop scanning
				break
			} else if timestamp.Before(startTime) {
				inRange = false
			}
		} else if inRange {
			// Line without timestamp but we're in the time range
			// Include it as part of the log entry
			byteCount += lineBytes
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error scanning file: %w", err)
	}

	return byteCount, nil
}

// UpdateWorkingSetHistogram updates the VolumeData in a working set
func (hb *HistogramBuilder) UpdateWorkingSetHistogram(workingSet *models.WorkingSet, binCount int) error {
	volumePoints, err := hb.BuildHistogram(workingSet, binCount)
	if err != nil {
		return fmt.Errorf("failed to build histogram: %w", err)
	}

	workingSet.VolumeData = volumePoints
	workingSet.LastUpdated = time.Now()

	return nil
}
