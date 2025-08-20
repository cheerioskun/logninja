package models

import (
	"path/filepath"
	"time"

	"github.com/spf13/afero"
)

// Bundle represents a log bundle directory with all its files and metadata
type Bundle struct {
	Path      string         `json:"path"`       // Root directory path
	Files     []FileInfo     `json:"files"`      // All files in bundle
	TotalSize int64          `json:"total_size"` // Total size in bytes
	TimeRange *TimeRange     `json:"time_range"` // Overall time span
	Metadata  BundleMetadata `json:"metadata"`   // Additional metadata
	ScanTime  time.Time      `json:"scan_time"`  // When bundle was scanned
	fs        afero.Fs       // Filesystem interface for operations
}

// FileInfo contains information about a single file in the bundle
type FileInfo struct {
	Path         string     `json:"path"`          // Relative path from bundle root
	Size         int64      `json:"size"`          // File size in bytes
	IsLogFile    bool       `json:"is_log_file"`   // Detected as log file
	TimeRange    *TimeRange `json:"time_range"`    // Time span (nil if not parsed)
	Selected     bool       `json:"selected"`      // User selection state
	LastModified time.Time  `json:"last_modified"` // File modification time
}

// BundleMetadata contains aggregate information about the bundle
type BundleMetadata struct {
	LogFileCount   int       `json:"log_file_count"`   // Number of log files
	TotalFileCount int       `json:"total_file_count"` // Total number of files
	OldestLog      time.Time `json:"oldest_log"`       // Earliest log timestamp
	NewestLog      time.Time `json:"newest_log"`       // Latest log timestamp
	CommonFormats  []string  `json:"common_formats"`   // Detected log formats
	ScanDepth      int       `json:"scan_depth"`       // Directory depth scanned
}

// NewBundle creates a new Bundle with the given filesystem
func NewBundle(path string, fs afero.Fs) *Bundle {
	return &Bundle{
		Path:      path,
		Files:     make([]FileInfo, 0),
		TotalSize: 0,
		TimeRange: nil,
		Metadata: BundleMetadata{
			LogFileCount:   0,
			TotalFileCount: 0,
			CommonFormats:  make([]string, 0),
			ScanDepth:      0,
		},
		ScanTime: time.Now(),
		fs:       fs,
	}
}

// AddFile adds a file to the bundle and updates metadata
func (b *Bundle) AddFile(file FileInfo) {
	b.Files = append(b.Files, file)
	b.TotalSize += file.Size
	b.Metadata.TotalFileCount++

	if file.IsLogFile {
		b.Metadata.LogFileCount++

		// Update overall time range
		if file.TimeRange != nil {
			if b.TimeRange == nil {
				b.TimeRange = &TimeRange{
					Start: file.TimeRange.Start,
					End:   file.TimeRange.End,
				}
			} else {
				if file.TimeRange.Start.Before(b.TimeRange.Start) {
					b.TimeRange.Start = file.TimeRange.Start
				}
				if file.TimeRange.End.After(b.TimeRange.End) {
					b.TimeRange.End = file.TimeRange.End
				}
			}

			// Update oldest/newest log times
			if b.Metadata.OldestLog.IsZero() || file.TimeRange.Start.Before(b.Metadata.OldestLog) {
				b.Metadata.OldestLog = file.TimeRange.Start
			}
			if b.Metadata.NewestLog.IsZero() || file.TimeRange.End.After(b.Metadata.NewestLog) {
				b.Metadata.NewestLog = file.TimeRange.End
			}
		}
	}
}

// GetSelectedFiles returns a slice of selected file paths
func (b *Bundle) GetSelectedFiles() []string {
	var selected []string
	for _, file := range b.Files {
		if file.Selected {
			selected = append(selected, file.Path)
		}
	}
	return selected
}

// GetLogFiles returns a slice of all log files
func (b *Bundle) GetLogFiles() []FileInfo {
	var logFiles []FileInfo
	for _, file := range b.Files {
		if file.IsLogFile {
			logFiles = append(logFiles, file)
		}
	}
	return logFiles
}

// SelectAll selects all files in the bundle
func (b *Bundle) SelectAll() {
	for i := range b.Files {
		b.Files[i].Selected = true
	}
}

// SelectNone deselects all files in the bundle
func (b *Bundle) SelectNone() {
	for i := range b.Files {
		b.Files[i].Selected = false
	}
}

// SelectLogFiles selects only log files
func (b *Bundle) SelectLogFiles() {
	for i := range b.Files {
		b.Files[i].Selected = b.Files[i].IsLogFile
	}
}

// GetFileByPath returns a file by its path, or nil if not found
func (b *Bundle) GetFileByPath(path string) *FileInfo {
	for i := range b.Files {
		if b.Files[i].Path == path {
			return &b.Files[i]
		}
	}
	return nil
}

// ToggleFileSelection toggles the selection state of a file
func (b *Bundle) ToggleFileSelection(path string) bool {
	for i := range b.Files {
		if b.Files[i].Path == path {
			b.Files[i].Selected = !b.Files[i].Selected
			return b.Files[i].Selected
		}
	}
	return false
}

// GetFilesystem returns the filesystem interface
func (b *Bundle) GetFilesystem() afero.Fs {
	return b.fs
}

// GetAbsolutePath returns the absolute path for a relative file path
func (b *Bundle) GetAbsolutePath(relativePath string) string {
	return filepath.Join(b.Path, relativePath)
}
