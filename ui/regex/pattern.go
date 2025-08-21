package regex

import "regexp"

// Pattern represents a regex pattern with metadata
type Pattern struct {
	Text       string         // The regex pattern text
	Compiled   *regexp.Regexp // Compiled regex (nil if invalid)
	Valid      bool           // Whether the pattern is valid
	MatchCount int            // Number of files matching this pattern
	Error      string         // Error message if invalid
}
