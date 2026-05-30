package findings

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

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderListPanel(m.width, listH),
		m.renderBodyPanel(bodyH),
		m.renderStatusBar(),
	)
	return tea.NewView(content)
}

func (m *Model) renderListPanel(w, h int) string {
	var dots string
	if len(m.findings) > 0 {
		dots = ilovetui.S.Faint.Render(m.pager.View())
	}
	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.listViewport.View(),
		lipgloss.PlaceHorizontal(m.listViewport.Width(), lipgloss.Center, dots),
	)
	return ilovetui.RenderWithTitle(ilovetui.S.PanelFocused, icons.I.Findings+"Findings", inner, w, h)
}

func (m *Model) renderBodyPanel(h int) string {
	title := "Description"
	if len(m.findings) > 0 {
		title = m.findings[m.cursor].Title
	}
	return ilovetui.RenderWithTitle(ilovetui.S.Panel, title, ilovetui.ViewportView(&m.bodyViewport), m.width, h)
}

func (m *Model) renderList() string {
	if len(m.findings) == 0 {
		return lipgloss.Place(
			m.listViewport.Width(), m.listViewport.Height(),
			lipgloss.Center, lipgloss.Center,
			ilovetui.S.Faint.Render(util.EmptyState(m.listViewport.Width(), "(҂◡_◡) ᕤ", "no findings")),
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
		flagSt := lipgloss.NewStyle().Foreground(ilovetui.S.Primary)

		w := m.listViewport.Width()
		const fixedW = 2 + 2 + 8 + 1 + 8 + 1 + 10 + 1
		titleW := w - fixedW
		if titleW < 0 {
			titleW = 0
		}

		pluginStr := ilovetui.S.Faint.Width(8).Render(util.Truncate(f.PluginName, 8))

		var line string
		if selected {
			bg := lipgloss.NewStyle().Background(ilovetui.S.Selection)
			flagStr := "  "
			if f.Flagged {
				flagStr = icons.I.Flag + " "
			}
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				bg.Bold(true).Foreground(ilovetui.S.Primary).Width(2).Render(">"),
				bg.Foreground(ilovetui.S.Primary).Width(2).Render(flagStr),
				sevStyle.Background(ilovetui.S.Selection).Width(8).Render(f.Severity),
				bg.Width(1).Render(""),
				bg.Foreground(ilovetui.S.Subtle).Width(8).Render(util.Truncate(f.PluginName, 8)),
				bg.Width(1).Render(""),
				bg.Foreground(ilovetui.S.Subtle).Width(10).Render(ts),
				bg.Width(1).Render(""),
				bg.Bold(true).Width(titleW).Render(f.Title),
			)
		} else {
			flagStr := "  "
			if f.Flagged {
				flagStr = icons.I.Flag + " "
			}
			line = lipgloss.JoinHorizontal(lipgloss.Top,
				"  ",
				flagSt.Width(2).Render(flagStr),
				sevLabel,
				" ",
				pluginStr,
				" ",
				ilovetui.S.Faint.Width(10).Render(ts),
				" ",
				ilovetui.S.Bold.Render(f.Title),
			)
		}
		sb.WriteString(fmt.Sprintf("%s\n", line))
	}
	return sb.String()
}
