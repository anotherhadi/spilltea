package util

import (
	"strings"

	"charm.land/bubbles/v2/paginator"
	"charm.land/lipgloss/v2"
)

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// EmptyState renders centered placeholder lines, word-wrapping to fit maxW.
func EmptyState(maxW int, lines ...string) string {
	return lipgloss.NewStyle().
		Width(maxW).
		AlignHorizontal(lipgloss.Center).
		Render(strings.Join(lines, "\n"))
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

// PageBounds returns clamped start/end indices for rendering a paginated list.
func PageBounds(p paginator.Model, total int) (start, end int) {
	start, end = p.GetSliceBounds(total)
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	return
}

// InferScheme returns "http" for port 80, "https" otherwise.
func InferScheme(host string) string {
	if strings.HasSuffix(host, ":80") {
		return "http"
	}
	return "https"
}
