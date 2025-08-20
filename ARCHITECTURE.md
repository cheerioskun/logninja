# LogNinja Architecture & Low-Level Design

## Overview

LogNinja is a Terminal User Interface (TUI) application for refining log bundles across two primary dimensions: file selection and time range filtering. Built with Go and the Charm TUI framework, it provides an intuitive interface for log analysis and bundle optimization.

## System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        LogNinja                             │
├─────────────────────────────────────────────────────────────┤
│  CLI Layer (cmd/)                                           │
│  ├─ main.go (entry point)                                   │
│  └─ cobra commands                                          │
├─────────────────────────────────────────────────────────────┤
│  UI Layer (ui/)                                             │
│  ├─ Application Model (bubbletea)                           │
│  ├─ FileTree Component                                      │
│  ├─ Regex Panel Component                                   │
│  ├─ Volume Histogram Component                              │
│  ├─ Time Range Modulator                                    │
│  └─ Status Panel                                            │
├─────────────────────────────────────────────────────────────┤
│  Business Logic (internal/)                                 │
│  ├─ Core Models (Bundle, WorkingSet, FileSet)              │
│  ├─ Log Parser (timestamp extraction)                       │
│  ├─ Filter Engine (regex + temporal)                        │
│  └─ Export Engine (bundle generation)                       │
├─────────────────────────────────────────────────────────────┤
│  Public APIs (pkg/)                                         │
│  └─ Utilities and shared types                              │
└─────────────────────────────────────────────────────────────┘
```

### Project Structure

```
logninja/
├── cmd/
│   └── logninja/
│       └── main.go                  # CLI entry point
├── internal/
│   ├── models/                      # Core data structures
│   │   ├── bundle.go               # Bundle definition
│   │   ├── fileset.go              # FileSet operations
│   │   ├── workingset.go           # Working state management
│   │   └── timerange.go            # Time range utilities
│   ├── parser/                      # Simple log parsing
│   │   ├── detector.go             # File type detection
│   │   ├── timestamp.go            # Regex-based timestamp extraction
│   │   └── patterns.go             # Timestamp regex patterns
│   ├── filter/                      # Filtering engine
│   │   ├── regex.go                # Regex filtering
│   │   ├── temporal.go             # Time-based filtering
│   │   └── engine.go               # Main filtering orchestration
│   └── export/                      # Bundle export logic
│       └── bundle.go               # Bundle creation and export
├── ui/                              # Public TUI components
│   ├── app.go                      # Main application model
│   ├── filetree/                   # File tree component
│   │   ├── model.go
│   │   └── view.go
│   ├── regex/                      # Regex panel component
│   │   ├── model.go
│   │   └── view.go
│   ├── histogram/                  # Volume histogram component
│   │   ├── model.go
│   │   └── view.go
│   ├── timerange/                  # Time range modulator
│   │   ├── model.go
│   │   └── view.go
│   └── status/                     # Status panel
│       ├── model.go
│       └── view.go
├── pkg/                            # Public APIs
│   └── utils/                      # Shared utilities
├── go.mod
├── go.sum
├── ARCHITECTURE.md                 # This document
├── .cursorrules                    # Development guidelines
└── README.md
```

## Core Data Models

### Bundle
Represents a log bundle directory with all its files and metadata.

```go
type Bundle struct {
    Path        string              // Root directory path
    Files       []FileInfo          // All files in bundle
    TotalSize   int64              // Total size in bytes
    TimeRange   TimeRange          // Overall time span
    Metadata    BundleMetadata     // Additional metadata
    ScanTime    time.Time          // When bundle was scanned
    fs          afero.Fs           // Filesystem interface for operations
}

type FileInfo struct {
    Path         string             // Relative path from bundle root
    Size         int64             // File size in bytes
    IsLogFile    bool              // Detected as log file
    TimeRange    *TimeRange        // Time span (nil if not parsed)
    LineCount    int64             // Approximate line count
    Selected     bool              // User selection state
    LastModified time.Time         // File modification time
}

