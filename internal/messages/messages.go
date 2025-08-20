package messages

// RegexPatternsChangedMsg is sent when regex patterns are modified
type RegexPatternsChangedMsg struct {
	Type            RegexPatternType // Include or Exclude
	Patterns        []string         // Updated patterns
	SourceComponent string           // Which component sent this
}

// RegexPatternType indicates the type of regex pattern
type RegexPatternType int

const (
	IncludePatternType RegexPatternType = iota
	ExcludePatternType
)

// WorkingSetUpdatedMsg is sent when the working set has been recalculated
type WorkingSetUpdatedMsg struct {
	SelectedCount int      // Number of currently selected files
	TotalMatched  int      // Total files that match regex filters
	FilteredFiles []string // Files that passed all filters
	TotalSize     int64    // Total size of selected files
}

// RefreshComponentsMsg is sent to trigger component refreshes
type RefreshComponentsMsg struct {
	Reason string // Why the refresh was triggered
}
