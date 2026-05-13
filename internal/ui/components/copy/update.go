package copy

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
)

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if kp, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case kp.String() == "enter":
			if item, ok := m.list.SelectedItem().(copyItem); ok {
				writeClipboard(m.extract(item.id))
			}
			m.open = false
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
