package copy

import (
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	copyasUI "github.com/anotherhadi/spilltea/internal/ui/components/copyas"
)

func (m *Model) View(background string) string {
	hint := lipgloss.NewStyle().Foreground(ilovetui.S.Subtle).
		Render("  enter: copy  •  /: filter  •  esc: cancel")

	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.list.View(),
		hint,
	)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ilovetui.S.Primary)

	popupH := m.popupHeight()
	popup := ilovetui.RenderWithTitle(border, "Copy", inner, m.popupInnerWidth()+2, popupH)

	return copyasUI.OverlayCenter(background, popup, m.width, m.height)
}
