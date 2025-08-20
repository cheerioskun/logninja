package parser

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
)

// TimestampPattern represents a timestamp regex pattern with its corresponding Go time layout
type TimestampPattern struct {
	Name     string         // Human-readable name
	Regex    *regexp.Regexp // Compiled regex pattern
	Layout   string         // Go time layout for parsing
	Priority int            // Lower number = higher priority
}

// TimestampExtractor handles timestamp detection and parsing from log files
type TimestampExtractor struct {
	patterns []TimestampPattern
	fs       afero.Fs
}

// NewTimestampExtractor creates a new timestamp extractor with default patterns
func NewTimestampExtractor(fs afero.Fs) *TimestampExtractor {
	return &TimestampExtractor{
		patterns: compileDefaultPatterns(),
		fs:       fs,
	}
}

// compileDefaultPatterns returns pre-compiled regex patterns for timestamp matching
// Based on the user's existing implementation with optimizations
func compileDefaultPatterns() []TimestampPattern {
	patterns := []struct {
		name   string
		regex  string
		layout string
	}{
		// ISO format variations
		{"ISO8601_Micro", `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?`, "2006-01-02T15:04:05.000000Z"},
		{"ISO8601", `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z?`, "2006-01-02T15:04:05Z"},

		// Standard date/time formats
		{"DateTime_Slash", `\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}`, "2006/01/02 15:04:05"},
		{"DateTime_Dash", `\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`, "2006-01-02 15:04:05"},
		{"DateTime_Micro", `\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d+`, "2006-01-02 15:04:05.000000"},
		{"DateTime_Milli", `\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2},\d{3}Z?`, "2006-01-02 15:04:05,000Z"},

		// Glog formats
		{"Glog_Short", `[IWEF]\d{4}\s+\d{2}:\d{2}:\d{2}\.\d+Z?`, "I0102 15:04:05.000000Z"},
		{"Glog_Long", `[IWEF]\d{8}\s+\d{2}:\d{2}:\d{2}\.\d+Z?`, "I20060102 15:04:05.000000Z"},

		// Syslog format
		{"Syslog", `\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`, "Jan 2 15:04:05"},

		// Apache log format
		{"Apache", `\d{2}/\w{3}/\d{4}:\d{2}:\d{2}:\d{2}\s+\+\d{4}`, "02/Jan/2006:15:04:05 +0000"},

		// Audit log format (Unix timestamp)
		{"Audit", `msg=audit\((\d{10,19}):\d+\)`, "unix"},
	}

	compiled := make([]TimestampPattern, 0, len(patterns)*2)

	// First pass: add anchored patterns for fast matching (80% of logs start with timestamp)
	for i, p := range patterns {
		regex := regexp.MustCompile("^" + p.regex)
		compiled = append(compiled, TimestampPattern{
			Name:     p.name + "_Anchored",
			Regex:    regex,
			Layout:   p.layout,
			Priority: i, // Higher priority for anchored patterns
		})
	}

	// Second pass: add unanchored patterns for full line matching
	for i, p := range patterns {
		regex := regexp.MustCompile(p.regex)
		compiled = append(compiled, TimestampPattern{
			Name:     p.name,
			Regex:    regex,
			Layout:   p.layout,
			Priority: i + len(patterns), // Lower priority for unanchored
		})
	}

	return compiled
}

// PatternDetectionResult holds the result of hybrid pattern detection
type PatternDetectionResult struct {
	BestPattern *TimestampPattern
	MatchCount  int
	SampleTime  time.Time
	Confidence  float64 // 0.0 to 1.0
}

