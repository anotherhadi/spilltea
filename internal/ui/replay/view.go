package replay

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/anotherhadi/spilltea/internal/util"
)

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading...")
	}

	listH, bodyH := ilovetui.SplitH(m.height, m.renderStatusBar(), 0.35)
	leftW, rightW := m.bodyHalfWidths()

	bodyRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderRequestPanel(leftW, bodyH),
		m.renderResponsePanel(rightW, bodyH),
	)

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderListPanel(m.width, listH),
		bodyRow,
		m.renderStatusBar(),
	)
	return tea.NewView(content)
}

func (m *Model) renderListPanel(w, h int) string {
	panelStyle := ilovetui.S.Panel
	if !m.editing && m.focusedPanel == panelList {
		panelStyle = ilovetui.S.PanelFocused
	}
	var dots string
	if len(m.entries) > 0 {
		dots = ilovetui.S.Faint.Render(m.pager.View())
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)
	return ilovetui.RenderWithTitle(panelStyle, icons.I.Replay+"Replay", inner, w, h)
}

func (m *Model) renderRequestPanel(w, h int) string {
	var body string
	border := ilovetui.S.Panel
	if m.editing {
		body = m.textarea.View()
		border = ilovetui.S.PanelFocused
	} else {
		body = ilovetui.ViewportView(&m.requestViewport)
		if m.focusedPanel == panelRequest {
			border = ilovetui.S.PanelFocused
		}
	}
	return ilovetui.RenderWithTitle(border, icons.I.Request+"Request", body, w, h)
}

func (m *Model) renderResponsePanel(w, h int) string {
	border := ilovetui.S.Panel
	if !m.editing && m.focusedPanel == panelResponse {
		border = ilovetui.S.PanelFocused
	}
	return ilovetui.RenderWithTitle(border, icons.I.Response+"Response", ilovetui.ViewportView(&m.responseViewport), w, h)
}

func (m *Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(replayKeyMap{width: m.width}))
}

func (m *Model) renderList() string {
	if len(m.entries) == 0 {
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			ilovetui.S.Faint.Render(util.EmptyState(m.listViewport.Width(), "(╥﹏╥)", "send a request from History or Intercept")),
		)
	}

	start, end := util.PageBounds(m.pager, len(m.entries))

	var sb strings.Builder
	for i, e := range m.entries[start:end] {
		globalIdx := start + i
		selected := globalIdx == m.cursor
		selBg := ilovetui.S.Selection

		w := m.listViewport.Width()
		const fixedW = 2 + 7 + 1 + 3 + 1
		hostPathW := w - fixedW
		if hostPathW < 0 {
			hostPathW = 0
		}

		statusStr, statusSt := entryStatus(e)

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(selBg)
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(ilovetui.S.Primary).Width(2).Render(">"),
				style.S.Method(e.Method).Background(selBg).Render(e.Method),
				bg.Width(1).Render(""),
				statusSt.Background(selBg).Render(statusStr),
				bg.Width(1).Render(""),
				bg.Bold(true).Width(hostPathW).Render(e.Host+e.Path),
			)
		} else {
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				style.S.Method(e.Method).Render(e.Method),
				" ",
				statusSt.Render(statusStr),
				" ",
				ilovetui.S.Bold.Render(e.Host),
				ilovetui.S.Faint.Render(e.Path),
			)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func entryStatus(e Entry) (string, lipgloss.Style) {
	base := lipgloss.NewStyle().Bold(true).Width(3)
	switch {
	case e.Sending:
		return "···", base.Foreground(ilovetui.S.Subtle)
	case e.Err != nil:
		return "ERR", base.Foreground(ilovetui.S.Error)
	case e.StatusCode == 0:
		return "---", base.Foreground(ilovetui.S.Subtle)
	}
	return fmt.Sprintf("%3d", e.StatusCode), style.StatusStyle(e.StatusCode, 3)
}
