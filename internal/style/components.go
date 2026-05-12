package style

import (
	"strings"

	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

func NewViewport() viewport.Model {
	vp := viewport.New()
	vp.MouseWheelEnabled = false
	return vp
}

func NewPaginator() paginator.Model {
	p := paginator.New()
	p.Type = paginator.Dots
	p.ActiveDot = S.PagerDotActive
	p.InactiveDot = S.PagerDotInactive
	return p
}

func NewTextarea(showLineNumbers bool) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = showLineNumbers
	ta.CharLimit = 0
	ts := ta.Styles()
	ts.Focused.Base = lipgloss.NewStyle()
	ts.Blurred.Base = lipgloss.NewStyle()
	ts.Focused.CursorLine = lipgloss.NewStyle().Background(S.Selection).Foreground(S.Text)
	ts.Focused.Placeholder = lipgloss.NewStyle().Foreground(S.Subtle)
	ts.Blurred.Placeholder = lipgloss.NewStyle().Foreground(S.Subtle)
	ts.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(S.SubtleBg)
	ts.Blurred.EndOfBuffer = lipgloss.NewStyle().Foreground(S.SubtleBg)
	ts.Blurred.Text = lipgloss.NewStyle().Foreground(S.MutedFg)
	ta.SetStyles(ts)
	return ta
}

// SeverityStyle returns a bold lipgloss style coloured by finding severity level.
func SeverityStyle(sev string) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true)
	switch sev {
	case "critical":
		return base.Foreground(S.Error)
	case "high":
		return base.Foreground(S.Warning)
	case "medium":
		return base.Foreground(S.Primary)
	case "low":
		return base.Foreground(S.Success)
	default:
		return base.Foreground(S.Subtle)
	}
}

// StatusStyle returns a bold lipgloss style coloured by HTTP status code.
func StatusStyle(code, width int) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true).Width(width)
	switch {
	case code >= 500:
		return base.Foreground(S.Error)
	case code >= 400:
		return base.Foreground(S.Warning)
	case code >= 300:
		return base.Foreground(S.Primary)
	default:
		return base.Foreground(S.Success)
	}
}

// SplitH splits totalHeight into top and bottom sections, accounting for the
// status bar height.
func SplitH(totalHeight int, statusBar string, ratio float64) (top, bottom int) {
	statusH := strings.Count(statusBar, "\n") + 1
	available := totalHeight - statusH
	top = int(float64(available) * ratio)
	bottom = available - top
	return
}
