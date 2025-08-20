package regex

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Section represents which section is currently active
type Section int

const (
	IncludeSection Section = iota
	ExcludeSection
)

// Pattern represents a regex pattern with metadata
type Pattern struct {
	Text       string         // The regex pattern text
	Compiled   *regexp.Regexp // Compiled regex (nil if invalid)
	Valid      bool           // Whether the pattern is valid
	MatchCount int            // Number of files matching this pattern
	Error      string         // Error message if invalid
}

// Model represents the regex panel state
type Model struct {
	// Data
	includePatterns []Pattern
	excludePatterns []Pattern

	// UI State
	activeSection Section // Which section (include/exclude) is active
	cursor        int     // Which pattern is selected in current section
	editMode      bool    // Whether we're editing a pattern
	editInput     textinput.Model
	editIndex     int // Index of pattern being edited (-1 for new pattern)

	// Component state
	focused bool
	width   int
	height  int

	// Files for pattern matching
	allFiles []string // List of all filenames for testing patterns
}

// NewModel creates a new regex panel model
func NewModel() *Model {
	input := textinput.New()
	input.Placeholder = "Enter regex pattern..."
	input.CharLimit = 256

	return &Model{
		includePatterns: make([]Pattern, 0),
		excludePatterns: make([]Pattern, 0),
		activeSection:   IncludeSection,
		cursor:          0,
		editMode:        false,
		editInput:       input,
		editIndex:       -1,
		focused:         false,
		width:           40,
		height:          20,
		allFiles:        make([]string, 0),
	}
}

// Update handles messages for the regex panel
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle edit mode input
	if m.editMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				return m.confirmEdit(), nil
			case "esc":
				return m.cancelEdit(), nil
			default:
				m.editInput, cmd = m.editInput.Update(msg)
				return m, cmd
			}
		default:
			m.editInput, cmd = m.editInput.Update(msg)
			return m, cmd
		}
	}

	// Handle normal navigation
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.switchSection()
		case "up", "k":
			m.moveCursorUp()
		case "down", "j":
			m.moveCursorDown()
		case "a":
			m.startAddPattern()
		case "e":
			if m.hasPatternAtCursor() {
				m.startEditPattern()
			}
		case "d", "delete":
			m.deletePattern()
		case "enter":
			if m.hasPatternAtCursor() {
				m.startEditPattern()
			} else {
				m.startAddPattern()
			}
		case "t":
			m.testPatterns()
		}
	}

	return m, cmd
}

// View renders the regex panel
func (m *Model) View() string {
	if m.editMode {
		return m.renderEditMode()
	}
	return m.renderNormalMode()
}

// Component interface methods

func (m *Model) Focus() {
	m.focused = true
}

func (m *Model) Blur() {
	m.focused = false
	if m.editMode {
		m.cancelEdit()
	}
}

