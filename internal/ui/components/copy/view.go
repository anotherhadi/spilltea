package copy

import (
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/style"
	copyasUI "github.com/anotherhadi/spilltea/internal/ui/components/copyas"
)

func (m *Model) View(background string) string {
	s := style.S

	hint := lipgloss.NewStyle().Foreground(s.Subtle).
		Render("  enter: copy  •  /: filter  •  esc: cancel")

	inner := lipgloss.JoinVertical(lipgloss.Left,
		m.list.View(),
		hint,
	)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Primary)

	popupH := m.popupHeight()
	popup := style.RenderWithTitle(border, "Copy", inner, popupInnerW+2, popupH)

	return copyasUI.OverlayCenter(background, popup, m.width, m.height)
}
