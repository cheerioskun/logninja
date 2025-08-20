package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/internal/scanner"
	"github.com/cheerioskun/logninja/ui"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	maxDepth int
)

// tuiCmd represents the tui command
var tuiCmd = &cobra.Command{
	Use:   "tui [path]",
	Short: "Start the interactive TUI interface",
	Long: `Start the interactive Terminal User Interface for log bundle refinement.

The TUI provides:
- File tree view with selection capabilities
- Regex pattern management (include/exclude)
- Volume histogram visualization
- Time range selection
- Real-time size estimation

Examples:
  logninja tui /var/log
  logninja tui ./my-logs --max-depth 5`,
	Args: cobra.ExactArgs(1),
	RunE: runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)

	// TUI-specific flags
	tuiCmd.Flags().IntVar(&maxDepth, "max-depth", 10, "maximum directory depth to scan")

	// Bind flags to viper
	viper.BindPFlag("max-depth", tuiCmd.Flags().Lookup("max-depth"))
}

func runTUI(cmd *cobra.Command, args []string) error {
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
	scanner := scanner.NewBundleScanner(fs)
	scanner.SetMaxDepth(maxDepth)

	if viper.GetBool("verbose") {
		fmt.Fprintf(os.Stderr, "Scanning bundle at: %s\n", absPath)
		fmt.Fprintf(os.Stderr, "Max depth: %d\n", maxDepth)
	}

	// Scan the bundle
	bundle, err := scanner.ScanBundle(absPath)
	if err != nil {
		return fmt.Errorf("failed to scan bundle: %w", err)
	}

	if viper.GetBool("verbose") {
		fmt.Fprintf(os.Stderr, "Found %d files (%d log files)\n",
			bundle.Metadata.TotalFileCount, bundle.Metadata.LogFileCount)
	}

	// Create working set
	workingSet := models.NewWorkingSet(bundle)

	// Initialize TUI
	model := ui.NewAppModel(workingSet, fs)

	// Start the TUI program
	program := tea.NewProgram(model, tea.WithAltScreen())

	if viper.GetBool("verbose") {
		fmt.Fprintf(os.Stderr, "Starting TUI...\n")
	}

	_, err = program.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
