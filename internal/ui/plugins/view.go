package plugins

import (
	"path/filepath"
	"strings"

	"charm.land/glamour/v2"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/config"
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
	panelStyle := s.PanelFocused
	if m.editing {
		panelStyle = s.Panel
	}
	dots := s.Faint.Render(m.pager.View())
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)
	return style.RenderWithTitle(panelStyle, icons.I.Plugin+"Plugins", inner, w, h)
}

func (m *Model) renderDetailPanel(h int) string {
	s := style.S
	panelStyle := s.Panel
	if m.editing {
		panelStyle = s.PanelFocused
	}
	info, ok := m.selected()
	if !ok {
		return style.RenderWithTitle(panelStyle, icons.I.Detail+"Detail", "", m.width, h)
	}

	statusSt := lipgloss.NewStyle().Foreground(s.Error)
	if info.Enabled {
		statusSt = lipgloss.NewStyle().Foreground(s.Success)
	}
	status := "disabled"
	if info.Enabled {
		status = "enabled"
	}

	pad := lipgloss.NewStyle().Padding(0, 1)

	header := pad.Render(
		s.Bold.Render(info.Name) + "  " + statusSt.Render(status) + "\n" +
			s.Faint.Render(filepath.Base(info.FilePath)),
	)

	parts := []string{header, m.detailViewport.View()}

	if m.hasConfig() {
		var configLabel string
		if m.editing {
			escKey := keys.Keys.Global.Escape.Help().Key
			configLabel = pad.Render(s.Faint.Render("editing config (" + escKey + " to save):"))
		} else {
			editKey := keys.Keys.Plugins.EditConfig.Help().Key
			configLabel = pad.Render(s.Faint.Render("config (" + editKey + " to edit):"))
		}
		parts = append(parts, "", configLabel, pad.Render(m.textarea.View()))
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return style.RenderWithTitle(panelStyle, icons.I.Detail+"Detail", inner, m.width, h)
}

func renderPluginDescription(desc string, width int) string {
	desc = strings.TrimSpace(desc)
	lines := strings.Split(desc, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeft(l, " \t")
	}
	desc = strings.Join(lines, "\n")

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style.GlamourStyleConfig(config.Global)),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return desc
	}
	out, err := r.Render(desc)
	if err != nil {
		return desc
	}
	return strings.Trim(out, "\n")
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
		return lipgloss.JoinVertical(lipgloss.Left, filterLine, pad.Render(m.help.View(pluginsKeyMap{editing: m.editing, hasConfig: m.hasConfig()})))
	}
	return pad.Render(m.help.View(pluginsKeyMap{editing: m.editing, hasConfig: m.hasConfig()}))
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
