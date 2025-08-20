package filelist

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheerioskun/logninja/internal/messages"
	"github.com/cheerioskun/logninja/internal/models"
)

// Model represents the file list component sorted by size
type Model struct {
	// Data
	files      []models.FileInfo
	totalSize  int64
	totalFiles int

	// UI state
	focused  bool
	width    int
	height   int
	viewport viewport.Model

	// Styles
	titleStyle lipgloss.Style
	fileStyle  lipgloss.Style
	sizeStyle  lipgloss.Style
	emptyStyle lipgloss.Style
}

// NewModel creates a new file list model
func NewModel() *Model {
	vp := viewport.New(40, 6) // Initial size, will be updated in SetSize
	vp.SetContent("")

	return &Model{
		files:      make([]models.FileInfo, 0),
		totalSize:  0,
		totalFiles: 0,
		focused:    false,
		width:      40,
		height:     10,
		viewport:   vp,

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Margin(0, 0, 1, 0),

		fileStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")),

		sizeStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Align(lipgloss.Right),

		emptyStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true),
	}
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case messages.WorkingSetUpdatedMsg:
		// This component listens for working set updates
		// We don't have direct access to working set here, so we'll get the data through the message
		// For now, we'll update when we get the data from the parent
		return m, nil

	case FileListDataMsg:
		// Custom message with file data
		m.files = msg.Files
		m.totalSize = msg.TotalSize
		m.totalFiles = msg.TotalFiles

		// Update viewport content
		m.updateViewportContent()
		return m, nil

	case tea.KeyMsg:
		if m.focused {
			switch msg.String() {
			case "j", "down":
				m.viewport.LineDown(1)
			case "k", "up":
				m.viewport.LineUp(1)
			case "pgdown", " ":
				m.viewport.ViewDown()
			case "pgup":
				m.viewport.ViewUp()
			case "home", "g":
				m.viewport.GotoTop()
			case "end", "G":
				m.viewport.GotoBottom()
			}
		}
	}

	// Always update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the component
func (m *Model) View() string {
	// Title
	title := "📁 File List (by Size)"
	if m.focused {
		title += " *"
	}

	header := m.titleStyle.Render(title)

	// Content using viewport
	var content string
	if len(m.files) == 0 {
		content = m.emptyStyle.Render("No files selected")
	} else {
		content = m.viewport.View()
	}

	// Summary
	summary := m.renderSummary()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, summary)
}

// updateViewportContent updates the viewport with the current file list content
func (m *Model) updateViewportContent() {
	if len(m.files) == 0 {
		m.viewport.SetContent(m.emptyStyle.Render("No files to display"))
		return
	}

	content := m.renderFileListContent()
	m.viewport.SetContent(content)
}

// renderFileListContent generates the content for the viewport
func (m *Model) renderFileListContent() string {
	var lines []string

	for i, file := range m.files {
		// Format filename (truncate if too long to fit in viewport)
		filename := filepath.Base(file.Path)
		maxFilenameWidth := m.width - 30 // Leave space for size and percentage
		if maxFilenameWidth < 10 {
			maxFilenameWidth = 10
		}
		if len(filename) > maxFilenameWidth {
			filename = filename[:maxFilenameWidth-3] + "..."
		}

		// Format size
		sizeStr := formatBytes(file.Size)

		// Calculate percentage of total working set size
		percentage := float64(file.Size) / float64(m.totalSize) * 100

		// Format line without progress bars, just percentage
		line := fmt.Sprintf("%d. %-*s %8s %5.1f%%",
			i+1,
			maxFilenameWidth,
			filename,
			sizeStr,
			percentage,
		)

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderSummary renders the summary information
func (m *Model) renderSummary() string {
	if m.totalFiles == 0 {
		return ""
	}

	// Add scroll position indicator
	scrollInfo := ""
	if len(m.files) > 0 && m.viewport.Height > 0 {
		scrollInfo = fmt.Sprintf(" • %d/%d", m.viewport.YOffset+1, len(m.files))
	}

	summary := fmt.Sprintf("Total: %s • %d files%s",
		formatBytes(m.totalSize),
		m.totalFiles,
		scrollInfo,
	)

	return m.sizeStyle.Render(summary)
}

// Component interface methods

func (m *Model) Focus() {
	m.focused = true
}

func (m *Model) Blur() {
	m.focused = false
}

func (m *Model) IsFocused() bool {
	return m.focused
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Update viewport size
	// Account for title (2 lines), summary (1 line), and some padding
	viewportHeight := height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	m.viewport.Width = width
	m.viewport.Height = viewportHeight

	// Update content after resize in case formatting needs to change
	if len(m.files) > 0 {
		m.updateViewportContent()
	}
}

// FileListDataMsg is a custom message containing file data for this component
type FileListDataMsg struct {
	Files      []models.FileInfo
	TotalSize  int64
	TotalFiles int
}

// formatBytes formats byte counts in human-readable format
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
