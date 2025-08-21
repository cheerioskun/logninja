package export

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheerioskun/logninja/internal/export"
	"github.com/cheerioskun/logninja/internal/models"
)

// Styling
var (
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Align(lipgloss.Center)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			Margin(1, 0)

	previewStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Margin(1, 0)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Align(lipgloss.Center).
			Margin(1, 0)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Margin(1, 0)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true).
			Margin(1, 0)
)

// State represents the modal's current state
type State int

const (
	StateInput State = iota
	StateExporting
	StateSuccess
	StateError
)

// Model represents the export modal
type Model struct {
	// UI components
	textInput textinput.Model

	// State
	state   State
	visible bool
	width   int
	height  int

	// Data
	workingSet     *models.WorkingSet
	exportService  *export.Service
	exportSummary  *export.ExportSummary
	errorMessage   string
	successMessage string
}

// ExportModalConfirmedMsg is sent when user confirms export
type ExportModalConfirmedMsg struct {
	DestinationPath string
}

// ExportModalCancelledMsg is sent when user cancels export
type ExportModalCancelledMsg struct{}

// ExportModalCompletedMsg is sent when export operation completes
type ExportModalCompletedMsg struct {
	Success bool
	Error   error
	Summary *export.ExportSummary
}

// NewModel creates a new export modal
func NewModel(exportService *export.Service) *Model {
	ti := textinput.New()
	ti.Placeholder = "Enter export destination..."
	ti.CharLimit = 256
	ti.Width = 50

	return &Model{
		textInput:     ti,
		state:         StateInput,
		visible:       false,
		exportService: exportService,
	}
}

// Show displays the modal with the given working set
func (m *Model) Show(ws *models.WorkingSet) tea.Cmd {
	m.visible = true
	m.state = StateInput
	m.workingSet = ws
	m.errorMessage = ""
	m.successMessage = ""

	defaultPath, err := export.GetDefaultExportPath(ws.Bundle.Path)
	if err != nil {
		defaultPath = "./export_refined"
	}

	m.textInput.SetValue(defaultPath)
	m.textInput.Focus()

	// Calculate export summary
	return m.updateSummary()
}

// Hide hides the modal
func (m *Model) Hide() {
	m.visible = false
	m.textInput.Blur()
	m.state = StateInput
}

// IsVisible returns true if the modal is visible
func (m *Model) IsVisible() bool {
	return m.visible
}

