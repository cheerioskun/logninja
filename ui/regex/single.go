package regex

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cheerioskun/logninja/internal/messages"
	"github.com/cheerioskun/logninja/internal/models"
)

// Styling constants
var (
	// Colors
	primaryColor   = lipgloss.Color("205")
	secondaryColor = lipgloss.Color("240")
	successColor   = lipgloss.Color("46")
	errorColor     = lipgloss.Color("196")
	warningColor   = lipgloss.Color("214")

	// Base styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true).
			Padding(0, 1)

	editInputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Margin(0, 0, 1, 0)

	selectedPatternStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	patternStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

// Model represents a unified regex pattern panel for both include and exclude patterns
type Model struct {
	// Data
	patterns []Pattern // Ordered list of patterns (includes and excludes mixed)

	// UI State
	cursor         int
	editMode       bool
	editInput      textinput.Model
	editIndex      int         // Index of pattern being edited (-1 for new pattern)
	newPatternType PatternType // Type for the next pattern to be added

	// Component state
	focused bool
	width   int
	height  int

	// Files for pattern matching
	allFiles []string
}

// NewModel creates a new unified regex model
func NewModel() *Model {
	input := textinput.New()
	input.Placeholder = "Enter regex pattern..."
	input.CharLimit = 256

	return &Model{
		patterns:       make([]Pattern, 0),
		cursor:         0,
		editMode:       false,
		editInput:      input,
		editIndex:      -1,
		newPatternType: IncludeType, // Default to include
		focused:        false,
		width:          40,
		height:         20,
		allFiles:       make([]string, 0),
	}
}

// Update handles messages for the unified pattern panel
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle edit mode input
	if m.editMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				return m.confirmEdit()
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
			// Add include pattern
			m.newPatternType = IncludeType
			m.startAddPattern()
		case "A":
			// Add exclude pattern (Shift+A)
			m.newPatternType = ExcludeType
			m.startAddPattern()
		case "e":
			if m.hasPatternAtCursor() {
				m.startEditPattern()
			}
		case "d", "delete":
			cmd = m.deletePattern()
			return m, cmd
		case "enter":
			if m.hasPatternAtCursor() {
				m.startEditPattern()
			} else {
				// Default to include pattern on enter
				m.newPatternType = IncludeType
				m.startAddPattern()
			}
		case "t":
			m.testPatterns()
		}
	}

	return m, cmd
}

// View renders the unified pattern panel
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

// GetRegexFilters returns all patterns as ordered RegexFilter list
func (m *Model) GetRegexFilters() []models.RegexFilter {
	filters := make([]models.RegexFilter, 0, len(m.patterns))
	for _, p := range m.patterns {
		if p.Valid {
			compiled, _ := regexp.Compile(p.Text) // Already validated
			filter := models.RegexFilter{
				Pattern:  p.Text,
				Take:     p.Type == IncludeType,
				Compiled: compiled,
				Valid:    true,
				Error:    "",
			}
			filters = append(filters, filter)
		}
	}
	return filters
}

// Legacy methods for backward compatibility
func (m *Model) GetIncludePatterns() []string {
	patterns := make([]string, 0)
	for _, p := range m.patterns {
		if p.Valid && p.Type == IncludeType {
			patterns = append(patterns, p.Text)
		}
	}
	return patterns
}

func (m *Model) GetExcludePatterns() []string {
	patterns := make([]string, 0)
	for _, p := range m.patterns {
		if p.Valid && p.Type == ExcludeType {
			patterns = append(patterns, p.Text)
		}
	}
	return patterns
}

// Rendering methods

func (m *Model) renderNormalMode() string {
	// Header
	titleColor := primaryColor
	title := "🎯 Filter Patterns"

	header := headerStyle.
		Foreground(titleColor).
		Render(title)

	if m.focused {
		header = headerStyle.
			Foreground(titleColor).
			Background(lipgloss.Color("235")).
			Render(title + " *")
	}

	// Help
	help := ""
	if m.focused {
		helpItems := []string{
			"↑/↓: Navigate",
			"a: Add Include",
			"A: Add Exclude",
			"e/Enter: Edit",
			"d: Delete",
			"t: Test",
		}
		help = helpStyle.Render(strings.Join(helpItems, " • "))
	}

	// Calculate available space for content
	headerHeight := lipgloss.Height(header)
	helpHeight := lipgloss.Height(help)
	contentHeight := m.height - headerHeight - helpHeight

	if contentHeight < 1 {
		contentHeight = 1
	}

	// Content constrained to available height
	content := m.renderPatterns()
	constrainedContent := lipgloss.NewStyle().
		Height(contentHeight).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, header, constrainedContent, help)
}

