package histogram

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cheerioskun/logninja/internal/models"
	"github.com/cheerioskun/logninja/internal/parser"
	"github.com/spf13/afero"
)

// Model represents the histogram panel state
type Model struct {
	// Data
	volumeData []models.VolumePoint
	workingSet *models.WorkingSet
	binCount   int
	loading    bool
	lastUpdate time.Time

	// Volume analyzer
	volumeAnalyzer *parser.VolumeAnalyzer

	// UI state
	focused bool
	width   int
	height  int

	// Display options
	showFiles   bool // Show file count per bin
	maxBarWidth int  // Maximum width for histogram bars

	// Status
	status string
	error  string
}

// NewModel creates a new histogram model
func NewModel(fs afero.Fs) *Model {
	return &Model{
		volumeData:     make([]models.VolumePoint, 0),
		binCount:       20, // Default bin count
		loading:        false,
		volumeAnalyzer: parser.NewVolumeAnalyzer(fs),
		focused:        false,
		width:          40,
		height:         20,
		showFiles:      false,
		maxBarWidth:    30,
		status:         "Ready",
	}
}

// Update handles messages for the histogram panel
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "r":
			// Refresh histogram
			return m, m.generateHistogram()
		case "f":
			// Toggle file count display
			m.showFiles = !m.showFiles
			return m, nil
		case "+", "=":
			// Increase bin count
			if m.binCount < 50 {
				m.binCount += 5
				return m, m.generateHistogram()
			}
		case "-", "_":
			// Decrease bin count
			if m.binCount > 5 {
				m.binCount -= 5
				return m, m.generateHistogram()
			}
		}

	case HistogramDataMsg:
		// Received new histogram data
		m.volumeData = msg.VolumeData
		m.loading = false
		m.lastUpdate = time.Now()
		m.status = fmt.Sprintf("Updated at %s (%d bins)",
			m.lastUpdate.Format("15:04:05"), len(m.volumeData))
		m.error = ""
		return m, nil

	case HistogramErrorMsg:
		// Error generating histogram
		m.loading = false
		m.error = msg.Error
		m.status = "Error generating histogram"
		return m, nil

	case HistogramLoadingMsg:
		// Started generating histogram
		m.loading = true
		m.status = "Generating histogram..."
		m.error = ""
		return m, nil

	case WorkingSetUpdatedMsg:
		// Working set changed, update our reference
		m.workingSet = msg.WorkingSet
		return m, m.generateHistogram() // Auto-regenerate on working set changes
	}

	return m, nil
}

// View renders the histogram panel
func (m *Model) View() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.error != "" {
		return m.renderError()
	}

	if len(m.volumeData) == 0 {
		return m.renderEmpty()
	}

	return m.renderHistogram()
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
	// Adjust max bar width based on available space
	m.maxBarWidth = width - 20 // Leave space for labels and borders
	if m.maxBarWidth < 10 {
		m.maxBarWidth = 10
	}
}

// Data management methods

func (m *Model) SetWorkingSet(workingSet *models.WorkingSet) {
	m.workingSet = workingSet
}

func (m *Model) GetVolumeData() []models.VolumePoint {
	return m.volumeData
}

// generateHistogram creates a command to generate histogram data
func (m *Model) generateHistogram() tea.Cmd {
	if m.workingSet == nil {
		return nil
	}

	return tea.Batch(
		func() tea.Msg { return HistogramLoadingMsg{} },
		func() tea.Msg {
			volumeData, err := m.volumeAnalyzer.GenerateHistogram(m.workingSet, m.binCount)
			if err != nil {
				return HistogramErrorMsg{Error: err.Error()}
			}
			return HistogramDataMsg{VolumeData: volumeData}
		},
	)
}

// Messages for histogram component

// HistogramDataMsg contains new histogram data
type HistogramDataMsg struct {
	VolumeData []models.VolumePoint
}

// HistogramErrorMsg contains histogram generation error
type HistogramErrorMsg struct {
	Error string
}

// HistogramLoadingMsg indicates histogram generation started
type HistogramLoadingMsg struct{}

// WorkingSetUpdatedMsg indicates the working set changed
type WorkingSetUpdatedMsg struct {
	WorkingSet *models.WorkingSet
}
