package models

import (
	"regexp"
	"time"
)

// RegexFilter represents a single regex filter with take/exclude logic
type RegexFilter struct {
	Pattern  string         `json:"pattern"` // The regex pattern text
	Take     bool           `json:"take"`    // true = include, false = exclude
	Compiled *regexp.Regexp `json:"-"`       // Compiled regex (not serialized)
	Valid    bool           `json:"valid"`   // Whether the pattern is valid
	Error    string         `json:"error"`   // Error message if invalid
}

// WorkingSet represents the current working state with all user selections and filters
type WorkingSet struct {
	Bundle        *Bundle         `json:"bundle"`         // Source bundle
	SelectedFiles map[string]bool `json:"selected_files"` // File selection state
	RegexFilters  []RegexFilter   `json:"regex_filters"`  // Ordered list of regex filters
	TimeFilter    *TimeRange      `json:"time_filter"`    // Time range filter
	EstimatedSize int64           `json:"estimated_size"` // Estimated output size
	VolumeData    []VolumePoint   `json:"volume_data"`    // Histogram data
	LastUpdated   time.Time       `json:"last_updated"`   // When last modified
}

// VolumePoint represents a time-binned data point for the histogram
type VolumePoint struct {
	BinStart  time.Time `json:"bin_start"`  // Bin start time
	BinEnd    time.Time `json:"bin_end"`    // Bin end time
	Count     int64     `json:"count"`      // Log entries in bin
	Size      int64     `json:"size"`       // Bytes in bin
	FileCount int       `json:"file_count"` // Files contributing to bin
}

// NewWorkingSet creates a new WorkingSet from a bundle
func NewWorkingSet(bundle *Bundle) *WorkingSet {
	selectedFiles := make(map[string]bool)

	// Initially select all log files
	for _, file := range bundle.Files {
		selectedFiles[file.Path] = file.IsLogFile
	}

	return &WorkingSet{
		Bundle:        bundle,
		SelectedFiles: selectedFiles,
		RegexFilters:  make([]RegexFilter, 0),
		TimeFilter:    nil, // No time filter initially
		EstimatedSize: bundle.TotalSize,
		VolumeData:    make([]VolumePoint, 0),
		LastUpdated:   time.Now(),
	}
}

// AddRegexFilter adds a new regex filter at the end of the list
func (ws *WorkingSet) AddRegexFilter(pattern string, take bool) error {
	compiled, err := regexp.Compile(pattern)
	filter := RegexFilter{
		Pattern:  pattern,
		Take:     take,
		Compiled: compiled,
		Valid:    err == nil,
		Error:    "",
	}
	if err != nil {
		filter.Error = err.Error()
	}

	ws.RegexFilters = append(ws.RegexFilters, filter)
	ws.LastUpdated = time.Now()
	return err
}

// RemoveRegexFilter removes a regex filter by index
func (ws *WorkingSet) RemoveRegexFilter(index int) {
	if index >= 0 && index < len(ws.RegexFilters) {
		ws.RegexFilters = append(ws.RegexFilters[:index], ws.RegexFilters[index+1:]...)
		ws.LastUpdated = time.Now()
	}
}

// UpdateRegexFilter updates a regex filter at the given index
func (ws *WorkingSet) UpdateRegexFilter(index int, pattern string, take bool) error {
	if index < 0 || index >= len(ws.RegexFilters) {
		return nil
	}

	compiled, err := regexp.Compile(pattern)
	ws.RegexFilters[index] = RegexFilter{
		Pattern:  pattern,
		Take:     take,
		Compiled: compiled,
		Valid:    err == nil,
		Error:    "",
	}
	if err != nil {
		ws.RegexFilters[index].Error = err.Error()
	}

	ws.LastUpdated = time.Now()
	return err
}

// SetRegexFilters replaces the entire regex filter list
func (ws *WorkingSet) SetRegexFilters(filters []RegexFilter) {
	ws.RegexFilters = make([]RegexFilter, len(filters))
	copy(ws.RegexFilters, filters)
	ws.LastUpdated = time.Now()
}

// SetTimeFilter sets the time range filter
func (ws *WorkingSet) SetTimeFilter(timeRange *TimeRange) {
	ws.TimeFilter = timeRange
	ws.LastUpdated = time.Now()
}

