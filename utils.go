package main

import (
	"fmt"
	"strings"
	"time"
)

// ParseWordPressDate attempts to parse a WordPress date string using multiple formats
func ParseWordPressDate(dateStr string) (time.Time, error) {
	// List of possible date formats in WordPress
	dateFormats := []string{
		"2006-01-02 15:04:05",           // MySQL datetime format
		time.RFC3339,                    // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05-07:00",     // WordPress often uses this format
		"2006-01-02T15:04:05.000-07:00", // With milliseconds
		"2006-01-02T15:04:05.000Z",      // UTC with milliseconds
	}

	// Try each format until one works
	for _, format := range dateFormats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	// None of the formats worked
	return time.Time{}, fmt.Errorf("could not parse date using any known WordPress formats: %s", dateStr)
}

// SanitizeFilename removes characters that might cause problems in filenames
func SanitizeFilename(filename string) string {
	// Replace problematic characters with underscores
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(filename)
}
