package strata

import (
	"fmt"
	"strings"
	"time"
)

func FormatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	totalSeconds := int64(duration.Round(time.Second) / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	parts := make([]string, 0, 3)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || hours > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

func FormatSignedDuration(duration time.Duration) string {
	if duration < 0 {
		return "-" + FormatDuration(-duration)
	}
	return FormatDuration(duration)
}

func FormatLocalTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}
