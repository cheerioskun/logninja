package regex

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	sectionStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Margin(0, 1, 1, 0)

	patternStyle = lipgloss.NewStyle().
			Padding(0, 1)

	selectedPatternStyle = lipgloss.NewStyle().
				Background(primaryColor).
				Foreground(lipgloss.Color("0")).
				Padding(0, 1)

	editInputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1).
			Margin(1, 0)

	helpStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Margin(1, 0, 0, 0)
)

func (m *Model) renderNormalMode() string {
	// Calculate section dimensions
	sectionHeight := (m.height - 6) / 2 // Leave space for title and help
	sectionWidth := m.width - 4

	// Render sections
	includeSection := m.renderSection(IncludeSection, sectionWidth, sectionHeight)
	excludeSection := m.renderSection(ExcludeSection, sectionWidth, sectionHeight)

	// Render help
	help := m.renderHelp()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		includeSection,
		excludeSection,
		help,
	)
}

func (m *Model) renderEditMode() string {
	title := "Edit Pattern"
	if m.editIndex == -1 {
		title = "Add Pattern"
	}

	sectionName := "Include"
	if m.activeSection == ExcludeSection {
		sectionName = "Exclude"
	}

	header := headerStyle.
		Foreground(primaryColor).
		Render(fmt.Sprintf("%s - %s Section", title, sectionName))

	input := editInputStyle.Render(m.editInput.View())

	editHelp := helpStyle.Render("Enter: Confirm • Esc: Cancel")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		input,
		editHelp,
	)
}

func (m *Model) renderSection(section Section, width, height int) string {
	isActive := section == m.activeSection
	patterns := m.getPatternsBySection(section)

	// Determine section title and color
	var title string
	var titleColor lipgloss.Color
	var borderColor lipgloss.Color

	if section == IncludeSection {
		title = "📥 Include Patterns"
		titleColor = successColor
	} else {
		title = "📤 Exclude Patterns"
		titleColor = errorColor
	}

	if isActive && m.focused {
		borderColor = primaryColor
	} else {
		borderColor = secondaryColor
	}

	// Create section header
	header := headerStyle.
		Foreground(titleColor).
		Render(title)

	if isActive && m.focused {
		header = headerStyle.
			Foreground(titleColor).
			Background(lipgloss.Color("235")).
			Render(title + " *")
	}

	// Render patterns
	content := m.renderPatterns(patterns, isActive && m.focused, height-4)

	// Style the section
	sectionContent := lipgloss.JoinVertical(lipgloss.Left, header, content)

	return sectionStyle.
		Width(width).
		Height(height).
		BorderForeground(borderColor).
		Render(sectionContent)
}

func (m *Model) renderPatterns(patterns []Pattern, isActive bool, maxHeight int) string {
	if len(patterns) == 0 {
		emptyMsg := "No patterns"
		if isActive {
			emptyMsg += " (press 'a' to add)"
		}
		return lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true).
			Render(emptyMsg)
	}

	var lines []string

	// Calculate visible range
	visibleStart := 0
	visibleEnd := len(patterns)

	if maxHeight > 0 && len(patterns) > maxHeight {
		// Adjust visible range to keep cursor in view
		if isActive {
			if m.cursor >= maxHeight {
				visibleStart = m.cursor - maxHeight + 1
			}
			visibleEnd = visibleStart + maxHeight
			if visibleEnd > len(patterns) {
				visibleEnd = len(patterns)
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
		pattern := patterns[i]
		line := m.renderPattern(pattern, i, isActive && i == m.cursor)
		lines = append(lines, line)
	}

	// Add scroll indicator if needed
	if visibleEnd < len(patterns) {
		lines = append(lines, lipgloss.NewStyle().Foreground(secondaryColor).Render("↓ ..."))
	}

	return strings.Join(lines, "\n")
}

func (m *Model) renderPattern(pattern Pattern, index int, isSelected bool) string {
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
	if len(patternText) > 30 {
		patternText = patternText[:27] + "..."
	}

	// Match count
	matchInfo := ""
	if pattern.Valid {
		matchInfo = fmt.Sprintf(" (%d matches)", pattern.MatchCount)
	} else {
		matchInfo = fmt.Sprintf(" (error: %s)", pattern.Error)
		if len(matchInfo) > 40 {
			matchInfo = matchInfo[:37] + "..."
		}
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

func (m *Model) renderHelp() string {
	if !m.focused {
		return ""
	}

	helpItems := []string{
		"Tab: Switch sections",
		"↑/↓: Navigate",
		"a: Add pattern",
		"e/Enter: Edit",
		"d: Delete",
		"t: Test patterns",
	}

	return helpStyle.Render(strings.Join(helpItems, " • "))
}

func (m *Model) getPatternsBySection(section Section) []Pattern {
	if section == IncludeSection {
		return m.includePatterns
	}
	return m.excludePatterns
}

// GetSectionStats returns statistics for display
func (m *Model) GetSectionStats() (includeCount, excludeCount, totalMatches int) {
	includeCount = len(m.includePatterns)
	excludeCount = len(m.excludePatterns)

	// Count total files that would be included
	filtered := m.GetFilteredFiles()
	totalMatches = len(filtered)

	return includeCount, excludeCount, totalMatches
}

// GetCurrentSectionName returns the name of the active section
func (m *Model) GetCurrentSectionName() string {
	if m.activeSection == IncludeSection {
		return "Include"
	}
	return "Exclude"
}
