package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheerioskun/logninja/internal/messages"
	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/ui/filelist"
	"github.com/cheerioskun/logninja/ui/regex"
)

// FocusedPanel represents which panel is currently focused
type FocusedPanel int

const (
	FileTreePanel FocusedPanel = iota
	IncludePanel
	ExcludePanel
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
	includePanel  *regex.SingleModel
	excludePanel  *regex.SingleModel
	fileListPanel *filelist.Model

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
func NewAppModel(workingSet *models.WorkingSet) *AppModel {
	// Create separate include and exclude panels
	includePanel := regex.NewSingleModel(regex.IncludeType)
	excludePanel := regex.NewSingleModel(regex.ExcludeType)
	fileListPanel := filelist.NewModel()

	if workingSet != nil && workingSet.Bundle != nil {
		var filePaths []string
		for _, file := range workingSet.Bundle.Files {
			filePaths = append(filePaths, file.Path)
		}
		includePanel.SetFiles(filePaths)
		excludePanel.SetFiles(filePaths)
	}

	return &AppModel{
		workingSet:    workingSet,
		includePanel:  includePanel,
		excludePanel:  excludePanel,
		fileListPanel: fileListPanel,
		focused:       FileTreePanel,
		width:         80,
		height:        24,
		panels:        []FocusedPanel{FileTreePanel, IncludePanel, ExcludePanel, FileListPanel, HistogramPanel, TimeRangePanel},
		currentPanel:  0,
		status:        "Ready",
		ready:         true,
		quitting:      false,
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

	case messages.RegexPatternsChangedMsg:
		// Handle regex pattern changes
		return m, m.handleRegexPatternChange(msg)

	case messages.WorkingSetUpdatedMsg:
		// Handle working set updates (for future components like file list)
		m.status = fmt.Sprintf("Working set updated: %d files selected", msg.SelectedCount)
		return m, nil

	case filelist.FileListDataMsg:
		// Forward file list data to the file list component
		var cmd tea.Cmd
		m.fileListPanel, cmd = m.fileListPanel.Update(msg)
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
			if m.focused == IncludePanel {
				var cmd tea.Cmd
				m.includePanel, cmd = m.includePanel.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.focused == ExcludePanel {
				var cmd tea.Cmd
				m.excludePanel, cmd = m.excludePanel.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else if m.focused == FileListPanel {
				var cmd tea.Cmd
				m.fileListPanel, cmd = m.fileListPanel.Update(msg)
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
	include := m.renderIncludePanel(topPanelWidth, topHeight)
	exclude := m.renderExcludePanel(topPanelWidth, topHeight)
	fileList := m.renderFileListPanel(bottomPanelWidth, bottomHeight)
	histogram := m.renderHistogramPanel(bottomPanelWidth, bottomHeight)
	timeRange := m.renderTimeRangePanel(bottomPanelWidth, bottomHeight)

	// Create status
	status := m.renderStatusPanel(m.width, statusHeight)

	// Combine panels
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, fileTree, include, exclude)
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

// renderIncludePanel renders the include patterns panel
func (m *AppModel) renderIncludePanel(width, height int) string {
	style := m.getPanelStyle(IncludePanel, width, height)

	// Set component size and focus state
	m.includePanel.SetSize(width-4, height-4) // Account for border and padding
	if m.focused == IncludePanel {
		m.includePanel.Focus()
	} else {
		m.includePanel.Blur()
	}

	// Get the component's view
	content := m.includePanel.View()

	return style.Render(content)
}

// renderExcludePanel renders the exclude patterns panel
func (m *AppModel) renderExcludePanel(width, height int) string {
	style := m.getPanelStyle(ExcludePanel, width, height)

	// Set component size and focus state
	m.excludePanel.SetSize(width-4, height-4) // Account for border and padding
	if m.focused == ExcludePanel {
		m.excludePanel.Focus()
	} else {
		m.excludePanel.Blur()
	}

	// Get the component's view
	content := m.excludePanel.View()

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

	title := "Volume Histogram"
	content := m.renderHistogramContent()

	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
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

	lines = append(lines, "Include patterns:")
	if len(m.workingSet.IncludeRegex) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, pattern := range m.workingSet.IncludeRegex {
			lines = append(lines, fmt.Sprintf("  + %s", pattern))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "Exclude patterns:")
	if len(m.workingSet.ExcludeRegex) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, pattern := range m.workingSet.ExcludeRegex {
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

// handleRegexPatternChange processes regex pattern change messages
func (m *AppModel) handleRegexPatternChange(msg messages.RegexPatternsChangedMsg) tea.Cmd {
	if m.workingSet == nil {
		return nil
	}

	// Update the appropriate pattern list based on message type
	switch msg.Type {
	case messages.IncludePatternType:
		m.workingSet.IncludeRegex = msg.Patterns
	case messages.ExcludePatternType:
		m.workingSet.ExcludeRegex = msg.Patterns
	}

	// Apply filtering and broadcast the update
	m.applyRegexFiltering()

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

// applyRegexFiltering applies regex patterns to filter file selections
// First applies all include patterns, then applies all exclude patterns
func (m *AppModel) applyRegexFiltering() {
	if m.workingSet == nil || m.workingSet.Bundle == nil {
		return
	}

	includePatterns := m.workingSet.IncludeRegex
	excludePatterns := m.workingSet.ExcludeRegex

	// Step 1: Apply include patterns to get initial set
	// If no include patterns, start with all files
	var includedFiles []string
	if len(includePatterns) == 0 {
		// No include patterns means include all files
		for _, file := range m.workingSet.Bundle.Files {
			includedFiles = append(includedFiles, file.Path)
		}
	} else {
		// Apply all include patterns
		includedFileSet := make(map[string]bool)
		for _, pattern := range includePatterns {
			for _, file := range m.workingSet.Bundle.Files {
				filename := file.Path

				// Try filepath pattern matching
				if matched, _ := filepath.Match(pattern, filename); matched {
					includedFileSet[filename] = true
					continue
				}

				// Try regex pattern matching
				if regex, err := regexp.Compile(pattern); err == nil && regex.MatchString(filename) {
					includedFileSet[filename] = true
				}
			}
		}

		// Convert set to slice
		for filename := range includedFileSet {
			includedFiles = append(includedFiles, filename)
		}
	}

	// Step 2: Apply exclude patterns to remove files from included set
	var filteredFiles []string
	if len(excludePatterns) == 0 {
		// No exclude patterns means keep all included files
		filteredFiles = includedFiles
	} else {
		// Apply all exclude patterns
		excludedFileSet := make(map[string]bool)
		for _, pattern := range excludePatterns {
			for _, filename := range includedFiles {
				// Try filepath pattern matching
				if matched, _ := filepath.Match(pattern, filename); matched {
					excludedFileSet[filename] = true
					continue
				}

				// Try regex pattern matching
				if regex, err := regexp.Compile(pattern); err == nil && regex.MatchString(filename) {
					excludedFileSet[filename] = true
				}
			}
		}

		// Keep files that weren't excluded
		for _, filename := range includedFiles {
			if !excludedFileSet[filename] {
				filteredFiles = append(filteredFiles, filename)
			}
		}
	}

	// Convert to set for efficient lookup
	filteredFileSet := make(map[string]bool)
	for _, file := range filteredFiles {
		filteredFileSet[file] = true
	}

	// Update file selections based on regex filtering
	// We need to reset all selections and then apply the filter
	// First, reset to original selection state (log files)
	for _, file := range m.workingSet.Bundle.Files {
		originallySelected := file.IsLogFile // Could be enhanced to track user selections
		passesFilter := filteredFileSet[file.Path]

		// File is selected if it was originally selected AND passes the filter
		shouldBeSelected := originallySelected && passesFilter
		m.workingSet.SetFileSelection(file.Path, shouldBeSelected)
	}

	// Update status with more detailed info
	selectedCount := m.workingSet.GetSelectedFileCount()
	totalCount := len(filteredFiles)
	includedCount := len(includedFiles)

	statusParts := []string{
		fmt.Sprintf("%d files selected", selectedCount),
	}

	if len(includePatterns) > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d included", includedCount))
	}

	if len(excludePatterns) > 0 {
		excludedCount := includedCount - totalCount
		statusParts = append(statusParts, fmt.Sprintf("%d excluded", excludedCount))
	}

	statusParts = append(statusParts, fmt.Sprintf("%d final", totalCount))

	m.status = fmt.Sprintf("Filter: %s", strings.Join(statusParts, ", "))
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
