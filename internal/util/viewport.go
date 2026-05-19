package util

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// ScrollViewport scrolls vp vertically by half its height.
// delta should be -1 for up, +1 for down.
func ScrollViewport(vp *viewport.Model, delta int) {
	step := vp.Height() / 2
	if step < 1 {
		step = 1
	}
	vp.SetYOffset(vp.YOffset() + delta*step)
}

// HandleMouseWheel applies standard mouse wheel scrolling to vp.
// Vertical: one line at a time. Shift+vertical or horizontal: scroll 6 columns.
func HandleMouseWheel(msg tea.MouseWheelMsg, vp *viewport.Model) {
	switch msg.Button {
	case tea.MouseWheelUp:
		if msg.Mod.Contains(tea.ModShift) {
			vp.ScrollLeft(6)
		} else {
			vp.SetYOffset(vp.YOffset() - 1)
		}
	case tea.MouseWheelDown:
		if msg.Mod.Contains(tea.ModShift) {
			vp.ScrollRight(6)
		} else {
			vp.SetYOffset(vp.YOffset() + 1)
		}
	case tea.MouseWheelLeft:
		vp.ScrollLeft(6)
	case tea.MouseWheelRight:
		vp.ScrollRight(6)
	}
}