// DetectBestPattern implements the hybrid approach: check first 10 lines,
// find the pattern with most matches, use it as primary with fallback to all patterns
func (te *TimestampExtractor) DetectBestPattern(filePath string) (*PatternDetectionResult, error) {
	file, err := te.fs.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	maxLines := 10

	// Track matches per pattern
	patternMatches := make(map[int]int)       // pattern index -> match count
	firstTimestamp := make(map[int]time.Time) // pattern index -> first parsed time

	for scanner.Scan() && lineCount < maxLines {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		lineCount++

		// Test each pattern against this line
		for i, pattern := range te.patterns {
			if pattern.Regex.MatchString(line) {
				patternMatches[i]++

				// Try to parse the timestamp to verify it's valid
				if timestamp, err := te.parseTimestampWithPattern(line, &pattern); err == nil {
					if _, exists := firstTimestamp[i]; !exists {
						firstTimestamp[i] = timestamp
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	// Find the pattern with the most matches
	bestPatternIndex := -1
	maxMatches := 0

	for patternIndex, matches := range patternMatches {
		if matches > maxMatches {
			maxMatches = matches
			bestPatternIndex = patternIndex
		}
	}

	if bestPatternIndex == -1 {
		return &PatternDetectionResult{
			BestPattern: nil,
			MatchCount:  0,
			Confidence:  0.0,
		}, nil
	}

	bestPattern := &te.patterns[bestPatternIndex]
	confidence := float64(maxMatches) / float64(lineCount)
	sampleTime := firstTimestamp[bestPatternIndex]

	return &PatternDetectionResult{
		BestPattern: bestPattern,
		MatchCount:  maxMatches,
		SampleTime:  sampleTime,
		Confidence:  confidence,
	}, nil
}

// parseTimestampWithPattern extracts and parses a timestamp from a line using a specific pattern
func (te *TimestampExtractor) parseTimestampWithPattern(line string, pattern *TimestampPattern) (time.Time, error) {
	matches := pattern.Regex.FindStringSubmatch(line)
	if len(matches) == 0 {
		return time.Time{}, fmt.Errorf("no timestamp match found")
	}

	timestampStr := matches[0]

	// Handle special cases
	switch pattern.Layout {
	case "unix":
		// Extract Unix timestamp from audit log format
		if len(matches) > 1 {
			timestampStr = matches[1] // Use the captured group
		}
		if unixTime, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
			// Handle different Unix timestamp precisions
			switch len(timestampStr) {
			case 10: // seconds
				return time.Unix(unixTime, 0), nil
			case 13: // milliseconds
				return time.Unix(unixTime/1000, (unixTime%1000)*1000000), nil
			case 16: // microseconds
				return time.Unix(unixTime/1000000, (unixTime%1000000)*1000), nil
			case 19: // nanoseconds
				return time.Unix(unixTime/1000000000, unixTime%1000000000), nil
			}
		}
		return time.Time{}, fmt.Errorf("invalid unix timestamp: %s", timestampStr)

	case "Jan 2 15:04:05": // Syslog format needs year
		// Add current year for syslog timestamps
		currentYear := time.Now().Year()
		timestampWithYear := fmt.Sprintf("%d %s", currentYear, timestampStr)
		return time.Parse("2006 Jan 2 15:04:05", timestampWithYear)

	default:
		// Handle microsecond precision by trying multiple layouts
		layouts := []string{
			pattern.Layout,
			strings.Replace(pattern.Layout, ".000000", ".000000000", 1), // nanoseconds
			strings.Replace(pattern.Layout, ".000000", ".000", 1),       // milliseconds
			strings.Replace(pattern.Layout, ".000000", "", 1),           // no sub-seconds
		}

		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, timestampStr); err == nil {
				return parsed, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp: %s with layout: %s", timestampStr, pattern.Layout)
}

// ParseTimestamp extracts a timestamp from a line using the hybrid approach:
// 1. Try the best pattern first (if available)
// 2. Fall back to testing all patterns sequentially
func (te *TimestampExtractor) ParseTimestamp(line string, bestPattern *TimestampPattern) (time.Time, error) {
	if bestPattern != nil {
		if timestamp, err := te.parseTimestampWithPattern(line, bestPattern); err == nil {
			return timestamp, nil
		}
	}

	// Fallback: try all patterns
	for i := range te.patterns {
		if timestamp, err := te.parseTimestampWithPattern(line, &te.patterns[i]); err == nil {
			return timestamp, nil
		}
	}

	return time.Time{}, fmt.Errorf("no timestamp found in line")
}

// FindLineWithTimestamp searches for the first line in a reader that contains a valid timestamp
// Used for finding earliest/latest timestamps efficiently
func (te *TimestampExtractor) FindLineWithTimestamp(reader io.Reader, bestPattern *TimestampPattern, maxLines int) (time.Time, string, error) {
	scanner := bufio.NewScanner(reader)
	lineCount := 0

	for scanner.Scan() && (maxLines <= 0 || lineCount < maxLines) {
		line := scanner.Text()
		lineCount++

		if timestamp, err := te.ParseTimestamp(line, bestPattern); err == nil {
			return timestamp, line, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return time.Time{}, "", err
	}

	return time.Time{}, "", fmt.Errorf("no valid timestamp found in %d lines", lineCount)
}
