package diff

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	notificationsUI "github.com/anotherhadi/spilltea/internal/ui/components/notifications"
)

// SendToDiffMsg carries a raw HTTP request or response to the diff page.
type SendToDiffMsg struct {
	Label string
	Raw   string
}

// DiffReadyMsg is emitted when both slots are filled and the diff is ready to view.
type DiffReadyMsg struct{}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SendToDiffMsg:
		if m.left.raw == "" {
			m.left = slot{label: msg.Label, raw: msg.Raw}
			m.refreshViewports()
			return m, func() tea.Msg {
				return notificationsUI.NotificationMsg{
					Title: "Entry selected",
					Body:  "Select a second entry to compare",
					Kind:  notificationsUI.KindInfo,
				}
			}
		} else if m.right.raw == "" {
			m.right = slot{label: msg.Label, raw: msg.Raw}
			m.computeDiff()
			m.focus = bothSlots
			m.leftViewport.SetYOffset(0)
			m.rightViewport.SetYOffset(0)
			m.leftViewport.SetXOffset(0)
			m.rightViewport.SetXOffset(0)
			m.refreshViewports()
			return m, func() tea.Msg { return DiffReadyMsg{} }
		} else {
			// Both full: reset and start new comparison
			m.left = slot{label: msg.Label, raw: msg.Raw}
			m.right = slot{}
			m.leftLines = nil
			m.rightLines = nil
			m.focus = bothSlots
			m.leftViewport.SetYOffset(0)
			m.rightViewport.SetYOffset(0)
			m.leftViewport.SetXOffset(0)
			m.rightViewport.SetXOffset(0)
			m.refreshViewports()
			return m, func() tea.Msg {
				return notificationsUI.NotificationMsg{
					Title: "Entry replaced",
					Body:  "Select a second entry to compare",
					Kind:  notificationsUI.KindInfo,
				}
			}
		}

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			if msg.Mod.Contains(tea.ModShift) {
				m.scrollH(-6)
			} else {
				m.scroll(-1)
			}
		case tea.MouseWheelDown:
			if msg.Mod.Contains(tea.ModShift) {
				m.scrollH(6)
			} else {
				m.scroll(1)
			}
		case tea.MouseWheelLeft:
			m.scrollH(-6)
		case tea.MouseWheelRight:
			m.scrollH(6)
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Keys.Global.CycleFocus):
			m.focus = m.focus.next()

		case key.Matches(msg, keys.Keys.Global.Up):
			m.scroll(-1)
		case key.Matches(msg, keys.Keys.Global.Down):
			m.scroll(1)
		case key.Matches(msg, keys.Keys.Global.ScrollUp):
			step := m.leftViewport.Height() / 2
			if step < 1 {
				step = 1
			}
			m.scroll(-step)
		case key.Matches(msg, keys.Keys.Global.ScrollDown):
			step := m.leftViewport.Height() / 2
			if step < 1 {
				step = 1
			}
			m.scroll(step)

		case key.Matches(msg, keys.Keys.Global.Left):
			m.scrollH(-6)
		case key.Matches(msg, keys.Keys.Global.Right):
			m.scrollH(6)

		case key.Matches(msg, keys.Keys.Diff.Clear):
			switch m.focus {
			case leftSlot:
				m.left = m.right
				m.right = slot{}
				m.leftLines = nil
				m.rightLines = nil
				m.focus = bothSlots
			case rightSlot:
				m.right = slot{}
				m.leftLines = nil
				m.rightLines = nil
				m.focus = bothSlots
			default:
				m.left = slot{}
				m.right = slot{}
				m.leftLines = nil
				m.rightLines = nil
				m.focus = bothSlots
			}
			m.leftViewport.SetYOffset(0)
			m.rightViewport.SetYOffset(0)
			m.leftViewport.SetXOffset(0)
			m.rightViewport.SetXOffset(0)
			m.refreshViewports()

		case key.Matches(msg, keys.Keys.Global.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.recalcSizes()
		}
	}

	return m, nil
}