type BundleMetadata struct {
    LogFileCount    int
    TotalFileCount  int
    OldestLog       time.Time
    NewestLog       time.Time
    CommonFormats   []string
}
```

### WorkingSet
Represents the current working state with all user selections and filters.

```go
type WorkingSet struct {
    Bundle          *Bundle           // Source bundle
    SelectedFiles   map[string]bool   // File selection state
    IncludeRegex    []string         // Include patterns
    ExcludeRegex    []string         // Exclude patterns
    TimeFilter      *TimeRange       // Time range filter
    EstimatedSize   int64            // Estimated output size
    VolumeData      []VolumePoint    // Histogram data
    LastUpdated     time.Time        // When last modified
}

type VolumePoint struct {
    BinStart    time.Time           // Bin start time
    BinEnd      time.Time           // Bin end time
    Count       int64               // Log entries in bin
    Size        int64               // Bytes in bin
    FileCount   int                 // Files contributing to bin
}
```

### FileSet
Represents the final result of applying all filters.

```go
type FileSet struct {
    Files       []string            // Selected file paths
    TotalSize   int64              // Total size after filtering
    TimeRange   TimeRange          // Effective time range
    Criteria    FilterCriteria     // Applied filter criteria
    LineCount   int64              // Estimated total lines
}

type FilterCriteria struct {
    IncludeRegex []string           // Include patterns
    ExcludeRegex []string           // Exclude patterns
    TimeRange    *TimeRange         // Time filter
    FilePattern  string             // File name pattern
}

type TimeRange struct {
    Start time.Time
    End   time.Time
}
```

## Component Specifications

### UI Components Architecture

All UI components follow the Bubble Tea pattern with a unified interface:

```go
type Component interface {
    Update(tea.Msg) (Component, tea.Cmd)
    View() string
    Focus()
    Blur()
    IsFocused() bool
    SetSize(width, height int)
    SetModel(interface{}) error       // For data binding
}
```

### 1. FileTree Component (`ui/filetree/`)

**Purpose**: Interactive file browser with selection capabilities.

**Features**:
- Hierarchical file tree display
- Multi-select with visual indicators
- File metadata display (size, type, time range)
- Expand/collapse directories
- Real-time filter preview

**Key Bindings**:
- `j/k` or `↑/↓`: Navigate
- `Space`: Toggle file selection
- `Enter`: Toggle directory expansion
- `a`: Select all visible
- `n`: Select none
- `i`: Show file info modal

**State Management**:
```go
type Model struct {
    bundle      *Bundle
    tree        []TreeNode
    cursor      int
    viewport    viewport.Model
    selected    map[string]bool
    expanded    map[string]bool
    focused     bool
}

type TreeNode struct {
    Path        string
    Info        *FileInfo
    Level       int
    IsDir       bool
    IsExpanded  bool
    Children    []*TreeNode
}
```

### 2. Regex Panel Component (`ui/regex/`)

**Purpose**: Manage include/exclude regex patterns with live validation.

**Layout**: Horizontally split into include (top) and exclude (bottom) sections.

**Features**:
- Pattern list with add/edit/delete
- Live regex validation
- Pattern testing against file names
- Common pattern templates
- Match count feedback

**Key Bindings**:
- `a`: Add new pattern
- `e`: Edit selected pattern
- `d`: Delete selected pattern
- `t`: Test pattern against current files
- `Tab`: Switch between include/exclude
- `Enter`: Confirm pattern edit

**State Management**:
```go
type Model struct {
    includePatterns []Pattern
    excludePatterns []Pattern
    activeSection   Section     // Include or Exclude
    cursor          int
    editMode        bool
    editInput       textinput.Model
    focused         bool
}

type Pattern struct {
    Text        string
    Compiled    *regexp.Regexp
    Valid       bool
    MatchCount  int
    Error       string
}
```

### 3. Volume Histogram Component (`ui/histogram/`)

**Purpose**: Visualize log volume over time with interactive time range selection.

**Features**:
- ASCII bar chart representation
- Configurable time binning (hour, day, week)
- Zoom and pan capabilities
- Time range selection via cursor
- Volume metrics display

**Key Bindings**:
- `+/-`: Zoom in/out
- `h/l` or `←/→`: Pan left/right
- `[/]`: Adjust bin size
- `Enter`: Select time range at cursor
- `r`: Reset zoom
- `Home/End`: Go to start/end

**State Management**:
```go
type Model struct {
    data        []VolumePoint
    binSize     time.Duration
    viewStart   time.Time
    viewEnd     time.Time
    cursor      int
    maxHeight   int
    width       int
    selection   *TimeRange
    focused     bool
}
```

### 4. Time Range Modulator (`ui/timerange/`)

**Purpose**: Precise time range selection with presets and manual input.

**Features**:
- Start/end time inputs
- Common presets (last hour, day, week, month)
- Relative time expressions ("2h ago", "yesterday")
- Time zone handling
- Range validation

**Key Bindings**:
- `s`: Focus start time
- `e`: Focus end time
- `p`: Show presets
- `c`: Clear range (show all)
- `Enter`: Apply time range
- `Esc`: Cancel edit

**State Management**:
```go
type Model struct {
    startInput      textinput.Model
    endInput        textinput.Model
    activeInput     InputField
    presets         []TimePreset
    currentRange    *TimeRange
    bundleRange     TimeRange
    focused         bool
}

