package history

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/anotherhadi/spilltea/internal/util"
)

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading...")
	}

	listH, bodyH := ilovetui.SplitH(m.height, m.renderStatusBar(), 0.35)

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderListPanel(m.width, listH),
		m.renderBodyPanel(bodyH),
		m.renderStatusBar(),
	)
	return tea.NewView(content)
}

func (m *Model) renderListPanel(w, h int) string {
	var dots string
	if len(m.entries) > 0 {
		dots = ilovetui.S.Faint.Render(m.pager.View())
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)
	return ilovetui.RenderWithTitle(ilovetui.S.PanelFocused, icons.I.History+"History", inner, w, h)
}

func (m *Model) renderBodyPanel(h int) string {
	title := icons.I.Request + "Request"
	if m.focusedPanel == panelResponse {
		title = icons.I.Response + "Response"
	}
	return ilovetui.RenderWithTitle(ilovetui.S.Panel, title, ilovetui.ViewportView(&m.bodyViewport), m.width, h)
}

func (m *Model) renderStatusBar() string {
	pad := lipgloss.NewStyle().Padding(0, 1)
	escKey := keys.Keys.Global.Escape.Help().Key
	switch m.searchKind {
	case searchKindFulltext:
		filterKey := keys.Keys.History.Filter.Help().Key
		if m.searchAccepted {
			accent := lipgloss.NewStyle().Foreground(ilovetui.S.Primary)
			filterLine := pad.Render(accent.Render(filterKey) + " " + ilovetui.S.Bold.Render(m.searchInput.Value()) + ilovetui.S.Faint.Render("  "+escKey+" to clear"))
			return lipgloss.JoinVertical(lipgloss.Left, filterLine, pad.Render(m.help.View(historyKeyMap{width: m.width})))
		}
		return pad.Render(ilovetui.S.Faint.Render(filterKey) + " " + m.searchInput.View())
	case searchKindSQL:
		sqlKey := keys.Keys.History.SqlQuery.Help().Key
		if m.searchAccepted {
			accent := lipgloss.NewStyle().Foreground(ilovetui.S.Primary)
			filterLine := pad.Render(accent.Render(sqlKey) + " " + ilovetui.S.Bold.Render(m.searchInput.Value()) + ilovetui.S.Faint.Render("  "+escKey+" to clear"))
			return lipgloss.JoinVertical(lipgloss.Left, filterLine, pad.Render(m.help.View(historyKeyMap{width: m.width})))
		}
		return pad.Render(ilovetui.S.Faint.Render(sqlKey) + " " + m.searchInput.View())
	default:
		return pad.Render(m.help.View(historyKeyMap{width: m.width}))
	}
}

func (m *Model) renderList() string {
	if m.searchErr != "" {
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(ilovetui.S.Error).Render(m.searchErr),
		)
	}
	if len(m.entries) == 0 {
		msg := util.EmptyState(m.listViewport.Width(), "(⌐■_■)", "no history yet")
		if m.searchKind != searchKindOff {
			msg = util.EmptyState(m.listViewport.Width(), "ʕノ•ᴥ•ʔノ ︵ ┻━┻", "no results")
		}
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			ilovetui.S.Faint.Render(msg),
		)
	}

	start, end := util.PageBounds(m.pager, len(m.entries))

	var sb strings.Builder
	for i, e := range m.entries[start:end] {
		globalIdx := start + i
		selected := globalIdx == m.cursor

		selBg := ilovetui.S.Selection
		w := m.listViewport.Width()

		statusStr := fmt.Sprintf("%3d", e.StatusCode)
		const fixedW = 2 + 2 + 7 + 1 + 3 + 1 + 10 + 1
		hostPathW := w - fixedW
		if hostPathW < 0 {
			hostPathW = 0
		}

		ts := e.Timestamp.Format("15:04:05")
		statusSt := style.StatusStyle(e.StatusCode, 3)
		flagSt := lipgloss.NewStyle().Foreground(ilovetui.S.Primary)

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(selBg)
			flagStr := "  "
			if e.Flagged {
				flagStr = icons.I.Flag + " "
			}
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(ilovetui.S.Primary).Width(2).Render(">"),
				bg.Foreground(ilovetui.S.Primary).Width(2).Render(flagStr),
				style.S.Method(e.Method).Background(selBg).Render(e.Method),
				bg.Width(1).Render(""),
				statusSt.Background(selBg).Render(statusStr),
				bg.Width(1).Render(""),
				bg.Foreground(ilovetui.S.Subtle).Width(10).Render(ts),
				bg.Width(1).Render(""),
				bg.Bold(true).Width(hostPathW).Render(e.Host+e.Path),
			)
		} else {
			flagStr := "  "
			if e.Flagged {
				flagStr = icons.I.Flag + " "
			}
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				flagSt.Width(2).Render(flagStr),
				style.S.Method(e.Method).Render(e.Method),
				" ",
				statusSt.Render(statusStr),
				" ",
				ilovetui.S.Faint.Width(10).Render(ts),
				" ",
				ilovetui.S.Bold.Render(e.Host),
				ilovetui.S.Faint.Render(e.Path),
			)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