// SetSize sets the modal size
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the export modal
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case StateInput:
			switch msg.String() {
			case "enter":
				return m.confirmExport()
			case "esc":
				m.Hide()
				return m, func() tea.Msg { return ExportModalCancelledMsg{} }
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				cmds = append(cmds, cmd)

				// Update summary when path changes
				cmds = append(cmds, m.updateSummary())
			}
		case StateExporting:
			// Don't handle input while exporting
			return m, nil
		case StateSuccess, StateError:
			// Any key closes the modal after success/error
			wasSuccess := m.state == StateSuccess
			m.Hide()
			if wasSuccess {
				return m, func() tea.Msg {
					return ExportModalCompletedMsg{
						Success: true,
						Summary: m.exportSummary,
						Error:   nil,
					}
				}
			} else {
				return m, func() tea.Msg { return ExportModalCancelledMsg{} }
			}
		}

	case exportSyncMsg:
		// Perform the export synchronously and update state directly
		if msg.modal.workingSet == nil {
			m.state = StateError
			m.errorMessage = "Working set is nil"
			return m, nil
		}

		if msg.modal.exportSummary == nil || msg.modal.exportSummary.FileCount == 0 {
			m.state = StateError
			m.errorMessage = "No files to export"
			return m, nil
		}

		opts := export.ExportOptions{
			DestinationPath:   msg.destPath,
			PreserveStructure: true,
			Overwrite:         true,
		}

		err := msg.modal.exportService.ExportWorkingSet(msg.modal.workingSet, opts)

		if err == nil {
			m.state = StateSuccess
			m.successMessage = fmt.Sprintf("Successfully exported %d files to %s",
				msg.modal.exportSummary.FileCount, msg.destPath)
		} else {
			m.state = StateError
			m.errorMessage = fmt.Sprintf("Export failed: %v", err)
		}
		return m, nil

	case ExportModalCompletedMsg:
		if msg.Success {
			m.state = StateSuccess
			m.successMessage = fmt.Sprintf("Successfully exported %d files to %s",
				msg.Summary.FileCount, msg.Summary.DestinationPath)
		} else {
			m.state = StateError
			m.errorMessage = fmt.Sprintf("Export failed: %v", msg.Error)
		}
		return m, nil

	default:
		if m.state == StateInput {
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the export modal
func (m *Model) View() string {
	if !m.visible {
		return ""
	}

	var content string

	switch m.state {
	case StateInput:
		content = m.renderInputState()
	case StateExporting:
		content = m.renderExportingState()
	case StateSuccess:
		content = m.renderSuccessState()
	case StateError:
		content = m.renderErrorState()
	}

	// Center the modal on screen
	modalWidth := 60
	modalHeight := lipgloss.Height(content) + 4 // Add padding

	// Calculate position to center the modal
	x := (m.width - modalWidth) / 2
	y := (m.height - modalHeight) / 2

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	styledContent := modalStyle.
		Width(modalWidth).
		Render(content)

	// Position the modal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styledContent)
}

// renderInputState renders the input state of the modal
func (m *Model) renderInputState() string {
	var parts []string

	// Title
	parts = append(parts, titleStyle.Render("Export Working Set"))

	// Preview information
	if m.exportSummary != nil {
		preview := fmt.Sprintf("Files to export: %d\nTotal size: %s",
			m.exportSummary.FileCount, formatBytes(m.exportSummary.TotalSize))
		parts = append(parts, previewStyle.Render(preview))
	}

	// Input
	parts = append(parts, "Destination Path:")
	parts = append(parts, inputStyle.Render(m.textInput.View()))

	// Error message if any
	if m.errorMessage != "" {
		parts = append(parts, errorStyle.Render(m.errorMessage))
	}

	// Help
	parts = append(parts, helpStyle.Render("Enter: Export • Esc: Cancel"))

	return strings.Join(parts, "\n")
}

// renderExportingState renders the exporting state
func (m *Model) renderExportingState() string {
	var parts []string

	parts = append(parts, titleStyle.Render("Exporting..."))
	parts = append(parts, previewStyle.Render("Please wait while files are being exported..."))

	return strings.Join(parts, "\n")
}

// renderSuccessState renders the success state
func (m *Model) renderSuccessState() string {
	var parts []string

	parts = append(parts, titleStyle.Render("Export Complete"))
	parts = append(parts, successStyle.Render(m.successMessage))
	parts = append(parts, helpStyle.Render("Press any key to close"))

	return strings.Join(parts, "\n")
}

// renderErrorState renders the error state
func (m *Model) renderErrorState() string {
	var parts []string

	parts = append(parts, titleStyle.Render("Export Failed"))
	parts = append(parts, errorStyle.Render(m.errorMessage))
	parts = append(parts, helpStyle.Render("Press any key to close"))

	return strings.Join(parts, "\n")
}

// confirmExport starts the export process
func (m *Model) confirmExport() (*Model, tea.Cmd) {
	destPath := strings.TrimSpace(m.textInput.Value())

	// Validate path
	if err := export.ValidateExportPath(destPath); err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}

	// Clear error and start export
	m.errorMessage = ""
	m.state = StateExporting

	// Return command to perform export with direct state update
	return m, m.performExportSync(destPath)
}

// updateSummary updates the export summary
func (m *Model) updateSummary() tea.Cmd {
	if m.workingSet == nil {
		return nil
	}

	// Update summary synchronously instead of via command
	destPath := strings.TrimSpace(m.textInput.Value())
	if destPath != "" {
		summary, err := m.exportService.GetExportSummary(m.workingSet, destPath)
		if err == nil {
			m.exportSummary = summary
		}
	}

	return nil
}

// performExportSync performs the export operation and updates modal state directly
func (m *Model) performExportSync(destPath string) tea.Cmd {
	return func() tea.Msg {
		// Create a custom message that includes the modal pointer for direct state update
		return exportSyncMsg{
			modal:    m,
			destPath: destPath,
		}
	}
}

// exportSyncMsg is used to trigger synchronous export with direct modal update
type exportSyncMsg struct {
	modal    *Model
	destPath string
}

// performExport performs the actual export operation (kept for compatibility)
func (m *Model) performExport(destPath string) tea.Cmd {
	return func() tea.Msg {
		// Debug: Check if we have valid data
		if m.workingSet == nil {
			return ExportModalCompletedMsg{
				Success: false,
				Error:   fmt.Errorf("working set is nil"),
				Summary: m.exportSummary,
			}
		}

		if m.exportSummary == nil || m.exportSummary.FileCount == 0 {
			return ExportModalCompletedMsg{
				Success: false,
				Error:   fmt.Errorf("no files to export"),
				Summary: m.exportSummary,
			}
		}

		opts := export.ExportOptions{
			DestinationPath:   destPath,
			PreserveStructure: true,
			Overwrite:         true,
		}

		err := m.exportService.ExportWorkingSet(m.workingSet, opts)

		return ExportModalCompletedMsg{
			Success: err == nil,
			Error:   err,
			Summary: m.exportSummary,
		}
	}
}

// formatBytes formats byte count as human readable string
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