type TimePreset struct {
    Name        string
    Description string
    Generator   func() TimeRange
}
```

### 5. Status Panel Component (`ui/status/`)

**Purpose**: Display current state and estimated output metrics.

**Features**:
- Real-time size estimation
- File count summary
- Processing status
- Error/warning indicators
- Export progress

**Display Elements**:
- Selected files: `X/Y files`
- Estimated size: `123.4 MB → 45.6 MB (63% reduction)`
- Time range: `2024-01-01 10:00 - 2024-01-02 15:30`
- Status: `Ready` | `Processing...` | `Error: ...`

## Log Parsing Strategy

### Simple Timestamp Extraction

LogNinja uses a **regex-based approach** for timestamp extraction - no complex parsing libraries needed.

```go
type TimestampExtractor struct {
    patterns []TimestampPattern
}

type TimestampPattern struct {
    Name        string              // Pattern name (e.g., "ISO8601")
    Regex       *regexp.Regexp      // Compiled regex
    Layout      string              // Go time layout for parsing
    Priority    int                 // Matching priority
}

// User-provided patterns will be configured here
var DefaultPatterns = []TimestampPattern{
    // Will be populated with user's regex patterns
}
```

**Parsing Flow**:
1. Read file line by line (first N lines only for efficiency)
2. Apply regex patterns in priority order
3. Extract first few timestamps to determine file time range
4. Cache results for subsequent operations

**Limitations** (by design):
- Only basic timestamp extraction
- No log level parsing
- No structured log parsing
- Performance over accuracy for large files

## Filter Engine Architecture

### Filtering Pipeline

```go
type FilterEngine struct {
    regexFilters    []RegexFilter
    temporalFilter  *TemporalFilter
    sizeCalculator  SizeCalculator
}

func (fe *FilterEngine) Apply(ws *WorkingSet) (*FileSet, error) {
    // 1. Apply file selection
    // 2. Apply regex filters
    // 3. Apply temporal filters
    // 4. Calculate final size
    // 5. Return FileSet
}
```

### Filter Types

**Regex Filters**:
- File name inclusion/exclusion
- Content-based filtering (future enhancement)
- Pattern compilation and caching

**Temporal Filters**:
- Time range intersection
- Bin-based volume calculation
- Efficient timestamp-based filtering

**Size Estimation**:
- Intelligent sampling for large files
- Linear extrapolation based on time ranges
- Caching for repeated calculations

## Data Flow

### Initialization Flow
```
User Input (bundle path) 
    → Bundle Discovery 
    → File Scanning 
    → Timestamp Extraction 
    → Initial WorkingSet 
    → UI Initialization
```

### User Interaction Flow
```
User Action (file select/regex/time range)
    → WorkingSet Update
    → Filter Engine Apply
    → Volume Histogram Update
    → Size Estimation Update
    → UI Refresh
```

### Export Flow
```
Export Command
    → Final Filter Application
    → File Copying/Linking
    → Bundle Generation
    → Success Confirmation
