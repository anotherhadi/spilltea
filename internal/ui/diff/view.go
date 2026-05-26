package diff

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading...")
	}

	statusH := strings.Count(m.renderStatusBar(), "\n") + 1
	panelH := m.height - statusH

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderPanels(panelH),
		m.renderStatusBar(),
	)
	return tea.NewView(content)
}

func (m *Model) renderPanels(panelH int) string {
	leftW := m.width / 2
	rightW := m.width - leftW

	leftTitle := icons.I.Diff + "First"
	if m.left.label != "" {
		leftTitle = icons.I.Diff + "First: " + m.left.label
	}
	rightTitle := icons.I.Diff + "Second"
	if m.right.label != "" {
		rightTitle = icons.I.Diff + "Second: " + m.right.label
	}
	if maxW := leftW - 4; maxW > 0 {
		leftTitle = ansi.Truncate(leftTitle, maxW, "…")
	}
	if maxW := rightW - 4; maxW > 0 {
		rightTitle = ansi.Truncate(rightTitle, maxW, "…")
	}

	leftBorder := ilovetui.S.Panel
	rightBorder := ilovetui.S.Panel
	switch m.focus {
	case bothSlots:
		leftBorder = ilovetui.S.PanelFocused
		rightBorder = ilovetui.S.PanelFocused
	case leftSlot:
		leftBorder = ilovetui.S.PanelFocused
	case rightSlot:
		rightBorder = ilovetui.S.PanelFocused
	}

	left := ilovetui.RenderWithTitle(leftBorder, leftTitle, ilovetui.ViewportView(&m.leftViewport), leftW, panelH)
	right := ilovetui.RenderWithTitle(rightBorder, rightTitle, ilovetui.ViewportView(&m.rightViewport), rightW, panelH)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(diffKeyMap{width: m.width}))
}

func renderLeftLines(lines []diffLine) string {
	var sb strings.Builder
	for _, l := range lines {
		switch l.kind {
		case lineRemoved:
			sb.WriteString(style.Paint(ilovetui.S.Error, "- ") + l.text + "\n")
		case lineAdded:
			sb.WriteString("\n")
		default:
			sb.WriteString("  " + l.text + "\n")
		}
	}
	return sb.String()
}

func renderRightLines(lines []diffLine) string {
	var sb strings.Builder
	for _, l := range lines {
		switch l.kind {
		case lineAdded:
			sb.WriteString(style.Paint(ilovetui.S.Success, "+ ") + l.text + "\n")
		case lineRemoved:
			sb.WriteString("\n")
		default:
			sb.WriteString("  " + l.text + "\n")
		}
	}
	return sb.String()
}
