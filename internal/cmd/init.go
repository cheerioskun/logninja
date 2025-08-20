package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/internal/scanner"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	outputConfig   string
	smartSelection bool
	includeNonLogs bool
	minFileSize    int64
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize a working set configuration for a log directory",
	Long: `Initialize and configure a working set for log analysis.

This command:
- Scans the directory for log files (using both extension and timestamp detection)
- Applies intelligent selection rules
- Saves the configuration for later use

Examples:
  logninja init /var/log
  logninja init ./my-logs --smart-selection
  logninja init /logs --output-config my-workset.json
  logninja init /logs --include-non-logs --min-size 1024`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Init-specific flags
	initCmd.Flags().StringVarP(&outputConfig, "output-config", "o", ".logninja-workset.json", "output configuration file")
	initCmd.Flags().BoolVar(&smartSelection, "smart-selection", true, "enable intelligent file selection")
	initCmd.Flags().BoolVar(&includeNonLogs, "include-non-logs", false, "include non-log files in analysis")
	initCmd.Flags().Int64Var(&minFileSize, "min-size", 0, "minimum file size in bytes to include")
	initCmd.Flags().IntVar(&maxDepth, "max-depth", 10, "maximum directory depth to scan")

	// Bind flags to viper
	viper.BindPFlag("smart-selection", initCmd.Flags().Lookup("smart-selection"))
	viper.BindPFlag("include-non-logs", initCmd.Flags().Lookup("include-non-logs"))
}

