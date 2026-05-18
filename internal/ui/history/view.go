package history

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading...")
	}

	listH, bodyH := style.SplitH(m.height, m.renderStatusBar(), 0.35)

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderListPanel(m.width, listH),
		m.renderBodyPanel(bodyH),
		m.renderStatusBar(),
	)
	return tea.NewView(content)
}

func (m *Model) renderListPanel(w, h int) string {
	s := style.S
	var dots string
	if len(m.entries) > 0 {
		dots = s.Faint.Render(m.pager.View())
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)
	return style.RenderWithTitle(s.PanelFocused, icons.I.History+"History", inner, w, h)
}

func (m *Model) renderBodyPanel(h int) string {
	s := style.S
	title := icons.I.Request + "Request"
	if m.focusedPanel == panelResponse {
		title = icons.I.Response + "Response"
	}
	return style.RenderWithTitle(s.Panel, title, m.bodyViewport.View(), m.width, h)
}

func (m *Model) renderStatusBar() string {
	s := style.S
	pad := lipgloss.NewStyle().Padding(0, 1)
	escKey := keys.Keys.Global.Escape.Help().Key
	switch m.searchKind {
	case searchKindFulltext:
		filterKey := keys.Keys.History.Filter.Help().Key
		if m.searchAccepted {
			accent := lipgloss.NewStyle().Foreground(s.Primary)
			filterLine := pad.Render(accent.Render(filterKey) + " " + s.Bold.Render(m.searchInput.Value()) + s.Faint.Render("  "+escKey+" to clear"))
			return lipgloss.JoinVertical(lipgloss.Left, filterLine, pad.Render(m.help.View(historyKeyMap{width: m.width})))
		}
		return pad.Render(s.Faint.Render(filterKey) + " " + m.searchInput.View())
	case searchKindSQL:
		sqlKey := keys.Keys.History.SqlQuery.Help().Key
		if m.searchAccepted {
			accent := lipgloss.NewStyle().Foreground(s.Primary)
			filterLine := pad.Render(accent.Render(sqlKey) + " " + s.Bold.Render(m.searchInput.Value()) + s.Faint.Render("  "+escKey+" to clear"))
			return lipgloss.JoinVertical(lipgloss.Left, filterLine, pad.Render(m.help.View(historyKeyMap{width: m.width})))
		}
		return pad.Render(s.Faint.Render(sqlKey) + " " + m.searchInput.View())
	default:
		return pad.Render(m.help.View(historyKeyMap{width: m.width}))
	}
}

func (m *Model) renderList() string {
	s := style.S
	if m.searchErr != "" {
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(s.Error).Render(m.searchErr),
		)
	}
	if len(m.entries) == 0 {
		msg := "    (⌐■_■)\nno history yet"
		if m.searchKind != searchKindOff {
			msg = "ʕノ•ᴥ•ʔノ ︵ ┻━┻\n   no results"
		}
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			s.Faint.Render(msg),
		)
	}

	start, end := m.pager.GetSliceBounds(len(m.entries))
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}

	var sb strings.Builder
	for i, e := range m.entries[start:end] {
		globalIdx := start + i
		selected := globalIdx == m.cursor

		selBg := s.Selection
		w := m.listViewport.Width()

		statusStr := fmt.Sprintf("%3d", e.StatusCode)
		const fixedW = 2 + 7 + 1 + 3 + 1 + 10 + 1
		hostPathW := w - fixedW
		if hostPathW < 0 {
			hostPathW = 0
		}

		ts := e.Timestamp.Format("15:04:05")
		statusSt := style.StatusStyle(e.StatusCode, 3)

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(selBg)
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(s.Primary).Width(2).Render(">"),
				s.Method(e.Method).Background(selBg).Render(e.Method),
				bg.Width(1).Render(""),
				statusSt.Background(selBg).Render(statusStr),
				bg.Width(1).Render(""),
				bg.Foreground(s.Subtle).Width(10).Render(ts),
				bg.Width(1).Render(""),
				bg.Bold(true).Width(hostPathW).Render(e.Host+e.Path),
			)
		} else {
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				s.Method(e.Method).Render(e.Method),
				" ",
				statusSt.Render(statusStr),
				" ",
				s.Faint.Width(10).Render(ts),
				" ",
				s.Bold.Render(e.Host),
				s.Faint.Render(e.Path),
			)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
