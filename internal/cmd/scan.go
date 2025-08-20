package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cheerioskun/logninja/internal/scanner"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	quickScan bool
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a directory for log files using content analysis",
	Long: `Scan a directory and identify log files through content-based detection.

This command uses a two-phase approach:
1. Collect ALL files in the directory tree
2. Analyze file contents to identify log files (timestamp detection)

Provides an overview of what would be available in the TUI:
- Total file count  
- Log files identified by content analysis
- File sizes and metadata
- Two-phase scanning statistics

Examples:
  logninja scan /var/log
  logninja scan ./my-logs --quick
  logninja scan /path/to/logs --max-depth 3`,
	Args: cobra.ExactArgs(1),
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	// Scan-specific flags
	scanCmd.Flags().IntVar(&maxDepth, "max-depth", 10, "maximum directory depth to scan")
	scanCmd.Flags().BoolVar(&quickScan, "quick", false, "perform quick scan (top-level only)")

	// Bind flags to viper
	viper.BindPFlag("quick", scanCmd.Flags().Lookup("quick"))
}

func runScan(cmd *cobra.Command, args []string) error {
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

	// Create filesystem interface
	fs := afero.NewOsFs()

	// Create bundle scanner
	bundleScanner := scanner.NewBundleScanner(fs)
	bundleScanner.SetMaxDepth(maxDepth)

	fmt.Printf("Scanning: %s\n", absPath)
	fmt.Printf("Max depth: %d\n", maxDepth)
	fmt.Println()

	if quickScan {
		// Quick scan
		metadata, err := bundleScanner.QuickScan(absPath)
		if err != nil {
			return fmt.Errorf("quick scan failed: %w", err)
		}

		fmt.Println("Quick Scan Results:")
		fmt.Printf("  Total files: %d\n", metadata.TotalFileCount)
		fmt.Printf("  Log files: %d\n", metadata.LogFileCount)
		fmt.Printf("  Scan depth: %d\n", metadata.ScanDepth)

	} else {
		// Full scan
		bundle, err := bundleScanner.ScanBundle(absPath)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		fmt.Println("Full Scan Results:")
		fmt.Printf("  Bundle path: %s\n", bundle.Path)
		fmt.Printf("  Total files: %d\n", bundle.Metadata.TotalFileCount)
		fmt.Printf("  Log files: %d\n", bundle.Metadata.LogFileCount)
		fmt.Printf("  Total size: %s\n", formatBytes(bundle.TotalSize))
		fmt.Printf("  Scan time: %s\n", bundle.ScanTime.Format("2006-01-02 15:04:05"))

		if bundle.TimeRange != nil {
			fmt.Printf("  Time range: %s\n", bundle.TimeRange.String())
		} else {
			fmt.Printf("  Time range: (not available - timestamps not parsed)\n")
		}

		fmt.Println()

		// Show scanning approach information
		fmt.Println("Scanning approach: Two-phase content-based detection")
		fmt.Printf("  Phase 1: Collected %d total files\n", len(bundleScanner.GetAllFiles()))
		fmt.Printf("  Phase 2: Identified %d log files by content analysis\n", len(bundleScanner.GetWorkingSet()))

		if viper.GetBool("verbose") {
			fmt.Println()
			fmt.Println("Files discovered:")

			logFiles := 0
			otherFiles := 0

			for _, file := range bundle.Files {
				if file.IsLogFile {
					logFiles++
					if logFiles <= 10 { // Show first 10 log files
						fmt.Printf("  [LOG] %s (%s, detected by content analysis)\n",
							file.Path, formatBytes(file.Size))
					}
				} else {
					otherFiles++
					if otherFiles <= 5 { // Show first 5 other files
						fmt.Printf("  [FILE] %s (%s)\n",
							file.Path, formatBytes(file.Size))
					}
				}
			}

			if logFiles > 10 {
				fmt.Printf("  ... and %d more log files\n", logFiles-10)
			}
			if otherFiles > 5 {
				fmt.Printf("  ... and %d more other files\n", otherFiles-5)
			}
		}
	}

	return nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
