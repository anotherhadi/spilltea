package style

import (
	"strings"

	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
)

func NewViewport() viewport.Model {
	vp := viewport.New()
	vp.MouseWheelEnabled = false
	return vp
}

func ViewportView(vp *viewport.Model) string {
	v := vp.View()
	if vp.AtBottom() {
		return v
	}
	lines := strings.Split(v, "\n")
	if len(lines) == 0 {
		return v
	}
	arrow := lipgloss.NewStyle().Foreground(ilovetui.S.Subtle).Render("↓")
	arrowW := lipgloss.Width(arrow)
	inner := vp.Width() - 2*arrowW
	if inner < 0 {
		inner = 0
	}
	lines[len(lines)-1] = arrow + strings.Repeat(" ", inner) + arrow
	return strings.Join(lines, "\n")
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
	ta.EndOfBufferCharacter = '~'
	ts := ta.Styles()
	ts.Focused.Base = lipgloss.NewStyle()
	ts.Blurred.Base = lipgloss.NewStyle()
	ts.Focused.Text = lipgloss.NewStyle().Foreground(ilovetui.S.Text)
	ts.Focused.CursorLine = lipgloss.NewStyle().Background(ilovetui.S.Selection).Foreground(ilovetui.S.Text)
	ts.Focused.CursorLineNumber = lipgloss.NewStyle().Background(ilovetui.S.Selection).Foreground(ilovetui.S.Primary).Bold(true)
	ts.Focused.LineNumber = lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)
	ts.Focused.Placeholder = lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)
	ts.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(ilovetui.S.SubtleBg)
	ts.Blurred.Text = lipgloss.NewStyle().Foreground(ilovetui.S.Muted)
	ts.Blurred.LineNumber = lipgloss.NewStyle().Foreground(ilovetui.S.SubtleBg)
	ts.Blurred.Placeholder = lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)
	ts.Blurred.EndOfBuffer = lipgloss.NewStyle().Foreground(ilovetui.S.SubtleBg)
	ta.SetStyles(ts)
	return ta
}

func SeverityStyle(sev string) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true)
	switch sev {
	case "critical":
		return base.Foreground(ilovetui.S.Error)
	case "high":
		return base.Foreground(ilovetui.S.Warning)
	case "medium":
		return base.Foreground(ilovetui.S.Primary)
	case "low":
		return base.Foreground(ilovetui.S.Success)
	default:
		return base.Foreground(ilovetui.S.Subtle)
	}
}

func StatusStyle(code, width int) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true).Width(width)
	switch {
	case code >= 500:
		return base.Foreground(ilovetui.S.Error)
	case code >= 400:
		return base.Foreground(ilovetui.S.Warning)
	case code >= 300:
		return base.Foreground(ilovetui.S.Primary)
	default:
		return base.Foreground(ilovetui.S.Success)
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
