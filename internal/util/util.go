package util

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// CenterLines centers each line horizontally relative to the longest one.
func CenterLines(lines ...string) string {
	maxWidth := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > maxWidth {
			maxWidth = w
		}
	}
	centered := make([]string, len(lines))
	for i, l := range lines {
		centered[i] = lipgloss.PlaceHorizontal(maxWidth, lipgloss.Center, l)
	}
	return strings.Join(centered, "\n")
}

// InferScheme returns "http" for port 80, "https" otherwise.
func InferScheme(host string) string {
	if strings.HasSuffix(host, ":80") {
		return "http"
	}
	return "https"
}
