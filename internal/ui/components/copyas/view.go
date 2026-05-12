package copyas

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/charmbracelet/x/ansi"
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
	popup := style.RenderWithTitle(border, "Copy as", inner, popupInnerW+2, popupH)

	return overlayCenter(background, popup, m.width, m.height)
}

func overlayCenter(bg, popup string, w, h int) string {
	s := style.S

	stripped := ansi.Strip(bg)
	rawLines := strings.Split(stripped, "\n")
	bgRunes := make([][]rune, h)
	for y := 0; y < h; y++ {
		var line []rune
		if y < len(rawLines) {
			line = []rune(rawLines[y])
		}
		if len(line) > w {
			line = line[:w]
		}
		for len(line) < w {
			line = append(line, ' ')
		}
		bgRunes[y] = line
	}

	popupLines := strings.Split(popup, "\n")
	popupH := len(popupLines)
	popupW := 0
	for _, l := range popupLines {
		if vw := lipgloss.Width(l); vw > popupW {
			popupW = vw
		}
	}

	startY := (h - popupH) / 2
	startX := (w - popupW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	dim := lipgloss.NewStyle().Foreground(s.Subtle).Faint(true)

	result := make([]string, h)
	for y := 0; y < h; y++ {
		popupY := y - startY
		if popupY >= 0 && popupY < popupH {
			leftEnd := startX
			if leftEnd > len(bgRunes[y]) {
				leftEnd = len(bgRunes[y])
			}
			prefix := dim.Render(string(bgRunes[y][:leftEnd]))
			rightStart := startX + popupW
			suffix := ""
			if rightStart < len(bgRunes[y]) {
				suffix = dim.Render(string(bgRunes[y][rightStart:]))
			}
			result[y] = prefix + popupLines[popupY] + suffix
		} else {
			result[y] = dim.Render(string(bgRunes[y]))
		}
	}

	return strings.Join(result, "\n")
}