// ClearTimeFilter removes the time range filter
func (ws *WorkingSet) ClearTimeFilter() {
	ws.TimeFilter = nil
	ws.LastUpdated = time.Now()
}

// ToggleFileSelection toggles the selection state of a file
func (ws *WorkingSet) ToggleFileSelection(path string) {
	if ws.SelectedFiles == nil {
		ws.SelectedFiles = make(map[string]bool)
	}
	ws.SelectedFiles[path] = !ws.SelectedFiles[path]
	ws.LastUpdated = time.Now()
}

// SetFileSelection sets the selection state of a file
func (ws *WorkingSet) SetFileSelection(path string, selected bool) {
	if ws.SelectedFiles == nil {
		ws.SelectedFiles = make(map[string]bool)
	}
	ws.SelectedFiles[path] = selected
	ws.LastUpdated = time.Now()
}

// IsFileSelected returns true if the file is selected
func (ws *WorkingSet) IsFileSelected(path string) bool {
	if ws.SelectedFiles == nil {
		return false
	}
	return ws.SelectedFiles[path]
}

// GetSelectedFileCount returns the number of selected files
func (ws *WorkingSet) GetSelectedFileCount() int {
	count := 0
	for _, selected := range ws.SelectedFiles {
		if selected {
			count++
		}
	}
	return count
}

// SelectAllFiles selects all files
func (ws *WorkingSet) SelectAllFiles() {
	if ws.Bundle != nil {
		for _, file := range ws.Bundle.Files {
			ws.SetFileSelection(file.Path, true)
		}
	}
}

// SelectLogFiles selects only log files
func (ws *WorkingSet) SelectLogFiles() {
	if ws.Bundle != nil {
		for _, file := range ws.Bundle.Files {
			ws.SetFileSelection(file.Path, file.IsLogFile)
		}
	}
}

// SelectNone deselects all files
func (ws *WorkingSet) SelectNone() {
	for path := range ws.SelectedFiles {
		ws.SelectedFiles[path] = false
	}
	ws.LastUpdated = time.Now()
}

// UpdateVolumeData updates the histogram data
func (ws *WorkingSet) UpdateVolumeData(data []VolumePoint) {
	ws.VolumeData = data
	ws.LastUpdated = time.Now()
}

// HasTimeFilter returns true if a time filter is active
func (ws *WorkingSet) HasTimeFilter() bool {
	return ws.TimeFilter != nil && !ws.TimeFilter.IsZero()
}

// GetEffectiveTimeRange returns the effective time range considering filters
func (ws *WorkingSet) GetEffectiveTimeRange() *TimeRange {
	if ws.HasTimeFilter() {
		return ws.TimeFilter
	}
	if ws.Bundle != nil && ws.Bundle.TimeRange != nil {
		return ws.Bundle.TimeRange
	}
	return nil
}

// GetSelectedFilesBySize returns selected files sorted by size (descending)
func (ws *WorkingSet) GetSelectedFilesBySize(limit int) []FileInfo {
	if ws.Bundle == nil {
		return []FileInfo{}
	}

	var selectedFiles []FileInfo
	for _, file := range ws.Bundle.Files {
		if ws.IsFileSelected(file.Path) {
			selectedFiles = append(selectedFiles, file)
		}
	}

	// Sort by size descending (largest first)
	for i := 0; i < len(selectedFiles)-1; i++ {
		for j := i + 1; j < len(selectedFiles); j++ {
			if selectedFiles[i].Size < selectedFiles[j].Size {
				selectedFiles[i], selectedFiles[j] = selectedFiles[j], selectedFiles[i]
			}
		}
	}

	// Apply limit
	if limit > 0 && limit < len(selectedFiles) {
		selectedFiles = selectedFiles[:limit]
	}

	return selectedFiles
}

// GetSelectedTotalSize returns the total size of all selected files
func (ws *WorkingSet) GetSelectedTotalSize() int64 {
	if ws.Bundle == nil {
		return 0
	}

	var totalSize int64
	for _, file := range ws.Bundle.Files {
		if ws.IsFileSelected(file.Path) {
			totalSize += file.Size
		}
	}
	return totalSize
}
