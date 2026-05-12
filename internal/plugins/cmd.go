package plugins

import tea "charm.land/bubbletea/v2"

func WaitForNotif(mgr *Manager) tea.Cmd {
	return func() tea.Msg {
		return <-mgr.Notifs
	}
}

func WaitForQuit(mgr *Manager) tea.Cmd {
	return func() tea.Msg {
		return PluginQuitMsg{Reason: <-mgr.Quit}
	}
}
