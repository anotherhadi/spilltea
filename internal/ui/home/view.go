package home

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/ui/components/teapot"
)

const inputPanelMaxW = 44

func (m Model) View() tea.View {
	iw := m.innerW()

	var sb strings.Builder
	sb.WriteString("\n")
	if m.height > teapotMinH {
		frames := teapot.TeapotFrames()
		frame := lipgloss.NewStyle().Foreground(ilovetui.S.Primary).Render(frames[m.teapotFrame])
		sb.WriteString(center(iw, frame))
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("\n")
	}
	sb.WriteString(center(iw, lipgloss.NewStyle().Bold(true).Foreground(ilovetui.S.Primary).Render("SPILLTEA")))
	sb.WriteString("\n")
	sb.WriteString(center(iw, ilovetui.S.Faint.Render("choose a project to get started")))
	sb.WriteString("\n\n")

	if m.mode == modeNaming {
		sb.WriteString(m.renderNamingPanel())
	} else {
		lw := m.listWidth()
		leftPad := (iw - lw) / 2
		sb.WriteString(padLeft(m.list.View(), leftPad))
		sb.WriteString("\n")
		sb.WriteString(center(iw, m.renderHelpLine()))
	}

	box := lipgloss.NewStyle().Padding(1, 1).Render(sb.String())
	content := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderNamingPanel() string {
	iw := m.innerW()

	panelW := inputPanelMaxW
	if iw < panelW+4 {
		panelW = iw - 4
	}
	if panelW < 10 {
		panelW = 10
	}
	innerW := inputPanelInnerW(iw)
	inputLine := lipgloss.NewStyle().Width(innerW).Render(m.nameInput.View())

	label := lipgloss.NewStyle().Foreground(ilovetui.S.Muted).Render("Project name")
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ilovetui.S.Primary).
		Padding(1, 2).
		Width(panelW).
		Render(label + "\n" + inputLine)

	hint := ilovetui.S.Faint.Render("[enter] confirm  [esc] cancel")

	var sb strings.Builder
	sb.WriteString(center(iw, panel))
	sb.WriteString("\n")
	sb.WriteString(center(iw, hint))
	sb.WriteString("\n")
	return sb.String()
}

// padLeft prepends n spaces to every non-empty line.
func padLeft(content string, n int) string {
	if n <= 0 {
		return content
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

func center(width int, s string) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, s)
}
