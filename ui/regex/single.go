package regex

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PatternType represents whether this is for include or exclude patterns
type PatternType int

const (
	IncludeType PatternType = iota
	ExcludeType
)

// SingleModel represents a single-purpose pattern panel (either include or exclude)
type SingleModel struct {
	// Config
	patternType PatternType
	title       string

	// Data
	patterns []Pattern

	// UI State
	cursor    int
	editMode  bool
	editInput textinput.Model
	editIndex int // Index of pattern being edited (-1 for new pattern)

	// Component state
	focused bool
	width   int
	height  int

	// Files for pattern matching
	allFiles []string
}

// NewSingleModel creates a new single-purpose regex model
func NewSingleModel(patternType PatternType) *SingleModel {
	var title string
	switch patternType {
	case IncludeType:
		title = "📥 Include Patterns"
	case ExcludeType:
		title = "📤 Exclude Patterns"
	}

	input := textinput.New()
	input.Placeholder = "Enter regex pattern..."
	input.CharLimit = 256

	return &SingleModel{
		patternType: patternType,
		title:       title,
		patterns:    make([]Pattern, 0),
		cursor:      0,
		editMode:    false,
		editInput:   input,
		editIndex:   -1,
		focused:     false,
		width:       40,
		height:      20,
		allFiles:    make([]string, 0),
	}
}

// Update handles messages for the single pattern panel
func (m *SingleModel) Update(msg tea.Msg) (*SingleModel, tea.Cmd) {
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

// View renders the single pattern panel
func (m *SingleModel) View() string {
	if m.editMode {
		return m.renderEditMode()
	}
	return m.renderNormalMode()
}

// Component interface methods

func (m *SingleModel) Focus() {
	m.focused = true
}

func (m *SingleModel) Blur() {
	m.focused = false
	if m.editMode {
		m.cancelEdit()
	}
}

func (m *SingleModel) IsFocused() bool {
	return m.focused
}

func (m *SingleModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Data management methods

func (m *SingleModel) SetFiles(files []string) {
	m.allFiles = files
	m.testPatterns() // Retest patterns with new file list
}

func (m *SingleModel) GetPatterns() []string {
	patterns := make([]string, 0, len(m.patterns))
	for _, p := range m.patterns {
		if p.Valid {
			patterns = append(patterns, p.Text)
		}
	}
	return patterns
}

// Rendering methods

func (m *SingleModel) renderNormalMode() string {
	// Header
	var titleColor lipgloss.Color
	if m.patternType == IncludeType {
		titleColor = successColor
	} else {
		titleColor = errorColor
	}

	header := headerStyle.
		Foreground(titleColor).
		Render(m.title)

	if m.focused {
		header = headerStyle.
			Foreground(titleColor).
			Background(lipgloss.Color("235")).
			Render(m.title + " *")
	}

	// Content
	content := m.renderPatterns()

	// Help
	help := ""
	if m.focused {
		helpItems := []string{
			"↑/↓: Navigate",
			"a: Add",
			"e/Enter: Edit",
			"d: Delete",
			"t: Test",
		}
		help = helpStyle.Render(strings.Join(helpItems, " • "))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, help)
}

func (m *SingleModel) renderEditMode() string {
	title := "Edit Pattern"
	if m.editIndex == -1 {
		title = "Add Pattern"
	}

	header := headerStyle.
		Foreground(primaryColor).
		Render(title)

	input := editInputStyle.Render(m.editInput.View())

	editHelp := helpStyle.Render("Enter: Confirm • Esc: Cancel")

	return lipgloss.JoinVertical(lipgloss.Left, header, input, editHelp)
}

func (m *SingleModel) renderPatterns() string {
	if len(m.patterns) == 0 {
		emptyMsg := "No patterns"
		if m.focused {
			emptyMsg += " (press 'a' to add)"
		}
		return lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true).
			Render(emptyMsg)
	}

	var lines []string
	maxHeight := m.height - 6 // Account for title and help

	// Calculate visible range
	visibleStart := 0
	visibleEnd := len(m.patterns)

	if maxHeight > 0 && len(m.patterns) > maxHeight {
		if m.focused {
			if m.cursor >= maxHeight {
				visibleStart = m.cursor - maxHeight + 1
			}
			visibleEnd = visibleStart + maxHeight
			if visibleEnd > len(m.patterns) {
				visibleEnd = len(m.patterns)
				visibleStart = visibleEnd - maxHeight
				if visibleStart < 0 {
					visibleStart = 0
				}
			}
		} else {
			visibleEnd = maxHeight
		}
	}

	// Add scroll indicator if needed
	if visibleStart > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(secondaryColor).Render("↑ ..."))
	}

	// Render visible patterns
	for i := visibleStart; i < visibleEnd; i++ {
		pattern := m.patterns[i]
		line := m.renderPattern(pattern, i, m.focused && i == m.cursor)
		lines = append(lines, line)
	}

	// Add scroll indicator if needed
	if visibleEnd < len(m.patterns) {
		lines = append(lines, lipgloss.NewStyle().Foreground(secondaryColor).Render("↓ ..."))
	}

	return strings.Join(lines, "\n")
}

