package style

import (
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
)

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
