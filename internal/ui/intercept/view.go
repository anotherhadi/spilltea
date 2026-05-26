package intercept

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

	listH, bodyH := style.SplitH(m.height, m.renderStatusBar(), 0.35)

	var listRow string
	if m.captureResponse {
		leftW, rightW := m.listHalfWidths()
		listRow = lipgloss.JoinHorizontal(lipgloss.Top,
			m.renderListPanel(leftW, listH),
			m.renderResponseListPanel(rightW, listH),
		)
	} else {
		listRow = m.renderListPanel(m.width, listH)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		listRow,
		m.renderBodyPanel(bodyH),
		m.renderStatusBar(),
	)
	return tea.NewView(content)
}

func (m *Model) renderListPanel(w, h int) string {
	focused := !m.editing && (!m.captureResponse || m.focusedPanel == panelRequests)
	border := ilovetui.S.Panel
	if focused {
		border = ilovetui.S.PanelFocused
	}

	var dots string
	if len(m.queue) > 0 {
		dots = ilovetui.S.Faint.Render(m.pager.View())
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)

	title := icons.I.Request + "Requests"
	if !m.interceptEnabled {
		title += " " + lipgloss.NewStyle().Foreground(ilovetui.S.Error).Render("[intercept off]")
	}
	return ilovetui.RenderWithTitle(border, title, inner, w, h)
}

func (m *Model) renderResponseListPanel(w, h int) string {
	focused := !m.editing && m.focusedPanel == panelResponses
	border := ilovetui.S.Panel
	if focused {
		border = ilovetui.S.PanelFocused
	}

	dots := ilovetui.S.Faint.Render(m.responsePager.View())
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.responseViewport.View(),
		lipgloss.PlaceHorizontal(m.responseViewport.Width(), lipgloss.Center, dots),
	)

	return ilovetui.RenderWithTitle(border, icons.I.Response+"Responses", inner, w, h)
}

func (m *Model) renderBodyPanel(h int) string {
	var body string
	if m.editing {
		body = m.textarea.View()
	} else {
		body = style.ViewportView(&m.bodyViewport)
	}

	border := ilovetui.S.Panel
	if m.editing {
		border = ilovetui.S.PanelFocused
	}

	title := icons.I.Detail + "Details"
	return ilovetui.RenderWithTitle(border, title, body, m.width, h)
}

func (m *Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(interceptKeyMap{width: m.width}))
}

func (m *Model) renderList() string {
	if len(m.queue) == 0 {
		return lipgloss.Place(m.listViewport.Width(), m.listViewport.Height(), lipgloss.Center, lipgloss.Center, ilovetui.S.Faint.Render(util.CenterLines("(｡◕‿‿◕｡)", "waiting for a request")))
	}

	start, end := util.PageBounds(m.pager, len(m.queue))

	var sb strings.Builder
	for i, req := range m.queue[start:end] {
		globalIdx := start + i
		r := req.Flow.Request
		path := r.URL.Path
		if path == "" {
			path = "/"
		}

		selected := globalIdx == m.cursor
		selBg := ilovetui.S.Selection

		w := m.listViewport.Width()
		const fixedW = 2 + 7 + 2
		hostPathW := w - fixedW
		if hostPathW < 0 {
			hostPathW = 0
		}

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(selBg)
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(ilovetui.S.Primary).Width(2).Render(">"),
				style.S.Method(r.Method).Background(selBg).Render(r.Method),
				bg.Width(2).Render(""),
				bg.Bold(true).Width(hostPathW).Render(r.URL.Host+path),
			)
		} else {
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				style.S.Method(r.Method).Render(r.Method),
				ilovetui.S.Faint.Render("  "),
				ilovetui.S.Bold.Render(r.URL.Host),
				ilovetui.S.Faint.Render(path),
			)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func (m *Model) renderResponseList() string {
	if len(m.responseQueue) == 0 {
		return lipgloss.Place(m.responseViewport.Width(), m.responseViewport.Height(), lipgloss.Center, lipgloss.Center, ilovetui.S.Faint.Render(util.CenterLines("(҂◡_◡)", "no response yet")))
	}

	start, end := util.PageBounds(m.responsePager, len(m.responseQueue))

	var sb strings.Builder
	for i, resp := range m.responseQueue[start:end] {
		globalIdx := start + i
		f := resp.Flow
		path := f.Request.URL.Path
		if path == "" {
			path = "/"
		}

		code := 0
		if f.Response != nil {
			code = f.Response.StatusCode
		}
		statusStr := fmt.Sprintf("%d", code)

		selected := globalIdx == m.responseCursor
		selBg := ilovetui.S.Selection

		statusSt := style.StatusStyle(code, 7)

		w := m.responseViewport.Width()
		const fixedW = 2 + 7 + 2
		hostPathW := w - fixedW
		if hostPathW < 0 {
			hostPathW = 0
		}

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(selBg)
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(ilovetui.S.Primary).Width(2).Render(">"),
				statusSt.Background(selBg).Render(statusStr),
				bg.Width(2).Render(""),
				bg.Bold(true).Width(hostPathW).Render(f.Request.URL.Host+path),
			)
		} else {
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				statusSt.Render(statusStr),
				ilovetui.S.Faint.Render("  "),
				ilovetui.S.Bold.Render(f.Request.URL.Host),
				ilovetui.S.Faint.Render(path),
			)
		}
		sb.WriteString(line + "\n")
	}
	return sb.String()
}
