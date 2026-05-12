package diff

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/style"
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
	s := style.S

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

	leftBorder := s.Panel
	rightBorder := s.Panel
	switch m.focus {
	case bothSlots:
		leftBorder = s.PanelFocused
		rightBorder = s.PanelFocused
	case leftSlot:
		leftBorder = s.PanelFocused
	case rightSlot:
		rightBorder = s.PanelFocused
	}

	left := style.RenderWithTitle(leftBorder, leftTitle, m.leftViewport.View(), leftW, panelH)
	right := style.RenderWithTitle(rightBorder, rightTitle, m.rightViewport.View(), rightW, panelH)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *Model) renderStatusBar() string {
	return lipgloss.NewStyle().Padding(0, 1).Render(m.help.View(diffKeyMap{width: m.width}))
}

func renderLeftLines(lines []diffLine) string {
	s := style.S
	var sb strings.Builder
	for _, l := range lines {
		switch l.kind {
		case lineRemoved:
			sb.WriteString(style.Paint(s.Error, "- ") + l.text + "\n")
		case lineAdded:
			sb.WriteString("\n")
		default:
			sb.WriteString("  " + l.text + "\n")
		}
	}
	return sb.String()
}

func renderRightLines(lines []diffLine) string {
	s := style.S
	var sb strings.Builder
	for _, l := range lines {
		switch l.kind {
		case lineAdded:
			sb.WriteString(style.Paint(s.Success, "+ ") + l.text + "\n")
		case lineRemoved:
			sb.WriteString("\n")
		default:
			sb.WriteString("  " + l.text + "\n")
		}
	}
	return sb.String()
}
