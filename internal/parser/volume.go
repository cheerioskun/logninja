package parser

import (
	"fmt"

	"github.com/cheerioskun/logninja/internal/models"
	"github.com/spf13/afero"
)

// VolumeAnalyzer provides a high-level interface for log volume analysis
type VolumeAnalyzer struct {
	histogramBuilder *HistogramBuilder
	fs               afero.Fs
}

// NewVolumeAnalyzer creates a new volume analyzer
func NewVolumeAnalyzer(fs afero.Fs) *VolumeAnalyzer {
	return &VolumeAnalyzer{
		histogramBuilder: NewHistogramBuilder(fs),
		fs:               fs,
	}
}

// GenerateHistogram is the main entry point for creating log volume histograms
// This implements the complete flow:
// 1. Extract time bounds from all selected files (linear search from top/bottom)
// 2. Create time bins based on global range
// 3. For each bin, use efficient scanning to count lines within time boundaries
// 4. Return volume data for histogram display
func (va *VolumeAnalyzer) GenerateHistogram(workingSet *models.WorkingSet, binCount int) ([]models.VolumePoint, error) {
	if workingSet == nil {
		return nil, fmt.Errorf("working set cannot be nil")
	}

	if binCount <= 0 {
		binCount = 20 // Default bin count
	}

	return va.histogramBuilder.BuildHistogram(workingSet, binCount)
}

// UpdateWorkingSetVolumeData updates the VolumeData field in a working set
// This is convenient for keeping the working set in sync with histogram data
func (va *VolumeAnalyzer) UpdateWorkingSetVolumeData(workingSet *models.WorkingSet, binCount int) error {
	return va.histogramBuilder.UpdateWorkingSetHistogram(workingSet, binCount)
}

// GetTimeBounds extracts just the time bounds without building the full histogram
// Useful for getting the overall time range before deciding on bin count
func (va *VolumeAnalyzer) GetTimeBounds(workingSet *models.WorkingSet) (*models.TimeRange, error) {
	if workingSet == nil {
		return nil, fmt.Errorf("working set cannot be nil")
	}

	fileBounds, err := va.histogramBuilder.boundsExtractor.ExtractBoundsFromWorkingSet(workingSet)
	if err != nil {
		return nil, fmt.Errorf("failed to extract file bounds: %w", err)
	}

	globalRange := va.histogramBuilder.boundsExtractor.FindGlobalTimeBounds(fileBounds)
	if globalRange == nil {
		return nil, fmt.Errorf("no valid time range found in selected files")
	}

	return globalRange, nil
}

// AnalyzeVolumeDistribution provides summary statistics about the volume distribution
type VolumeDistribution struct {
	TotalLines    int64   `json:"total_lines"`
	TotalSize     int64   `json:"total_size"`
	TotalFiles    int     `json:"total_files"`
	AveragePerBin float64 `json:"average_per_bin"`
	PeakBinLines  int64   `json:"peak_bin_lines"`
	EmptyBins     int     `json:"empty_bins"`
	TimeSpan      string  `json:"time_span"`
}

// AnalyzeDistribution provides statistical analysis of volume data
func (va *VolumeAnalyzer) AnalyzeDistribution(volumePoints []models.VolumePoint) *VolumeDistribution {
	if len(volumePoints) == 0 {
		return &VolumeDistribution{}
	}

	analysis := &VolumeDistribution{}

	for _, point := range volumePoints {
		analysis.TotalLines += point.Count
		analysis.TotalSize += point.Size

		if point.Count > analysis.PeakBinLines {
			analysis.PeakBinLines = point.Count
		}

		if point.Count == 0 {
			analysis.EmptyBins++
		}

		// We can't easily track unique files from VolumePoint, so use FileCount as approximation
		if point.FileCount > analysis.TotalFiles {
			analysis.TotalFiles = point.FileCount
		}
	}

	if len(volumePoints) > 0 {
		analysis.AveragePerBin = float64(analysis.TotalLines) / float64(len(volumePoints))

		// Calculate time span
		start := volumePoints[0].BinStart
		end := volumePoints[len(volumePoints)-1].BinEnd
		analysis.TimeSpan = fmt.Sprintf("%s to %s",
			start.Format("2006-01-02 15:04:05"),
			end.Format("2006-01-02 15:04:05"))
	}

	return analysis
}
