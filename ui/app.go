package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheerioskun/logninja/internal/messages"
	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/ui/filelist"
	"github.com/cheerioskun/logninja/ui/histogram"
	"github.com/cheerioskun/logninja/ui/regex"
	"github.com/spf13/afero"
)

// FocusedPanel represents which panel is currently focused
type FocusedPanel int

const (
	FileTreePanel FocusedPanel = iota
	RegexPanel
	FileListPanel
	HistogramPanel
	TimeRangePanel
	StatusPanel
)

// AppModel represents the main application model
type AppModel struct {
	// Core state
	workingSet *models.WorkingSet

	// UI Components
	regexPanel     *regex.Model
	fileListPanel  *filelist.Model
	histogramPanel *histogram.Model

	// UI state
	focused      FocusedPanel
	width        int
	height       int
	panels       []FocusedPanel
	currentPanel int

	// Status
	status   string
	ready    bool
	quitting bool
}

// NewAppModel creates a new application model
func NewAppModel(workingSet *models.WorkingSet, fs afero.Fs) *AppModel {
	// Create unified regex panel
	regexPanel := regex.NewModel()
	fileListPanel := filelist.NewModel()
	histogramPanel := histogram.NewModel(fs)

	if workingSet != nil && workingSet.Bundle != nil {
		var filePaths []string
		for _, file := range workingSet.Bundle.Files {
			filePaths = append(filePaths, file.Path)
		}
		regexPanel.SetFiles(filePaths)
		histogramPanel.SetWorkingSet(workingSet)
	}

	return &AppModel{
		workingSet:     workingSet,
		regexPanel:     regexPanel,
		fileListPanel:  fileListPanel,
		histogramPanel: histogramPanel,
		focused:        FileListPanel,
		width:          80,
		height:         24,
		panels:         []FocusedPanel{FileListPanel, RegexPanel, HistogramPanel},
		currentPanel:   0,
		status:         "Ready",
		ready:          true,
		quitting:       false,
	}
}

// Init implements tea.Model
func (m *AppModel) Init() tea.Cmd {
	// Initialize components with initial data
	if m.workingSet != nil {
		return m.broadcastWorkingSetUpdate()
	}
	return nil
}

// Update implements tea.Model
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case messages.RegexFiltersChangedMsg:
		// Handle ordered regex filter changes
		return m, m.handleRegexFiltersChange(msg)

	case messages.WorkingSetUpdatedMsg:
		// Handle working set updates and notify histogram panel
		m.status = fmt.Sprintf("Working set updated: %d files selected", msg.SelectedCount)

		// Forward to histogram panel with proper message type
		histogramMsg := histogram.WorkingSetUpdatedMsg{WorkingSet: m.workingSet}
		var cmd tea.Cmd
		m.histogramPanel, cmd = m.histogramPanel.Update(histogramMsg)
		return m, cmd

	case filelist.FileListDataMsg:
		// Forward file list data to the file list component
		var cmd tea.Cmd
		m.fileListPanel, cmd = m.fileListPanel.Update(msg)
		return m, cmd

	case histogram.HistogramDataMsg, histogram.HistogramErrorMsg, histogram.HistogramLoadingMsg:
		// Forward histogram messages to the histogram component
		var cmd tea.Cmd
		m.histogramPanel, cmd = m.histogramPanel.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "tab":
			m.nextPanel()
			return m, nil

		case "shift+tab":
			m.prevPanel()
			return m, nil

		case "?":
			// Show help (placeholder)
			m.status = "Help: Tab/Shift+Tab to navigate, q to quit"
			return m, nil

		default:
			// Forward message to focused panel and collect commands
			if m.focused == RegexPanel {
				var cmd tea.Cmd
				m.regexPanel, cmd = m.regexPanel.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.focused == FileListPanel {
				var cmd tea.Cmd
				m.fileListPanel, cmd = m.fileListPanel.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.focused == HistogramPanel {
				var cmd tea.Cmd
				m.histogramPanel, cmd = m.histogramPanel.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	// Return batched commands if any
	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// View implements tea.Model
func (m *AppModel) View() string {
	if m.quitting {
		return "Thanks for using LogNinja!\n"
	}

	if !m.ready {
		return "Loading...\n"
	}

	// Create layout
	content := m.renderLayout()

	return content
}

// renderLayout creates the main application layout
func (m *AppModel) renderLayout() string {
	// Calculate dimensions for panels
	headerHeight := 3
	statusHeight := 3
	contentHeight := m.height - headerHeight - statusHeight

	// Split content area - 3 panels on top, 3 on bottom
	topPanelWidth := m.width / 3
	bottomPanelWidth := m.width / 3
	topHeight := contentHeight * 2 / 3
	bottomHeight := contentHeight - topHeight

	// Create header
	header := m.renderHeader()

	// Create panel contents
	fileTree := m.renderFileTreePanel(topPanelWidth, topHeight)
	regexPanel := m.renderRegexPanel(topPanelWidth, topHeight)
	fileList := m.renderFileListPanel(bottomPanelWidth, bottomHeight)
	histogram := m.renderHistogramPanel(bottomPanelWidth, bottomHeight)
	timeRange := m.renderTimeRangePanel(bottomPanelWidth, bottomHeight)

	// Create status
	status := m.renderStatusPanel(m.width, statusHeight)

	// Combine panels
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, fileTree, regexPanel)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, fileList, histogram, timeRange)
	content := lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)

	// Combine all
	return lipgloss.JoinVertical(lipgloss.Left, header, content, status)
}

