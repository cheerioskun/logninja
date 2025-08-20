package histogram

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles for histogram rendering
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	barStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	emptyBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("111"))
)

// renderHistogram renders the main histogram view
func (m *Model) renderHistogram() string {
	var parts []string

	// Title - always showing bytes now
	title := titleStyle.Render("Log Volume Histogram (Bytes)")
	parts = append(parts, title)

	// Histogram bars
	histogram := m.renderHistogramBars()
	parts = append(parts, histogram)

	// Status and help
	status := m.renderStatus()
	help := m.renderHelp()
	parts = append(parts, status, help)

	return strings.Join(parts, "\n")
}

// renderHistogramBars creates the ASCII art histogram
func (m *Model) renderHistogramBars() string {
	if len(m.volumeData) == 0 {
		return "No data"
	}

	// Calculate maximum value for scaling
	var maxValue int64
	for _, point := range m.volumeData {
		value := point.Size // Always use bytes now
		if value > maxValue {
			maxValue = value
		}
	}

	if maxValue == 0 {
		return "No log data in time range"
	}

	var lines []string
	availableHeight := m.height - 6 // Reserve space for title, status, help
	barsToShow := len(m.volumeData)

	// Limit number of bars if too many for height
	if barsToShow > availableHeight {
		barsToShow = availableHeight
	}

	// Calculate time range for labels
	timeRange := m.getTimeRange()

	for i := 0; i < barsToShow; i++ {
		point := m.volumeData[i]

		// Get value to display (always bytes now)
		value := point.Size

		// Calculate bar length
		barLength := int((float64(value) / float64(maxValue)) * float64(m.maxBarWidth))

		// Create bar
		bar := m.createBar(barLength, value > 0)

		// Create time label
		timeLabel := m.formatTimeLabel(point.BinStart, timeRange)

		// Create value label
		valueLabel := m.formatValueLabel(value, point.FileCount)

		// Combine into line
		line := fmt.Sprintf("%s %s %s", timeLabel, bar, valueLabel)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// createBar creates a single histogram bar
func (m *Model) createBar(length int, hasData bool) string {
	if length <= 0 {
		return emptyBarStyle.Render("▏")
	}

	// Create bar with different characters for visual appeal
	bar := strings.Repeat("█", length)

	if hasData {
		return barStyle.Render(bar)
	}
	return emptyBarStyle.Render(bar)
}

// formatTimeLabel creates a time label for the histogram
func (m *Model) formatTimeLabel(t time.Time, timeRange time.Duration) string {
	var format string

	// Choose format based on time range
	if timeRange < time.Hour {
		format = "15:04"
	} else if timeRange < 24*time.Hour {
		format = "15:04"
	} else if timeRange < 30*24*time.Hour {
		format = "01-02"
	} else {
		format = "01-02"
	}

	label := t.Format(format)
	return labelStyle.Render(fmt.Sprintf("%-5s", label))
}

// formatValueLabel creates a value label for the histogram
func (m *Model) formatValueLabel(value int64, fileCount int) string {
	// Always format as bytes now
	valueStr := formatBytes(value)

	label := valueStr
	if m.showFiles && fileCount > 0 {
		label = fmt.Sprintf("%s (%df)", valueStr, fileCount)
	}

	return labelStyle.Render(label)
}

// getTimeRange calculates the total time range of the histogram
func (m *Model) getTimeRange() time.Duration {
	if len(m.volumeData) < 2 {
		return time.Hour
	}

	start := m.volumeData[0].BinStart
	end := m.volumeData[len(m.volumeData)-1].BinEnd
	return end.Sub(start)
}

// renderStatus renders the status line
func (m *Model) renderStatus() string {
	if m.status == "" {
		return ""
	}

	status := statusStyle.Render(m.status)

	// Add summary statistics
	if len(m.volumeData) > 0 {
		var total int64
		var peakValue int64
		nonEmptyBins := 0

		for _, point := range m.volumeData {
			value := point.Size // Always use bytes now

			total += value
			if value > peakValue {
				peakValue = value
			}
			if value > 0 {
				nonEmptyBins++
			}
		}

		// Always format as bytes now
		totalStr := formatBytes(total)
		peakStr := formatBytes(peakValue)

		summary := fmt.Sprintf("Total: %s | Peak: %s | Active bins: %d/%d",
			totalStr, peakStr, nonEmptyBins, len(m.volumeData))

		return fmt.Sprintf("%s\n%s", status, statusStyle.Render(summary))
	}

	return status
}

// renderHelp renders the help text
func (m *Model) renderHelp() string {
	if !m.focused {
		return ""
	}

	var helpParts []string
	helpParts = append(helpParts, "r:refresh")
	if m.showFiles {
		helpParts = append(helpParts, "f:hide files")
	} else {
		helpParts = append(helpParts, "f:show files")
	}
	helpParts = append(helpParts, "+/-:bins")

	help := strings.Join(helpParts, " | ")
	return helpStyle.Render(help)
}

// renderLoading renders the loading state
func (m *Model) renderLoading() string {
	title := titleStyle.Render("Volume Histogram")
	loading := "Generating histogram..."

	// Simple loading animation could be added here
	dots := strings.Repeat(".", (int(time.Now().Unix())%4)+1)
	loadingLine := fmt.Sprintf("%s%s", loading, dots)

	return fmt.Sprintf("%s\n\n%s", title, loadingLine)
}

// renderError renders the error state
func (m *Model) renderError() string {
	title := titleStyle.Render("Volume Histogram")
	error := errorStyle.Render(fmt.Sprintf("Error: %s", m.error))
	help := helpStyle.Render("Press 'r' to retry")

	return fmt.Sprintf("%s\n\n%s\n%s", title, error, help)
}

// renderEmpty renders the empty state
func (m *Model) renderEmpty() string {
	title := titleStyle.Render("Volume Histogram")
	empty := "No volume data available"
	help := helpStyle.Render("Select files and press 'r' to generate histogram")

	return fmt.Sprintf("%s\n\n%s\n%s", title, empty, help)
}

// Utility functions

// formatBytes formats byte count in human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatNumber formats large numbers with K/M/B suffixes
func formatNumber(num int64) string {
	if num < 1000 {
		return fmt.Sprintf("%d", num)
	}
	if num < 1000000 {
		return fmt.Sprintf("%.1fK", float64(num)/1000)
	}
	if num < 1000000000 {
		return fmt.Sprintf("%.1fM", float64(num)/1000000)
	}
	return fmt.Sprintf("%.1fB", float64(num)/1000000000)
}
