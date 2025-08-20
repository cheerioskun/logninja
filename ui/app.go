package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/ui/regex"
)

// FocusedPanel represents which panel is currently focused
type FocusedPanel int

const (
	FileTreePanel FocusedPanel = iota
	IncludePanel
	ExcludePanel
	HistogramPanel
	TimeRangePanel
	StatusPanel
)

// AppModel represents the main application model
type AppModel struct {
	// Core state
	workingSet *models.WorkingSet

	// UI Components
	includePanel *regex.SingleModel
	excludePanel *regex.SingleModel

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

	if workingSet != nil && workingSet.Bundle != nil {
		var filePaths []string
		for _, file := range workingSet.Bundle.Files {
			filePaths = append(filePaths, file.Path)
		}
		includePanel.SetFiles(filePaths)
		excludePanel.SetFiles(filePaths)
	}

	return &AppModel{
		workingSet:   workingSet,
		includePanel: includePanel,
		excludePanel: excludePanel,
		focused:      FileTreePanel,
		width:        80,
		height:       24,
		panels:       []FocusedPanel{FileTreePanel, IncludePanel, ExcludePanel, HistogramPanel, TimeRangePanel},
		currentPanel: 0,
		status:       "Ready",
		ready:        true,
		quitting:     false,
	}
}

// Init implements tea.Model
func (m *AppModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

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
			// Forward message to focused panel
			if m.focused == IncludePanel {
				var cmd tea.Cmd
				m.includePanel, cmd = m.includePanel.Update(msg)
				m.updateWorkingSetFromRegex()
				return m, cmd
			} else if m.focused == ExcludePanel {
				var cmd tea.Cmd
				m.excludePanel, cmd = m.excludePanel.Update(msg)
				m.updateWorkingSetFromRegex()
				return m, cmd
			}
		}
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

	// Split content area - 3 panels on top, 2 on bottom
	topPanelWidth := m.width / 3
	bottomPanelWidth := m.width / 2
	topHeight := contentHeight * 2 / 3
	bottomHeight := contentHeight - topHeight

	// Create header
	header := m.renderHeader()

	// Create panel contents
	fileTree := m.renderFileTreePanel(topPanelWidth, topHeight)
	include := m.renderIncludePanel(topPanelWidth, topHeight)
	exclude := m.renderExcludePanel(topPanelWidth, topHeight)
	histogram := m.renderHistogramPanel(bottomPanelWidth, bottomHeight)
	timeRange := m.renderTimeRangePanel(bottomPanelWidth, bottomHeight)

	// Create status
	status := m.renderStatusPanel(m.width, statusHeight)

	// Combine panels
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, fileTree, include, exclude)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, histogram, timeRange)
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

// updateWorkingSetFromRegex syncs regex patterns from the components to the working set
func (m *AppModel) updateWorkingSetFromRegex() {
	if m.workingSet == nil {
		return
	}

	// Get patterns from separate panels
	includePatterns := m.includePanel.GetPatterns()
	excludePatterns := m.excludePanel.GetPatterns()

	// Update working set
	m.workingSet.IncludeRegex = includePatterns
	m.workingSet.ExcludeRegex = excludePatterns

	// Apply filename filtering to update file selections
	m.applyRegexFiltering()
}

// applyRegexFiltering applies regex patterns to filter file selections
func (m *AppModel) applyRegexFiltering() {
	if m.workingSet == nil || m.workingSet.Bundle == nil {
		return
	}

	// Apply filtering logic manually
	includePatterns := m.workingSet.IncludeRegex
	excludePatterns := m.workingSet.ExcludeRegex

	var filteredFiles []string

	for _, file := range m.workingSet.Bundle.Files {
		filename := file.Path

		// Check include patterns (if any)
		includeMatch := len(includePatterns) == 0 // Default to true if no include patterns
		for _, pattern := range includePatterns {
			if matched, _ := filepath.Match(pattern, filename); matched {
				includeMatch = true
				break
			}
			// Also try as regex
			if regex, err := regexp.Compile(pattern); err == nil && regex.MatchString(filename) {
				includeMatch = true
				break
			}
		}

		if !includeMatch {
			continue
		}

		// Check exclude patterns
		excludeMatch := false
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, filename); matched {
				excludeMatch = true
				break
			}
			// Also try as regex
			if regex, err := regexp.Compile(pattern); err == nil && regex.MatchString(filename) {
				excludeMatch = true
				break
			}
		}

		if !excludeMatch {
			filteredFiles = append(filteredFiles, filename)
		}
	}

	// Convert to set for efficient lookup
	filteredFileSet := make(map[string]bool)
	for _, file := range filteredFiles {
		filteredFileSet[file] = true
	}

	// Update file selections based on regex filtering
	for _, file := range m.workingSet.Bundle.Files {
		// Only keep selected if file passes regex filter
		if m.workingSet.IsFileSelected(file.Path) {
			m.workingSet.SetFileSelection(file.Path, filteredFileSet[file.Path])
		}
	}

	// Update status
	selectedCount := m.workingSet.GetSelectedFileCount()
	totalCount := len(filteredFiles)
	m.status = fmt.Sprintf("Regex filter: %d/%d files match", selectedCount, totalCount)
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
