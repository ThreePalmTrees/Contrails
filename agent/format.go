package agent

import (
	"strings"
	"time"
)

// FormatTimestamp converts a millisecond Unix timestamp to a human-readable string.
func FormatTimestamp(milliseconds int64) string {
	if milliseconds == 0 {
		return "unknown"
	}
	t := time.UnixMilli(milliseconds)
	return t.Format("2006-01-02 15:04:05")
}

// FormatISO8601Timestamp converts an ISO 8601 timestamp string to a human-readable string.
func FormatISO8601Timestamp(isoTimestamp string) string {
	if isoTimestamp == "" {
		return "unknown"
	}
	parsed, err := time.Parse(time.RFC3339Nano, isoTimestamp)
	if err != nil {
		// Try without fractional seconds
		parsed, err = time.Parse("2006-01-02T15:04:05Z", isoTimestamp)
		if err != nil {
			return isoTimestamp
		}
	}
	return parsed.Format("2006-01-02 15:04:05")
}

// SanitizeFilename removes characters that are invalid in filenames.
func SanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	result := replacer.Replace(name)
	if len(result) > 100 {
		result = result[:100]
	}
	return strings.TrimSpace(result)
}
