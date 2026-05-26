package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
)

type sidebarState string

const (
	sidebarHidden    sidebarState = "hidden"
	sidebarCollapsed sidebarState = "collapsed"
	sidebarExpanded  sidebarState = "expanded"
)

func (m *Model) cycleSidebarState() {
	switch m.sidebarState {
	case sidebarHidden:
		m.sidebarState = sidebarCollapsed
	case sidebarCollapsed:
		m.sidebarState = sidebarExpanded
	default:
		m.sidebarState = sidebarHidden
	}
}

func (m Model) getSidebarWidth() int {
	switch m.sidebarState {
	case sidebarHidden:
		return 0
	case sidebarCollapsed:
		return 8
	default:
		return 18
	}
}

func (m *Model) renderSidebar() string {
	if m.sidebarState == sidebarHidden {
		return ""
	}
	// content width inside bordered panel
	inner := m.getSidebarWidth() - 2

	titleText := "SPILLTEA"
	if m.sidebarState == sidebarCollapsed {
		titleText = "SPLT"
	}
	title := lipgloss.NewStyle().Width(inner).Bold(true).Foreground(ilovetui.S.Primary).Padding(0, 1).Render(titleText)

	divider := strings.Repeat("─", inner)

	badgeSelected := lipgloss.NewStyle().Foreground(ilovetui.S.Primary).Bold(true)
	badgeNormal := lipgloss.NewStyle().Foreground(ilovetui.S.Subtle)
	textSelected := lipgloss.NewStyle().Foreground(ilovetui.S.Primary)
	textNormal := lipgloss.NewStyle().Foreground(ilovetui.S.Text)
	lineStyle := lipgloss.NewStyle().Width(inner).Padding(0, 1)

	var items strings.Builder
	badgeUnread := lipgloss.NewStyle().Foreground(ilovetui.S.Warning).Bold(true)

	for i, entry := range sidebarEntries {
		selected := entry.id == m.page
		badgeStyle, textStyle := badgeNormal, textNormal
		if selected {
			badgeStyle, textStyle = badgeSelected, textSelected
		} else if entry.hasUpdate != nil && entry.hasUpdate(m) {
			badgeStyle = badgeUnread
		}
		icon := ""
		if entry.icon != nil {
			icon = entry.icon()
		}
		label := " " + icon
		if m.sidebarState != sidebarCollapsed {
			label += string(entry.id)
		}
		line := lineStyle.Render(badgeStyle.Render(strconv.Itoa(i+1)) + textStyle.Render(label))
		if m.sidebarState == sidebarCollapsed && icon == "" {
			line = " " + line
		}
		items.WriteString(line + "\n")
	}

	maxLen := inner - 2
	name := m.projectName
	if m.sidebarState == sidebarCollapsed && name == "temporary" {
		name = "tmp"
	} else if len(name) > maxLen {
		name = name[:maxLen-1] + "…"
	}
	parts := []string{
		title,
		lipgloss.NewStyle().Width(inner).Foreground(ilovetui.S.Subtle).Padding(0, 1).Render(name),
	}
	parts = append(parts,
		lipgloss.NewStyle().Foreground(ilovetui.S.Subtle).Render(divider),
		items.String(),
	)
	body := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return ilovetui.S.Panel.Width(m.getSidebarWidth()).Height(m.height).Render(body)
}
