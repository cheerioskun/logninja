package models

import (
	"fmt"
	"time"
)

// TimeRange represents a time span with start and end times
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// NewTimeRange creates a new TimeRange with validation
func NewTimeRange(start, end time.Time) (*TimeRange, error) {
	if start.After(end) {
		return nil, fmt.Errorf("start time %v cannot be after end time %v", start, end)
	}
	return &TimeRange{Start: start, End: end}, nil
}

// Duration returns the duration of the time range
func (tr *TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// Contains checks if the given time is within the range
func (tr *TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.Start) && !t.After(tr.End)
}

// Overlaps checks if this range overlaps with another range
func (tr *TimeRange) Overlaps(other *TimeRange) bool {
	if other == nil {
		return false
	}
	return tr.Start.Before(other.End) && tr.End.After(other.Start)
}

// Intersection returns the intersection of two time ranges
func (tr *TimeRange) Intersection(other *TimeRange) *TimeRange {
	if other == nil || !tr.Overlaps(other) {
		return nil
	}

	start := tr.Start
	if other.Start.After(start) {
		start = other.Start
	}

	end := tr.End
	if other.End.Before(end) {
		end = other.End
	}

	result, _ := NewTimeRange(start, end) // Already validated by overlaps check
	return result
}

// String returns a human-readable representation of the time range
func (tr *TimeRange) String() string {
	return fmt.Sprintf("%s - %s", tr.Start.Format(time.RFC3339), tr.End.Format(time.RFC3339))
}

// IsZero returns true if the time range is uninitialized
func (tr *TimeRange) IsZero() bool {
	return tr.Start.IsZero() && tr.End.IsZero()
}
