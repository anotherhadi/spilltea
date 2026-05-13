package plugins

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
)

// PluginsChangedMsg is sent when the plugin list should be refreshed.
type PluginsChangedMsg struct{}

// RefreshCmd returns a command that triggers a list refresh.
func RefreshCmd() tea.Cmd {
	return func() tea.Msg { return PluginsChangedMsg{} }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case PluginsChangedMsg:
		m.Refresh()
		return m, nil
	}

	// Route non-key messages to textarea when editing so internal
	// textarea messages (e.g. clipboard paste) are handled correctly.
	if m.editing {
		if _, ok := msg.(tea.KeyPressMsg); !ok {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		if !m.editing {
			switch msg.Button {
			case tea.MouseWheelUp:
				m.detailViewport.SetYOffset(m.detailViewport.YOffset() - 1)
			case tea.MouseWheelDown:
				m.detailViewport.SetYOffset(m.detailViewport.YOffset() + 1)
			}
		}

	case tea.KeyPressMsg:
		pk := keys.Keys.Plugins
		g := keys.Keys.Global

		// Filtering mode: esc clears+closes, enter just closes, rest goes to filterInput.
		if m.filtering {
			switch {
			case key.Matches(msg, g.Escape):
				m.filtering = false
				m.filter = ""
				m.filterInput.SetValue("")
				m.filterInput.Blur()
				m.applyFilter()
				m.recalcSizes()
			case msg.String() == "enter":
				m.filtering = false
				m.filterInput.Blur()
				m.recalcSizes()
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.filter = m.filterInput.Value()
				m.applyFilter()
				return m, cmd
			}
			return m, nil
		}

		// Editing mode: only esc exits, everything else goes to textarea.
		if m.editing {
			if key.Matches(msg, g.Escape) {
				m.editing = false
				m.textarea.Blur()
				if info, ok := m.selected(); ok && m.manager != nil {
					val := m.textarea.Value()
					m.manager.SaveConfig(info.Name, val)
					// Update cached info.
					m.filtered[m.cursor].ConfigText = val
					for i := range m.items {
						if m.items[i].Name == info.Name {
							m.items[i].ConfigText = val
							break
						}
					}
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, g.Escape):
			if m.filter != "" {
				m.filter = ""
				m.filterInput.SetValue("")
				m.applyFilter()
			}

		case key.Matches(msg, pk.Filter):
			m.filtering = true
			m.filterInput.Focus()
			m.recalcSizes()

		case key.Matches(msg, g.Up):
			if m.cursor > 0 {
				m.cursor--
				m.recalcSizes()
				m.syncTextarea()
				m.detailViewport.GotoTop()
			}

		case key.Matches(msg, g.Down):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.recalcSizes()
				m.syncTextarea()
				m.detailViewport.GotoTop()
			}

		case key.Matches(msg, pk.Toggle):
			if info, ok := m.selected(); ok && m.manager != nil {
				m.manager.TogglePlugin(info.Name)
				m.filtered[m.cursor].Enabled = !info.Enabled
				for i := range m.items {
					if m.items[i].Name == info.Name {
						m.items[i].Enabled = !info.Enabled
						break
					}
				}
				m.refreshListViewport()
			}

		case key.Matches(msg, pk.EditConfig):
			if _, ok := m.selected(); ok && m.hasConfig() {
				m.editing = true
				m.textarea.Focus()
			}

		case key.Matches(msg, g.ScrollUp):
			step := m.detailViewport.Height() / 2
			if step < 1 {
				step = 1
			}
			m.detailViewport.SetYOffset(m.detailViewport.YOffset() - step)

		case key.Matches(msg, g.ScrollDown):
			step := m.detailViewport.Height() / 2
			if step < 1 {
				step = 1
			}
			m.detailViewport.SetYOffset(m.detailViewport.YOffset() + step)

		case key.Matches(msg, g.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.recalcSizes()
		}
	}

	return m, nil
}
