package copyas

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	notificationsUI "github.com/anotherhadi/spilltea/internal/ui/components/notifications"
)

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case kp.String() == "enter":
			m.open = false
			if item, ok := m.list.SelectedItem().(formatItem); ok {
				return m, tea.Batch(
					tea.SetClipboard(formatAs(item.id, m.rawRequest, m.scheme)),
					func() tea.Msg {
						return notificationsUI.NotificationMsg{
							Title: "Copied",
							Body:  "Request copied to clipboard",
							Kind:  notificationsUI.KindSuccess,
						}
					},
				)
			}
			return m, nil
		case key.Matches(kp, keys.Keys.Global.Escape):
			if m.list.SettingFilter() {
				break
			}
			m.open = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}
