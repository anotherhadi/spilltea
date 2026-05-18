package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/style"
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
	s := style.S
	// content width inside bordered panel
	inner := m.getSidebarWidth() - 2

	titleText := "SPILLTEA"
	if m.sidebarState == sidebarCollapsed {
		titleText = "SPLT"
	}
	title := lipgloss.NewStyle().Width(inner).Bold(true).Foreground(s.Primary).Padding(0, 1).Render(titleText)

	divider := strings.Repeat("─", inner)

	badgeSelected := lipgloss.NewStyle().Foreground(s.Primary).Bold(true)
	badgeNormal := lipgloss.NewStyle().Foreground(s.Subtle)
	textSelected := lipgloss.NewStyle().Foreground(s.Primary)
	textNormal := lipgloss.NewStyle().Foreground(s.Text)
	lineStyle := lipgloss.NewStyle().Width(inner).Padding(0, 1)

	var items strings.Builder
	for i, entry := range sidebarEntries {
		selected := entry.id == m.page
		badgeStyle, textStyle := badgeNormal, textNormal
		if selected {
			badgeStyle, textStyle = badgeSelected, textSelected
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
		lipgloss.NewStyle().Width(inner).Foreground(s.Subtle).Padding(0, 1).Render(name),
	}
	parts = append(parts,
		lipgloss.NewStyle().Foreground(s.Subtle).Render(divider),
		items.String(),
	)
	body := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return s.Panel.Width(m.getSidebarWidth()).Height(m.height).Render(body)
}