func (m *Model) IsFocused() bool {
	return m.focused
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Data management methods

func (m *Model) SetFiles(files []string) {
	m.allFiles = files
	m.testPatterns() // Retest patterns with new file list
}

func (m *Model) GetIncludePatterns() []string {
	patterns := make([]string, 0, len(m.includePatterns))
	for _, p := range m.includePatterns {
		if p.Valid {
			patterns = append(patterns, p.Text)
		}
	}
	return patterns
}

func (m *Model) GetExcludePatterns() []string {
	patterns := make([]string, 0, len(m.excludePatterns))
	for _, p := range m.excludePatterns {
		if p.Valid {
			patterns = append(patterns, p.Text)
		}
	}
	return patterns
}

// Internal methods

func (m *Model) getCurrentPatterns() *[]Pattern {
	if m.activeSection == IncludeSection {
		return &m.includePatterns
	}
	return &m.excludePatterns
}

func (m *Model) hasPatternAtCursor() bool {
	patterns := m.getCurrentPatterns()
	return m.cursor >= 0 && m.cursor < len(*patterns)
}

func (m *Model) switchSection() {
	if m.activeSection == IncludeSection {
		m.activeSection = ExcludeSection
	} else {
		m.activeSection = IncludeSection
	}
	m.cursor = 0 // Reset cursor when switching sections
}

func (m *Model) moveCursorUp() {
	patterns := m.getCurrentPatterns()
	if m.cursor > 0 {
		m.cursor--
	} else if len(*patterns) > 0 {
		m.cursor = len(*patterns) - 1 // Wrap to bottom
	}
}

func (m *Model) moveCursorDown() {
	patterns := m.getCurrentPatterns()
	if len(*patterns) == 0 {
		m.cursor = 0
		return
	}

	if m.cursor < len(*patterns)-1 {
		m.cursor++
	} else {
		m.cursor = 0 // Wrap to top
	}
}

func (m *Model) startAddPattern() {
	m.editMode = true
	m.editIndex = -1
	m.editInput.SetValue("")
	m.editInput.Focus()
}

func (m *Model) startEditPattern() {
	if !m.hasPatternAtCursor() {
		return
	}

	patterns := m.getCurrentPatterns()
	pattern := (*patterns)[m.cursor]

	m.editMode = true
	m.editIndex = m.cursor
	m.editInput.SetValue(pattern.Text)
	m.editInput.Focus()
}

func (m *Model) confirmEdit() *Model {
	value := strings.TrimSpace(m.editInput.Value())
	if value == "" {
		return m.cancelEdit()
	}

	pattern := m.compilePattern(value)
	patterns := m.getCurrentPatterns()

	if m.editIndex == -1 {
		// Adding new pattern
		*patterns = append(*patterns, pattern)
		m.cursor = len(*patterns) - 1
	} else {
		// Editing existing pattern
		(*patterns)[m.editIndex] = pattern
	}

	m.testPatterns()
	return m.cancelEdit()
}

func (m *Model) cancelEdit() *Model {
	m.editMode = false
	m.editIndex = -1
	m.editInput.Blur()
	m.editInput.SetValue("")
	return m
}

func (m *Model) deletePattern() {
	if !m.hasPatternAtCursor() {
		return
	}

	patterns := m.getCurrentPatterns()
	*patterns = append((*patterns)[:m.cursor], (*patterns)[m.cursor+1:]...)

	// Adjust cursor if needed
	if m.cursor >= len(*patterns) && len(*patterns) > 0 {
		m.cursor = len(*patterns) - 1
	} else if len(*patterns) == 0 {
		m.cursor = 0
	}

	m.testPatterns()
}

func (m *Model) compilePattern(text string) Pattern {
	compiled, err := regexp.Compile(text)
	if err != nil {
		return Pattern{
			Text:       text,
			Compiled:   nil,
			Valid:      false,
			MatchCount: 0,
			Error:      err.Error(),
		}
	}

	return Pattern{
		Text:       text,
		Compiled:   compiled,
		Valid:      true,
		MatchCount: 0,
		Error:      "",
	}
}

func (m *Model) testPatterns() {
	// Test include patterns
	for i := range m.includePatterns {
		m.includePatterns[i].MatchCount = m.countMatches(&m.includePatterns[i])
	}

	// Test exclude patterns
	for i := range m.excludePatterns {
		m.excludePatterns[i].MatchCount = m.countMatches(&m.excludePatterns[i])
	}
}

func (m *Model) countMatches(pattern *Pattern) int {
	if !pattern.Valid || pattern.Compiled == nil {
		return 0
	}

	count := 0
	for _, filename := range m.allFiles {
		if pattern.Compiled.MatchString(filename) {
			count++
		}
	}
	return count
}

// GetFilteredFiles returns files that match include patterns and don't match exclude patterns
func (m *Model) GetFilteredFiles() []string {
	if len(m.allFiles) == 0 {
		return []string{}
	}

	filtered := make([]string, 0, len(m.allFiles))

	for _, filename := range m.allFiles {
		// Check include patterns (if any)
		includeMatch := len(m.includePatterns) == 0 // Default to true if no include patterns
		for _, pattern := range m.includePatterns {
			if pattern.Valid && pattern.Compiled != nil && pattern.Compiled.MatchString(filename) {
				includeMatch = true
				break
			}
		}

		if !includeMatch {
			continue
		}

		// Check exclude patterns
		excludeMatch := false
		for _, pattern := range m.excludePatterns {
			if pattern.Valid && pattern.Compiled != nil && pattern.Compiled.MatchString(filename) {
				excludeMatch = true
				break
			}
		}

		if !excludeMatch {
			filtered = append(filtered, filename)
		}
	}

	return filtered
}