func runInit(cmd *cobra.Command, args []string) error {
	bundlePath := args[0]

	// Convert to absolute path
	absPath, err := filepath.Abs(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Verify path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	fmt.Printf("Initializing working set for: %s\n", absPath)
	fmt.Printf("Configuration will be saved to: %s\n", outputConfig)
	fmt.Println()

	// Create filesystem interface
	fs := afero.NewOsFs()

	// Create bundle scanner (always uses timestamp detection)
	bundleScanner := scanner.NewBundleScanner(fs)
	bundleScanner.SetMaxDepth(maxDepth)

	// Scan the bundle
	fmt.Println("Scanning directory...")
	bundle, err := bundleScanner.ScanBundle(absPath)
	if err != nil {
		return fmt.Errorf("failed to scan bundle: %w", err)
	}

	fmt.Printf("Found %d files (%d log files)\n",
		bundle.Metadata.TotalFileCount, bundle.Metadata.LogFileCount)

	// Create working set with intelligent defaults
	workingSet := createIntelligentWorkingSet(bundle)

	// Apply intelligent selection rules
	if smartSelection {
		fmt.Println("Applying intelligent selection rules...")
		applySmartSelection(workingSet)
	}

	// Apply size filter
	if minFileSize > 0 {
		fmt.Printf("Filtering files smaller than %d bytes...\n", minFileSize)
		applySizeFilter(workingSet, minFileSize)
	}

	// Apply non-log inclusion preference
	if !includeNonLogs {
		fmt.Println("Excluding non-log files...")
		workingSet.SelectLogFiles()
	}

	// Print summary
	printWorkingSetSummary(workingSet)

	// Save configuration
	if err := saveWorkingSetConfig(workingSet, outputConfig); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("\nConfiguration saved to: %s\n", outputConfig)
	fmt.Printf("Use 'logninja load-config %s' to load this configuration\n", outputConfig)

	return nil
}

// createIntelligentWorkingSet creates a working set with smart defaults
func createIntelligentWorkingSet(bundle *models.Bundle) *models.WorkingSet {
	workingSet := models.NewWorkingSet(bundle)

	// Add some common exclude patterns by default
	commonExcludes := []string{
		`\.DS_Store$`,   // macOS metadata
		`\.git/`,        // Git directories
		`node_modules/`, // Node.js dependencies
		`\.pyc$`,        // Python compiled files
		`\.tmp$`,        // Temporary files
		`\.swp$`,        // Vim swap files
		`\.lock$`,       // Lock files
	}

	for _, pattern := range commonExcludes {
		workingSet.AddExcludeRegex(pattern)
	}

	return workingSet
}

// applySmartSelection applies intelligent selection rules
func applySmartSelection(ws *models.WorkingSet) {
	if ws.Bundle == nil {
		return
	}

	// Prioritize larger log files (they likely contain more useful data)
	largeFiles := ws.GetSelectedFilesBySize(0) // Get all files sorted by size

	// If we have many files, be more selective
	totalLogFiles := len(ws.Bundle.GetLogFiles())
	if totalLogFiles > 50 {
		// For many files, prefer larger ones and recent ones
		ws.SelectNone()

		// Select top 70% by size
		selectCount := int(float64(totalLogFiles) * 0.7)
		if selectCount < 10 {
			selectCount = 10 // Always select at least 10 files
		}
		if selectCount > totalLogFiles {
			selectCount = totalLogFiles
		}

		selectedCount := 0
		for _, file := range largeFiles {
			if file.IsLogFile && selectedCount < selectCount {
				ws.SetFileSelection(file.Path, true)
				selectedCount++
			}
		}
	}
	// If we have few files, keep them all selected (default behavior)
}

// applySizeFilter excludes files smaller than minSize
func applySizeFilter(ws *models.WorkingSet, minSize int64) {
	if ws.Bundle == nil {
		return
	}

	for _, file := range ws.Bundle.Files {
		if file.Size < minSize {
			ws.SetFileSelection(file.Path, false)
		}
	}
}

// printWorkingSetSummary prints a summary of the working set
func printWorkingSetSummary(ws *models.WorkingSet) {
	if ws.Bundle == nil {
		return
	}

	selectedCount := 0
	selectedSize := int64(0)
	logFileCount := 0

	for _, file := range ws.Bundle.Files {
		if ws.IsFileSelected(file.Path) {
			selectedCount++
			selectedSize += file.Size
			if file.IsLogFile {
				logFileCount++
			}
		}
	}

	fmt.Println("\nWorking Set Summary:")
	fmt.Printf("  Total files in bundle: %d\n", len(ws.Bundle.Files))
	fmt.Printf("  Selected files: %d\n", selectedCount)
	fmt.Printf("  Selected log files: %d\n", logFileCount)
	fmt.Printf("  Total size of selected files: %.2f MB\n", float64(selectedSize)/(1024*1024))
	fmt.Printf("  Include patterns: %d\n", len(ws.IncludeRegex))
	fmt.Printf("  Exclude patterns: %d\n", len(ws.ExcludeRegex))

	if ws.HasTimeFilter() {
		fmt.Printf("  Time filter: %s - %s\n",
			ws.TimeFilter.Start.Format("2006-01-02 15:04:05"),
			ws.TimeFilter.End.Format("2006-01-02 15:04:05"))
	}
}

// WorkingSetConfig represents the serializable configuration
type WorkingSetConfig struct {
	BundlePath    string            `json:"bundle_path"`
	SelectedFiles map[string]bool   `json:"selected_files"`
	IncludeRegex  []string          `json:"include_regex"`
	ExcludeRegex  []string          `json:"exclude_regex"`
	TimeFilter    *models.TimeRange `json:"time_filter,omitempty"`
	Metadata      ConfigMetadata    `json:"metadata"`
}

// ConfigMetadata contains metadata about the configuration
type ConfigMetadata struct {
	CreatedAt       string `json:"created_at"`
	LogNinjaVersion string `json:"logninja_version"`
	TotalFiles      int    `json:"total_files"`
	SelectedFiles   int    `json:"selected_files"`
	BundleSize      int64  `json:"bundle_size"`
}

// saveWorkingSetConfig saves the working set configuration to a file
func saveWorkingSetConfig(ws *models.WorkingSet, filename string) error {
	selectedCount := 0
	for _, selected := range ws.SelectedFiles {
		if selected {
			selectedCount++
		}
	}

	config := WorkingSetConfig{
		BundlePath:    ws.Bundle.Path,
		SelectedFiles: ws.SelectedFiles,
		IncludeRegex:  ws.IncludeRegex,
		ExcludeRegex:  ws.ExcludeRegex,
		TimeFilter:    ws.TimeFilter,
		Metadata: ConfigMetadata{
			CreatedAt:       ws.LastUpdated.Format("2006-01-02T15:04:05Z"),
			LogNinjaVersion: "0.1.0", // TODO: Get from version
			TotalFiles:      len(ws.Bundle.Files),
			SelectedFiles:   selectedCount,
			BundleSize:      ws.Bundle.TotalSize,
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}
