package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheerioskun/logninja/internal/export"
	"github.com/cheerioskun/logninja/internal/messages"
	"github.com/cheerioskun/logninja/internal/models"
	exportui "github.com/cheerioskun/logninja/ui/export"
	"github.com/cheerioskun/logninja/ui/filelist"
	"github.com/cheerioskun/logninja/ui/regex"
	"github.com/spf13/afero"
)

// FocusedPanel represents which panel is currently focused
type FocusedPanel int

const (
	RegexPanel FocusedPanel = iota
	FileListPanel
	StatusPanel
)

// AppModel represents the main application model
type AppModel struct {
	// Core state
	workingSet *models.WorkingSet

	// Services
	exportService *export.Service

	// UI Components
	regexPanel    *regex.Model
	fileListPanel *filelist.Model
	exportModal   *exportui.Model

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
	// Create services
	exportService := export.NewService(fs)

	// Create the two working panels
	regexPanel := regex.NewModel()
	fileListPanel := filelist.NewModel()
	exportModal := exportui.NewModel(exportService)

	if workingSet != nil && workingSet.Bundle != nil {
		var filePaths []string
		for _, file := range workingSet.Bundle.Files {
			filePaths = append(filePaths, file.Path)
		}
		regexPanel.SetFiles(filePaths)
	}

	return &AppModel{
		workingSet:    workingSet,
		exportService: exportService,
		regexPanel:    regexPanel,
		fileListPanel: fileListPanel,
		exportModal:   exportModal,
		focused:       RegexPanel,
		width:         80,
		height:        24,
		panels:        []FocusedPanel{RegexPanel, FileListPanel},
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
		m.exportModal.SetSize(msg.Width, msg.Height)
		return m, nil

	case messages.RegexFiltersChangedMsg:
		// Handle ordered regex filter changes
		return m, m.handleRegexFiltersChange(msg)

	case messages.WorkingSetUpdatedMsg:
		// Handle working set updates
		m.status = fmt.Sprintf("Working set updated: %d files selected", msg.SelectedCount)
		return m, nil

	case filelist.FileListDataMsg:
		// Forward file list data to the file list component
		var cmd tea.Cmd
		m.fileListPanel, cmd = m.fileListPanel.Update(msg)
		return m, cmd

	case exportui.ExportModalConfirmedMsg:
		m.status = "Export started..."
		return m, nil

	case exportui.ExportModalCancelledMsg:
		m.status = "Export cancelled"
		return m, nil

	case exportui.ExportModalCompletedMsg:
		// Handle completion message for status updates
		if msg.Success {
			fileCount := 0
			if msg.Summary != nil {
				fileCount = msg.Summary.FileCount
			}
			m.status = fmt.Sprintf("Export completed: %d files exported", fileCount)
		} else {
			m.status = fmt.Sprintf("Export failed: %v", msg.Error)
		}
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
			m.status = "Help: Tab/Shift+Tab to navigate, Shift+E to export, q to quit"
			return m, nil

		case "E":
			// Show export modal (Shift+E)
			if m.workingSet != nil {
				cmd := m.exportModal.Show(m.workingSet)
				m.status = "Export modal opened"
				return m, cmd
			}
			m.status = "No working set available for export"
			return m, nil

		default:
			// Forward message to focused panel and collect commands only if modal is not visible
			if !m.exportModal.IsVisible() {
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
				}
			}
		}
	}

	// Update export modal for all messages when visible, or for non-keyboard messages when hidden
	if m.exportModal.IsVisible() || !isKeyboardMsg(msg) {
		var modalCmd tea.Cmd
		m.exportModal, modalCmd = m.exportModal.Update(msg)
		if modalCmd != nil {
			cmds = append(cmds, modalCmd)
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

	// Render export modal overlay if visible
	if m.exportModal.IsVisible() {
		modalView := m.exportModal.View()
		if modalView != "" {
			// Replace content with modal (modal handles its own positioning)
			content = modalView
		}
	}

	return content
}

// renderLayout creates the main application layout
func (m *AppModel) renderLayout() string {
	// Calculate dimensions for panels
	headerHeight := 3
	statusHeight := 3
	contentHeight := m.height - headerHeight - statusHeight

	// Split content area into two equal panels side by side
	panelWidth := m.width / 2

	// Create header
	header := m.renderHeader()

	// Create the two working panels
	regexPanel := m.renderRegexPanel(panelWidth, contentHeight)
	fileListPanel := m.renderFileListPanel(panelWidth, contentHeight)

	// Create status
	status := m.renderStatusPanel(m.width, statusHeight)

	// Combine panels side by side
	content := lipgloss.JoinHorizontal(lipgloss.Top, regexPanel, fileListPanel)

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
			fmt.Sprintf("Size: %s", formatBytes(m.workingSet.GetSelectedTotalSize())),
			fmt.Sprintf("Status: %s", m.status),
		)
	}

	return style.Render(strings.Join(statusParts, " | "))
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

// isKeyboardMsg checks if a message is a keyboard message
func isKeyboardMsg(msg tea.Msg) bool {
	_, ok := msg.(tea.KeyMsg)
	return ok
}
