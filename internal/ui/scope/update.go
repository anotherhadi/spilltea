package scope

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	kp, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		return m, nil
	}

	if key.Matches(kp, keys.Keys.Global.CycleFocus) {
		return m.cycleFocus()
	}

	if key.Matches(kp, keys.Keys.Global.Help) && !m.IsEditing() {
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	}

	switch m.focusIdx {
	case fieldWhitelist:
		if key.Matches(kp, keys.Keys.Global.Escape) {
			return m.blurAll()
		}
		var cmd tea.Cmd
		m.wlTextarea, cmd = m.wlTextarea.Update(kp)
		return m, cmd

	case fieldBlacklist:
		if key.Matches(kp, keys.Keys.Global.Escape) {
			return m.blurAll()
		}
		var cmd tea.Cmd
		m.blTextarea, cmd = m.blTextarea.Update(kp)
		return m, cmd
	}

	return m, nil
}

func (m Model) blurAll() (tea.Model, tea.Cmd) {
	m.wlTextarea.Blur()
	m.blTextarea.Blur()
	m.focusIdx = fieldNone
	m.syncLayout()
	return m, m.scopeChangedCmd()
}

func (m Model) cycleFocus() (tea.Model, tea.Cmd) {
	scopeCmd := m.scopeChangedCmd()

	var focusCmd tea.Cmd
	switch m.focusIdx {
	case fieldNone, fieldBlacklist:
		m.blTextarea.Blur()
		m.focusIdx = fieldWhitelist
		focusCmd = m.wlTextarea.Focus()
	case fieldWhitelist:
		m.wlTextarea.Blur()
		m.focusIdx = fieldBlacklist
		focusCmd = m.blTextarea.Focus()
	}

	m.syncLayout()
	return m, tea.Batch(focusCmd, scopeCmd)
}