func (m *Model) renderEditMode() string {
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

func (m *Model) renderPatterns() string {
	if len(m.patterns) == 0 {
		emptyMsg := "No patterns"
		if m.focused {
			emptyMsg += " (press 'a' for include, 'A' for exclude)"
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

func (m *Model) renderPattern(pattern Pattern, index int, isSelected bool) string {
	// Pattern type indicator and color
	var typeIcon string
	var typeColor lipgloss.Color

	if pattern.Type == IncludeType {
		typeIcon = "📥"
		typeColor = successColor
	} else {
		typeIcon = "📤"
		typeColor = errorColor
	}

	// Pattern validation indicator
	var statusIcon string
	if pattern.Valid {
		statusIcon = "✓"
	} else {
		statusIcon = "✗"
		typeColor = warningColor // Override color for invalid patterns
	}

	// Format pattern text
	patternText := pattern.Text
	if len(patternText) > 20 { // Shorter to make room for type icon
		patternText = patternText[:17] + "..."
	}

	// Match count
	matchInfo := ""
	if pattern.Valid {
		matchInfo = fmt.Sprintf(" (%d)", pattern.MatchCount)
	} else {
		matchInfo = " (error)"
	}

	content := fmt.Sprintf("%s %s %s%s", typeIcon, statusIcon, patternText, matchInfo)

	// Apply styling
	if isSelected {
		return selectedPatternStyle.Render(content)
	}

	return patternStyle.
		Foreground(typeColor).
		Render(content)
}

// Internal methods (same logic as the full regex model)

func (m *Model) hasPatternAtCursor() bool {
	return m.cursor >= 0 && m.cursor < len(m.patterns)
}

func (m *Model) moveCursorUp() {
	if m.cursor > 0 {
		m.cursor--
	} else if len(m.patterns) > 0 {
		m.cursor = len(m.patterns) - 1
	}
}

func (m *Model) moveCursorDown() {
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

	pattern := m.patterns[m.cursor]
	m.editMode = true
	m.editIndex = m.cursor
	m.editInput.SetValue(pattern.Text)
	m.editInput.Focus()
}

func (m *Model) confirmEdit() (*Model, tea.Cmd) {
	value := strings.TrimSpace(m.editInput.Value())
	if value == "" {
		model := m.cancelEdit()
		return model, nil
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
	model := m.cancelEdit()

	// Emit message to notify parent of pattern changes
	return model, m.emitPatternsChangedCmd()
}

func (m *Model) cancelEdit() *Model {
	m.editMode = false
	m.editIndex = -1
	m.editInput.Blur()
	m.editInput.SetValue("")
	return m
}

func (m *Model) deletePattern() tea.Cmd {
	if !m.hasPatternAtCursor() {
		return nil
	}

	m.patterns = append(m.patterns[:m.cursor], m.patterns[m.cursor+1:]...)

	// Adjust cursor if needed
	if m.cursor >= len(m.patterns) && len(m.patterns) > 0 {
		m.cursor = len(m.patterns) - 1
	} else if len(m.patterns) == 0 {
		m.cursor = 0
	}

	m.testPatterns()

	// Emit message to notify parent of pattern changes
	return m.emitPatternsChangedCmd()
}

func (m *Model) compilePattern(text string) Pattern {
	compiled, err := regexp.Compile(text)
	if err != nil {
		return Pattern{
			Text:       text,
			Type:       m.newPatternType,
			Compiled:   nil,
			Valid:      false,
			MatchCount: 0,
			Error:      err.Error(),
		}
	}

	return Pattern{
		Text:       text,
		Type:       m.newPatternType,
		Compiled:   compiled,
		Valid:      true,
		MatchCount: 0,
		Error:      "",
	}
}

func (m *Model) testPatterns() {
	for i := range m.patterns {
		m.patterns[i].MatchCount = m.countMatches(&m.patterns[i])
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

// emitPatternsChangedCmd creates a command that emits ordered regex filters
func (m *Model) emitPatternsChangedCmd() tea.Cmd {
	return func() tea.Msg {
		return messages.RegexFiltersChangedMsg{
			Filters:         m.GetRegexFilters(),
			SourceComponent: "unified_regex_panel",
		}
	}
}
