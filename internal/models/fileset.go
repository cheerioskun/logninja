package models

// FileSet represents the final result of applying all filters
type FileSet struct {
	Files     []string       `json:"files"`      // Selected file paths
	TotalSize int64          `json:"total_size"` // Total size after filtering
	TimeRange *TimeRange     `json:"time_range"` // Effective time range
	Criteria  FilterCriteria `json:"criteria"`   // Applied filter criteria
	LineCount int64          `json:"line_count"` // Estimated total lines
}

// FilterCriteria represents the criteria used to create a FileSet
type FilterCriteria struct {
	IncludeRegex []string   `json:"include_regex"`  // Include patterns
	ExcludeRegex []string   `json:"exclude_regex"`  // Exclude patterns
	TimeRange    *TimeRange `json:"time_range"`     // Time filter
	FilePattern  string     `json:"file_pattern"`   // File name pattern
	OnlyLogFiles bool       `json:"only_log_files"` // Filter to log files only
}

// NewFileSet creates a new FileSet
func NewFileSet() *FileSet {
	return &FileSet{
		Files:     make([]string, 0),
		TotalSize: 0,
		TimeRange: nil,
		Criteria: FilterCriteria{
			IncludeRegex: make([]string, 0),
			ExcludeRegex: make([]string, 0),
			TimeRange:    nil,
			FilePattern:  "",
			OnlyLogFiles: false,
		},
		LineCount: 0,
	}
}

// NewFileSetFromWorkingSet creates a FileSet from a WorkingSet
func NewFileSetFromWorkingSet(ws *WorkingSet) *FileSet {
	fs := NewFileSet()

	// Copy filter criteria
	fs.Criteria.IncludeRegex = make([]string, len(ws.IncludeRegex))
	copy(fs.Criteria.IncludeRegex, ws.IncludeRegex)

	fs.Criteria.ExcludeRegex = make([]string, len(ws.ExcludeRegex))
	copy(fs.Criteria.ExcludeRegex, ws.ExcludeRegex)

	fs.Criteria.TimeRange = ws.TimeFilter
	fs.TimeRange = ws.GetEffectiveTimeRange()

	// Add selected files
	if ws.Bundle != nil {
		for _, file := range ws.Bundle.Files {
			if ws.IsFileSelected(file.Path) {
				fs.AddFile(file.Path, file.Size, file.LineCount)
			}
		}
	}

	return fs
}

// AddFile adds a file to the FileSet
func (fs *FileSet) AddFile(path string, size int64, lineCount int64) {
	fs.Files = append(fs.Files, path)
	fs.TotalSize += size
	fs.LineCount += lineCount
}

// RemoveFile removes a file from the FileSet by path
func (fs *FileSet) RemoveFile(path string) bool {
	for i, file := range fs.Files {
		if file == path {
			fs.Files = append(fs.Files[:i], fs.Files[i+1:]...)
			return true
		}
	}
	return false
}

// Contains checks if a file path is in the FileSet
func (fs *FileSet) Contains(path string) bool {
	for _, file := range fs.Files {
		if file == path {
			return true
		}
	}
	return false
}

// IsEmpty returns true if the FileSet contains no files
func (fs *FileSet) IsEmpty() bool {
	return len(fs.Files) == 0
}

// GetFileCount returns the number of files in the FileSet
func (fs *FileSet) GetFileCount() int {
	return len(fs.Files)
}

// GetSizeReduction calculates the size reduction compared to original
func (fs *FileSet) GetSizeReduction(originalSize int64) float64 {
	if originalSize == 0 {
		return 0.0
	}
	reduction := float64(originalSize-fs.TotalSize) / float64(originalSize)
	if reduction < 0 {
		return 0.0
	}
	return reduction
}

// GetCompressionRatio calculates the compression ratio
func (fs *FileSet) GetCompressionRatio(originalSize int64) float64 {
	if originalSize == 0 {
		return 1.0
	}
	return float64(fs.TotalSize) / float64(originalSize)
}

// Clone creates a deep copy of the FileSet
func (fs *FileSet) Clone() *FileSet {
	clone := &FileSet{
		Files:     make([]string, len(fs.Files)),
		TotalSize: fs.TotalSize,
		TimeRange: fs.TimeRange,
		LineCount: fs.LineCount,
		Criteria: FilterCriteria{
			IncludeRegex: make([]string, len(fs.Criteria.IncludeRegex)),
			ExcludeRegex: make([]string, len(fs.Criteria.ExcludeRegex)),
			TimeRange:    fs.Criteria.TimeRange,
			FilePattern:  fs.Criteria.FilePattern,
			OnlyLogFiles: fs.Criteria.OnlyLogFiles,
		},
	}

	copy(clone.Files, fs.Files)
	copy(clone.Criteria.IncludeRegex, fs.Criteria.IncludeRegex)
	copy(clone.Criteria.ExcludeRegex, fs.Criteria.ExcludeRegex)

	return clone
}