// renderHeader creates the application header
func (m *AppModel) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("LogNinja - Log Bundle Refinement Tool")

	bundlePath := ""
	if m.workingSet != nil && m.workingSet.Bundle != nil {
		bundlePath = m.workingSet.Bundle.Path
	}

	path := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf("Bundle: %s", bundlePath))

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Tab: Navigate | ?: Help | q: Quit")

	return lipgloss.JoinVertical(lipgloss.Left, title, path, help)
}

// renderFileTreePanel renders the file tree panel
func (m *AppModel) renderFileTreePanel(width, height int) string {
	style := m.getPanelStyle(FileTreePanel, width, height)

	title := "File Tree"
	content := m.renderFileTreeContent()

	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

// renderRegexPanel renders the unified regex patterns panel
func (m *AppModel) renderRegexPanel(width, height int) string {
	style := m.getPanelStyle(RegexPanel, width, height)

	// Set component size and focus state
	m.regexPanel.SetSize(width-4, height-4) // Account for border and padding
	if m.focused == RegexPanel {
		m.regexPanel.Focus()
	} else {
		m.regexPanel.Blur()
	}

	// Get the component's view
	content := m.regexPanel.View()

	return style.Render(content)
}

// renderFileListPanel renders the file list panel
func (m *AppModel) renderFileListPanel(width, height int) string {
	style := m.getPanelStyle(FileListPanel, width, height)

	// Set component size and focus state
	m.fileListPanel.SetSize(width-4, height-4) // Account for border and padding
	if m.focused == FileListPanel {
		m.fileListPanel.Focus()
	} else {
		m.fileListPanel.Blur()
	}

	// Get the component's view
	content := m.fileListPanel.View()

	return style.Render(content)
}

// renderHistogramPanel renders the histogram panel
func (m *AppModel) renderHistogramPanel(width, height int) string {
	style := m.getPanelStyle(HistogramPanel, width, height)

	// Set component size and focus state
	m.histogramPanel.SetSize(width-4, height-4) // Account for border and padding
	if m.focused == HistogramPanel {
		m.histogramPanel.Focus()
	} else {
		m.histogramPanel.Blur()
	}

	// Get the component's view
	content := m.histogramPanel.View()

	return style.Render(content)
}

// renderTimeRangePanel renders the time range panel
func (m *AppModel) renderTimeRangePanel(width, height int) string {
	style := m.getPanelStyle(TimeRangePanel, width, height)

	title := "Time Range"
	content := m.renderTimeRangeContent()

	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

// renderStatusPanel renders the status panel
func (m *AppModel) renderStatusPanel(width, height int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width-2).
		Height(height-2).
		Padding(0, 1)

	var statusParts []string

	if m.workingSet != nil {
		selectedCount := m.workingSet.GetSelectedFileCount()
		totalCount := 0
		if m.workingSet.Bundle != nil {
			totalCount = len(m.workingSet.Bundle.Files)
		}

		statusParts = append(statusParts,
			fmt.Sprintf("Files: %d/%d selected", selectedCount, totalCount),
			fmt.Sprintf("Size: %s", formatBytes(m.workingSet.EstimatedSize)),
			fmt.Sprintf("Status: %s", m.status),
		)
	}

	return style.Render(strings.Join(statusParts, " | "))
}

// Panel content rendering methods (placeholders for Phase 1)

func (m *AppModel) renderFileTreeContent() string {
	if m.workingSet == nil || m.workingSet.Bundle == nil {
		return "No bundle loaded"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Total files: %d", len(m.workingSet.Bundle.Files)))
	lines = append(lines, fmt.Sprintf("Log files: %d", m.workingSet.Bundle.Metadata.LogFileCount))

	// Show first few files as example
	for i, file := range m.workingSet.Bundle.Files {
		if i >= 10 { // Limit to first 10 files for now
			lines = append(lines, "...")
			break
		}

		marker := "□"
		if m.workingSet.IsFileSelected(file.Path) {
			marker = "☑"
		}

		lines = append(lines, fmt.Sprintf("%s %s", marker, file.Path))
	}

	return strings.Join(lines, "\n")
}

func (m *AppModel) renderRegexContent() string {
	var lines []string

	// Extract patterns from ordered filters for display
	var includePatterns []string
	var excludePatterns []string

	for _, filter := range m.workingSet.RegexFilters {
		if filter.Valid {
			if filter.Take {
				includePatterns = append(includePatterns, filter.Pattern)
			} else {
				excludePatterns = append(excludePatterns, filter.Pattern)
			}
		}
	}

	lines = append(lines, "Include patterns:")
	if len(includePatterns) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, pattern := range includePatterns {
			lines = append(lines, fmt.Sprintf("  + %s", pattern))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "Exclude patterns:")
	if len(excludePatterns) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, pattern := range excludePatterns {
			lines = append(lines, fmt.Sprintf("  - %s", pattern))
		}
	}

	return strings.Join(lines, "\n")
}

func (m *AppModel) renderHistogramContent() string {
	if len(m.workingSet.VolumeData) == 0 {
		return "No volume data available\n(Will be generated in Phase 4)"
	}

	return "Volume histogram placeholder\n(Implementation in Phase 4)"
}

func (m *AppModel) renderTimeRangeContent() string {
	var lines []string

	if m.workingSet.Bundle != nil && m.workingSet.Bundle.TimeRange != nil {
		lines = append(lines, "Bundle time range:")
		lines = append(lines, fmt.Sprintf("  Start: %s", m.workingSet.Bundle.TimeRange.Start.Format("2006-01-02 15:04:05")))
		lines = append(lines, fmt.Sprintf("  End:   %s", m.workingSet.Bundle.TimeRange.End.Format("2006-01-02 15:04:05")))
	} else {
		lines = append(lines, "No time range available")
		lines = append(lines, "(Timestamps not yet parsed)")
	}

	lines = append(lines, "")

	if m.workingSet.HasTimeFilter() {
		lines = append(lines, "Active filter:")
		lines = append(lines, fmt.Sprintf("  Start: %s", m.workingSet.TimeFilter.Start.Format("2006-01-02 15:04:05")))
		lines = append(lines, fmt.Sprintf("  End:   %s", m.workingSet.TimeFilter.End.Format("2006-01-02 15:04:05")))
	} else {
		lines = append(lines, "No time filter active")
	}

	return strings.Join(lines, "\n")
}

// Helper methods

func (m *AppModel) getPanelStyle(panel FocusedPanel, width, height int) lipgloss.Style {
	borderColor := lipgloss.Color("240")
	if panel == m.focused {
		borderColor = lipgloss.Color("205")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width-2).
		Height(height-2).
		Padding(0, 1)
}

func (m *AppModel) nextPanel() {
	m.currentPanel = (m.currentPanel + 1) % len(m.panels)
	m.focused = m.panels[m.currentPanel]
}

func (m *AppModel) prevPanel() {
	m.currentPanel = (m.currentPanel - 1 + len(m.panels)) % len(m.panels)
	m.focused = m.panels[m.currentPanel]
}

// handleRegexFiltersChange processes ordered regex filter changes
func (m *AppModel) handleRegexFiltersChange(msg messages.RegexFiltersChangedMsg) tea.Cmd {
	if m.workingSet == nil {
		return nil
	}

	// Update the working set with the new ordered filter list
	m.workingSet.SetRegexFilters(msg.Filters)

	// Apply filtering and broadcast the update
	m.applyOrderedRegexFiltering()

	// Return command to broadcast working set update to other components
	return m.broadcastWorkingSetUpdate()
}

// broadcastWorkingSetUpdate creates a command to notify other components of working set changes
func (m *AppModel) broadcastWorkingSetUpdate() tea.Cmd {
	if m.workingSet == nil {
		return nil
	}

	selectedCount := m.workingSet.GetSelectedFileCount()
	totalSize := m.workingSet.GetSelectedTotalSize()

	// Get list of filtered files for other components
	filteredFiles := m.getFilteredFileNames()

	// Get all files sorted by size for the file list component
	allFiles := m.workingSet.GetSelectedFilesBySize(0)

	// Return batched commands for different components
	return tea.Batch(
		func() tea.Msg {
			return messages.WorkingSetUpdatedMsg{
				SelectedCount: selectedCount,
				TotalMatched:  len(filteredFiles),
				FilteredFiles: filteredFiles,
				TotalSize:     totalSize,
			}
		},
		func() tea.Msg {
			return filelist.FileListDataMsg{
				Files:      allFiles,
				TotalSize:  totalSize,
				TotalFiles: selectedCount,
			}
		},
	)
}

// getFilteredFileNames returns the names of files that pass the current filters
func (m *AppModel) getFilteredFileNames() []string {
	if m.workingSet == nil || m.workingSet.Bundle == nil {
		return []string{}
	}

	var filteredFiles []string
	for _, file := range m.workingSet.Bundle.Files {
		if m.workingSet.IsFileSelected(file.Path) {
			filteredFiles = append(filteredFiles, file.Path)
		}
	}
	return filteredFiles
}

// applyOrderedRegexFiltering applies regex filters in order (take/exclude)
// Files are selected IF AND ONLY IF the last regex that matched them was an include regex.
func (m *AppModel) applyOrderedRegexFiltering() {
	if m.workingSet == nil || m.workingSet.Bundle == nil {
		return
	}

	// Start with empty working set (no files selected)
	for _, file := range m.workingSet.Bundle.Files {
		m.workingSet.SetFileSelection(file.Path, false)
	}

	// Track the last matching regex for each file
	lastMatchingRegex := make(map[string]*models.RegexFilter)

	// Apply each regex filter in order, tracking the last match for each file
	for _, filter := range m.workingSet.RegexFilters {
		if !filter.Valid || filter.Compiled == nil {
			continue
		}

		for _, file := range m.workingSet.Bundle.Files {
			// Check if the pattern matches this file path
			if filter.Compiled.MatchString(file.Path) {
				// Track this as the last matching regex for this file
				filterCopy := filter // Make a copy to store in the map
				lastMatchingRegex[file.Path] = &filterCopy
			}
		}
	}

	// Select files where the last matching regex was an include regex (Take = true)
	for _, file := range m.workingSet.Bundle.Files {
		if lastMatch, exists := lastMatchingRegex[file.Path]; exists {
			// File is selected only if the last matching regex was an include regex
			m.workingSet.SetFileSelection(file.Path, lastMatch.Take)
		} else {
			// No regex matched this file, so it remains unselected
			m.workingSet.SetFileSelection(file.Path, false)
		}
	}
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
