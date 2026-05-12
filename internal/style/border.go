package style

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// PanelContentH returns the usable inner content height for a panel rendered by
// RenderWithTitle. It subtracts the two border lines (top + bottom) from the
// total panel height.
func PanelContentH(totalH int) int {
	h := totalH - 2
	if h < 0 {
		return 0
	}
	return h
}

// RenderWithTitle renders a lipgloss bordered box with a title embedded in the
// top border, matching the border's own foreground color. height is the total
// desired output height (including both border lines).
func RenderWithTitle(border lipgloss.Style, title, content string, width, height int) string {
	boxH := height - 1
	if contentH := boxH - 1; contentH > 0 {
		lines := strings.Split(content, "\n")
		if len(lines) > contentH {
			content = strings.Join(lines[:contentH], "\n")
		}
	}
	box := border.BorderTop(false).Width(width).Height(boxH).Render(content)

	boxWidth := lipgloss.Width(strings.SplitN(box, "\n", 2)[0])
	label := " " + title + " "
	fillW := boxWidth - lipgloss.Width(label) - 2
	if fillW < 0 {
		fillW = 0
	}
	topLine := "╭" + label + strings.Repeat("─", fillW) + "╮"
	topLine = lipgloss.NewStyle().Foreground(border.GetBorderTopForeground()).Render(topLine)

	return lipgloss.JoinVertical(lipgloss.Left, topLine, box)
}
