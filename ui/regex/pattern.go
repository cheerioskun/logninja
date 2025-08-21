package regex

import "regexp"

// PatternType represents whether this is for include or exclude patterns
type PatternType int

const (
	IncludeType PatternType = iota
	ExcludeType
)

// String returns a human-readable representation of the pattern type
func (pt PatternType) String() string {
	switch pt {
	case IncludeType:
		return "Include"
	case ExcludeType:
		return "Exclude"
	default:
		return "Unknown"
	}
}

// Pattern represents a regex pattern with metadata
type Pattern struct {
	Text       string         // The regex pattern text
	Type       PatternType    // Whether this is an include or exclude pattern
	Compiled   *regexp.Regexp // Compiled regex (nil if invalid)
	Valid      bool           // Whether the pattern is valid
	MatchCount int            // Number of files matching this pattern
	Error      string         // Error message if invalid
}
