package findings

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/anotherhadi/spilltea/internal/util"
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
	if len(m.findings) > 0 {
		dots = s.Faint.Render(m.pager.View())
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)
	return style.RenderWithTitle(s.PanelFocused, icons.I.Findings+"Findings", inner, w, h)
}

func (m *Model) renderBodyPanel(h int) string {
	s := style.S
	title := "Description"
	if len(m.findings) > 0 {
		title = m.findings[m.cursor].Title
	}
	return style.RenderWithTitle(s.Panel, title, style.ViewportView(&m.bodyViewport), m.width, h)
}

func (m *Model) renderList() string {
	s := style.S
	if len(m.findings) == 0 {
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			s.Faint.Render(util.CenterLines("(҂◡_◡) ᕤ", "no findings")),
		)
	}

	start, end := util.PageBounds(m.pager, len(m.findings))

	var sb strings.Builder
	for i, f := range m.findings[start:end] {
		globalIdx := start + i
		selected := globalIdx == m.cursor

		sevStyle := style.SeverityStyle(f.Severity)
		sevLabel := sevStyle.Width(8).Render(f.Severity)
		ts := f.CreatedAt.Format("15:04:05")

		w := m.listViewport.Width()
		const fixedW = 2 + 8 + 1 + 8 + 1 + 10 + 1
		titleW := w - fixedW
		if titleW < 0 {
			titleW = 0
		}

		pluginStr := s.Faint.Width(8).Render(util.Truncate(f.PluginName, 8))

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(s.Selection)
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(s.Primary).Width(2).Render(">"),
				sevStyle.Background(s.Selection).Width(8).Render(f.Severity),
				bg.Width(1).Render(""),
				bg.Foreground(s.Subtle).Width(8).Render(util.Truncate(f.PluginName, 8)),
				bg.Width(1).Render(""),
				bg.Foreground(s.Subtle).Width(10).Render(ts),
				bg.Width(1).Render(""),
				bg.Bold(true).Width(titleW).Render(f.Title),
			)
		} else {
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				sevLabel,
				" ",
				pluginStr,
				" ",
				s.Faint.Width(10).Render(ts),
				" ",
				s.Bold.Render(f.Title),
			)
		}
		sb.WriteString(fmt.Sprintf("%s\n", line))
	}
	return sb.String()
}
