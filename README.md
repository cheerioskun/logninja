# LogNinja

A Terminal User Interface (TUI) tool for refining log bundles with intuitive file selection and time range filtering.

## Features

- 📁 **Interactive File Tree** - Browse and select files with visual indicators
- 🔍 **Regex Filtering** - Include/exclude patterns for flexible file filtering  
- 📊 **Volume Histogram** - Visualize log volume over time
- ⏰ **Time Range Selection** - Filter logs by precise time ranges
- 💾 **Size Estimation** - Real-time bundle size calculation
- 🚀 **Fast Scanning** - Efficient directory traversal with configurable depth

## Installation

```bash
# Clone the repository
git clone https://github.com/cheerioskun/logninja.git
cd logninja

# Build the application
go build -o logninja ./cmd/logninja

# Install globally (optional)
go install ./cmd/logninja
```

## Quick Start

### Scan a Directory
```bash
# Quick scan to see basic information
logninja scan /var/log --quick

# Full scan with detailed file information
logninja scan /path/to/logs --verbose

# Limit scanning depth
logninja scan /deep/directory --max-depth 3
```

### Start the TUI Interface
```bash
# Launch interactive TUI
logninja tui /var/log

# With custom scan depth
logninja tui /path/to/logs --max-depth 5
```

## Usage

### TUI Navigation

- **Tab / Shift+Tab** - Navigate between panels
- **q / Ctrl+C** - Quit application
- **?** - Show help

### Panels

1. **📁 File Tree Panel**
   - Browse directory structure
   - Select/deselect files
   - View file metadata

2. **🔍 Regex Panel** 
   - Add include patterns
   - Add exclude patterns
   - Live pattern validation

3. **📊 Volume Histogram**
   - Time-binned log volume visualization
   - Interactive time range selection
   - Zoom and pan capabilities

4. **⏰ Time Range Panel**
   - Set start/end times
   - Use preset ranges
   - Clear time filters

5. **💾 Status Panel**
   - Selected file count
   - Estimated bundle size
   - Current operation status

## Configuration

Create a configuration file at `~/.logninja.yaml`:

```yaml
# Timestamp patterns for log parsing
timestamp_patterns:
  - name: "ISO8601"
    regex: '\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}'
    layout: "2006-01-02T15:04:05"
  - name: "Syslog"
    regex: '\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}'
    layout: "Jan 2 15:04:05"

# UI settings
ui:
  theme: "default"
  histogram_bins: 50

# Performance tuning
performance:
  max_scan_depth: 10
  cache_size: "100MB"
```

## Commands

### `logninja scan`
Scan a directory and display bundle information without starting the TUI.

```bash
logninja scan [path] [flags]

Flags:
  --quick              Perform quick scan (top-level only)
  --max-depth int      Maximum directory depth to scan (default 10)
  -v, --verbose        Verbose output
```

### `logninja tui`
Start the interactive TUI interface.

```bash
logninja tui [path] [flags]

Flags:
  --max-depth int      Maximum directory depth to scan (default 10)
  -v, --verbose        Verbose output
```

## Development Status

LogNinja is currently in **Phase 1** implementation:

✅ **Phase 1: Foundation** (Current)
- [x] Project setup and CLI structure
- [x] Core data models
- [x] Bundle discovery and file enumeration
- [x] Basic TUI application shell

🚧 **Phase 2: Core Parsing** (Next)
- [ ] Timestamp regex patterns
- [ ] Log file detection
- [ ] Time range extraction
- [ ] Basic filtering logic

📋 **Phase 3: Basic UI**
- [ ] FileTree component
- [ ] Navigation and selection
- [ ] Component integration

⭐ **Phase 4: Advanced Features**
- [ ] Regex panel implementation
- [ ] Volume histogram
- [ ] Time range modulator
- [ ] Real-time size estimation

🎯 **Phase 5: Polish & Export**
- [ ] Bundle export functionality
- [ ] Performance optimization
- [ ] Comprehensive testing

## Architecture

LogNinja follows clean architecture principles:

```
├── cmd/                 # Application entry points
├── internal/            # Private application code
│   ├── models/         # Core data structures
│   ├── scanner/        # Bundle discovery logic
│   └── cmd/            # CLI commands
├── ui/                 # Public TUI components
└── pkg/                # Public APIs and utilities
```

Key technologies:
- **Go 1.21+** - Primary language
- **Charm TUI** - Terminal interface framework
- **Cobra** - CLI framework
- **Afero** - Filesystem abstraction
- **Viper** - Configuration management

## Contributing

1. Fork the repository
2. Create a feature branch
3. Follow the coding standards in `.cursorrules`
4. Add tests for new functionality
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Roadmap

- [ ] Plugin system for custom log formats
- [ ] Export to multiple bundle formats
- [ ] Advanced filtering expressions
- [ ] Log content preview
- [ ] Performance profiling tools
- [ ] Remote log source support
