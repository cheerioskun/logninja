package messages

import "github.com/cheerioskun/logninja/internal/models"

// RegexFiltersChangedMsg is sent when the ordered regex filter list is modified
type RegexFiltersChangedMsg struct {
	Filters         []models.RegexFilter // Complete ordered list of regex filters
	SourceComponent string               // Which component sent this
}

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