func (m *SingleModel) renderPattern(pattern Pattern, index int, isSelected bool) string {
	// Pattern text with validation indicator
	var statusIcon string
	var textColor lipgloss.Color

	if pattern.Valid {
		statusIcon = "✓"
		textColor = successColor
	} else {
		statusIcon = "✗"
		textColor = errorColor
	}

	// Format pattern text
	patternText := pattern.Text
	if len(patternText) > 25 {
		patternText = patternText[:22] + "..."
	}

	// Match count
	matchInfo := ""
	if pattern.Valid {
		matchInfo = fmt.Sprintf(" (%d)", pattern.MatchCount)
	} else {
		matchInfo = " (error)"
	}

	content := fmt.Sprintf("%s %s%s", statusIcon, patternText, matchInfo)

	// Apply styling
	if isSelected {
		return selectedPatternStyle.Render(content)
	}

	return patternStyle.
		Foreground(textColor).
		Render(content)
}

// Internal methods (same logic as the full regex model)

func (m *SingleModel) hasPatternAtCursor() bool {
	return m.cursor >= 0 && m.cursor < len(m.patterns)
}

func (m *SingleModel) moveCursorUp() {
	if m.cursor > 0 {
		m.cursor--
	} else if len(m.patterns) > 0 {
		m.cursor = len(m.patterns) - 1
	}
}

func (m *SingleModel) moveCursorDown() {
	if len(m.patterns) == 0 {
		m.cursor = 0
		return
	}

	if m.cursor < len(m.patterns)-1 {
		m.cursor++
	} else {
		m.cursor = 0
	}
}

func (m *SingleModel) startAddPattern() {
	m.editMode = true
	m.editIndex = -1
	m.editInput.SetValue("")
	m.editInput.Focus()
}

func (m *SingleModel) startEditPattern() {
	if !m.hasPatternAtCursor() {
		return
	}

	pattern := m.patterns[m.cursor]
	m.editMode = true
	m.editIndex = m.cursor
	m.editInput.SetValue(pattern.Text)
	m.editInput.Focus()
}

func (m *SingleModel) confirmEdit() *SingleModel {
	value := strings.TrimSpace(m.editInput.Value())
	if value == "" {
		return m.cancelEdit()
	}

	pattern := m.compilePattern(value)

	if m.editIndex == -1 {
		// Adding new pattern
		m.patterns = append(m.patterns, pattern)
		m.cursor = len(m.patterns) - 1
	} else {
		// Editing existing pattern
		m.patterns[m.editIndex] = pattern
	}

	m.testPatterns()
	return m.cancelEdit()
}

func (m *SingleModel) cancelEdit() *SingleModel {
	m.editMode = false
	m.editIndex = -1
	m.editInput.Blur()
	m.editInput.SetValue("")
	return m
}

func (m *SingleModel) deletePattern() {
	if !m.hasPatternAtCursor() {
		return
	}

	m.patterns = append(m.patterns[:m.cursor], m.patterns[m.cursor+1:]...)

	// Adjust cursor if needed
	if m.cursor >= len(m.patterns) && len(m.patterns) > 0 {
		m.cursor = len(m.patterns) - 1
	} else if len(m.patterns) == 0 {
		m.cursor = 0
	}

	m.testPatterns()
}

func (m *SingleModel) compilePattern(text string) Pattern {
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

func (m *SingleModel) testPatterns() {
	for i := range m.patterns {
		m.patterns[i].MatchCount = m.countMatches(&m.patterns[i])
	}
}

func (m *SingleModel) countMatches(pattern *Pattern) int {
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