```

## Performance Considerations

### Optimization Strategies

1. **Lazy Loading**: Load file metadata on-demand
2. **Caching**: Cache timestamp extraction results
3. **Streaming**: Process large files in chunks
4. **Parallelization**: Concurrent file processing
5. **Sampling**: Estimate vs. exact calculations for large datasets

### Memory Management

- Use file streaming for large log files
- Implement LRU cache for parsed metadata
- Limit in-memory volume data points
- Background garbage collection for unused data
- Leverage afero for efficient filesystem operations and testing

### Scalability Targets

- **Bundle Size**: Up to 10GB efficiently
- **File Count**: Thousands of files
- **Time Range**: Years of historical data
- **UI Responsiveness**: < 100ms for user interactions

## Implementation Phases

### Phase 1: Foundation (Week 1)
- [ ] Project setup and basic CLI structure
- [ ] Core data models implementation
- [ ] Bundle discovery and file enumeration
- [ ] Basic TUI application shell

### Phase 2: Core Parsing (Week 2)
- [ ] Timestamp regex patterns integration
- [ ] Simple log file detection
- [ ] Time range extraction per file
- [ ] Basic filtering logic implementation

### Phase 3: Basic UI (Week 3)
- [ ] FileTree component implementation
- [ ] Navigation and selection logic
- [ ] Basic application layout
- [ ] Component integration

### Phase 4: Advanced Features (Week 4)
- [ ] Regex panel with include/exclude
- [ ] Volume histogram visualization
- [ ] Time range modulator
- [ ] Status panel and size estimation

### Phase 5: Polish & Export (Week 5)
- [ ] Bundle export functionality
- [ ] Performance optimization
- [ ] Error handling and validation
- [ ] Documentation and testing

## Technical Dependencies

### Core Dependencies
```go
require (
    github.com/charmbracelet/bubbletea v0.24.2
    github.com/charmbracelet/bubbles v0.16.1
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/spf13/cobra v1.7.0
    github.com/spf13/viper v1.16.0
    github.com/spf13/afero v1.10.0
)
```

### Development Dependencies
```go
require (
    github.com/stretchr/testify v1.8.4
    github.com/golang/mock v1.6.0
)
```

## Filesystem Abstraction

### Afero Integration

LogNinja uses [Afero](https://github.com/spf13/afero) for filesystem abstraction, providing several key benefits:

**Benefits**:
- **Testability**: Mock filesystem operations in unit tests
- **Flexibility**: Support different filesystem backends (OS, memory, readonly)
- **Performance**: Memory-based filesystem for testing and development
- **Security**: Restricted filesystem access when needed

**Architecture Pattern**:
```go
type BundleScanner struct {
    fs afero.Fs  // Injected filesystem interface
}

// Production usage
scanner := &BundleScanner{
    fs: afero.NewOsFs(), // Real filesystem
}

// Testing usage
scanner := &BundleScanner{
    fs: afero.NewMemMapFs(), // In-memory filesystem
}
```

**Core Filesystem Operations**:
```go
// Bundle scanning and file discovery
func (b *BundleScanner) ScanBundle(path string) (*Bundle, error) {
    return b.scanDirectory(path, b.fs)
}

// Log file reading with streaming
func (p *TimestampExtractor) ExtractFromFile(path string, fs afero.Fs) (*TimeRange, error) {
    file, err := fs.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()
    
    return p.extractTimestamps(file)
}

// Export operations
func (e *BundleExporter) Export(fileSet *FileSet, outputPath string, fs afero.Fs) error {
    return e.copyFiles(fileSet, outputPath, fs)
}
```

**Testing Strategy**:
- Use `afero.NewMemMapFs()` for unit tests
- Pre-populate test filesystem with sample log files
- Mock complex filesystem scenarios (permissions, large files)
- Test export operations without touching real filesystem

## Configuration

### User Configuration (`~/.logninja/config.yaml`)
```yaml
timestamp_patterns:
  - name: "ISO8601"
    regex: '\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}'
    layout: "2006-01-02T15:04:05"
  # Additional user patterns...

ui:
  theme: "default"
  histogram_bins: 50
  max_file_preview: 1000

performance:
  max_scan_depth: 10
  timestamp_sample_lines: 100
  cache_size: "100MB"
```

## Error Handling Strategy

### Error Categories
1. **User Errors**: Invalid paths, malformed regex
2. **System Errors**: File permissions, disk space
3. **Parse Errors**: Unrecognized log formats
4. **Performance Errors**: Memory limits, timeouts

### Error Recovery
- Graceful degradation for parse failures
- Retry mechanisms for transient errors
- Clear user feedback for actionable errors
- Fallback modes for unsupported formats

---

*This document serves as the authoritative source for LogNinja's architecture and will be updated as the implementation progresses.*
