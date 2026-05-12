package plugins

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

func (m Model) View() tea.View {
	if m.width == 0 || m.manager == nil {
		return tea.NewView(style.S.Faint.Render("\nno plugins loaded"))
	}

	listH, detailH := style.SplitH(m.height, m.renderStatusBar(), 0.4)

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderListPanel(m.width, listH),
		m.renderDetailPanel(detailH),
		m.renderStatusBar(),
	)
	return tea.NewView(content)
}

func (m *Model) renderListPanel(w, h int) string {
	s := style.S
	dots := s.Faint.Render(m.pager.View())
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)
	return style.RenderWithTitle(s.PanelFocused, icons.I.Plugin+"Plugins", inner, w, h)
}

func (m *Model) renderDetailPanel(h int) string {
	s := style.S
	info, ok := m.selected()
	if !ok {
		return style.RenderWithTitle(s.Panel, "Config", "", m.width, h)
	}

	var sb strings.Builder

	statusSt := lipgloss.NewStyle().Foreground(s.Error)
	if info.Enabled {
		statusSt = lipgloss.NewStyle().Foreground(s.Success)
	}
	status := "disabled"
	if info.Enabled {
		status = "enabled"
	}
	sb.WriteString(s.Bold.Render(info.Name) + "  " + statusSt.Render(status) + "\n")
	sb.WriteString(s.Faint.Render(shortenPath(info.FilePath)) + "\n\n")

	if m.editing {
		escKey := keys.Keys.Global.Escape.Help().Key
		sb.WriteString(s.Faint.Render("editing config (" + escKey + " to save):"))
	} else {
		editKey := keys.Keys.Plugins.EditConfig.Help().Key
		sb.WriteString(s.Faint.Render("config (" + editKey + " to edit):"))
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Padding(0, 1).Render(sb.String()),
		lipgloss.NewStyle().Padding(0, 1).Render(m.textarea.View()),
	)
	return style.RenderWithTitle(s.Panel, "Detail", inner, m.width, h)
}

func (m *Model) renderStatusBar() string {
	s := style.S
	pad := lipgloss.NewStyle().Padding(0, 1)
	filterKey := keys.Keys.Plugins.Filter.Help().Key
	if m.filtering {
		return pad.Render(s.Faint.Render(filterKey) + " " + m.filterInput.View())
	}
	if m.filter != "" {
		escKey := keys.Keys.Global.Escape.Help().Key
		accent := lipgloss.NewStyle().Foreground(s.Primary)
		filterLine := pad.Render(accent.Render(filterKey) + " " + s.Bold.Render(m.filter) + s.Faint.Render("  "+escKey+" to clear"))
		return lipgloss.JoinVertical(lipgloss.Left, filterLine, pad.Render(m.help.View(pluginsKeyMap{editing: m.editing})))
	}
	return pad.Render(m.help.View(pluginsKeyMap{editing: m.editing}))
}

func (m *Model) renderList() string {
	s := style.S
	if len(m.filtered) == 0 {
		msg := " (ง •̀_•́)ง\nno plugins"
		if m.filter != "" {
			msg = "   = _ =\nno results"
		}
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			s.Faint.Render(msg),
		)
	}

	start, end := m.pager.GetSliceBounds(len(m.filtered))
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}

	var sb strings.Builder
	for i, p := range m.filtered[start:end] {
		globalIdx := start + i
		selected := globalIdx == m.cursor

		enabledSt := lipgloss.NewStyle().Foreground(s.Error)
		enabledStr := "off"
		if p.Enabled {
			enabledSt = lipgloss.NewStyle().Foreground(s.Success)
			enabledStr = "on "
		}

		w := m.listViewport.Width()
		const fixedW = 2 + 3 + 1
		nameW := w - fixedW
		if nameW < 0 {
			nameW = 0
		}

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(s.Selection)
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(s.Primary).Width(2).Render(">"),
				enabledSt.Background(s.Selection).Width(3).Render(enabledStr),
				bg.Width(1).Render(""),
				bg.Bold(true).Width(nameW).Render(p.Name),
			)
		} else {
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				enabledSt.Width(3).Render(enabledStr),
				" ",
				s.Bold.Render(p.Name),
			)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
