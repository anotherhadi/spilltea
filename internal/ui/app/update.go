package app

import (
	"log"
	"os/exec"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/plugins"
	proxyPkg "github.com/anotherhadi/spilltea/internal/proxy"
	copyUI "github.com/anotherhadi/spilltea/internal/ui/components/copy"
	copyasUI "github.com/anotherhadi/spilltea/internal/ui/components/copyas"
	notificationsUI "github.com/anotherhadi/spilltea/internal/ui/components/notifications"
	diffUI "github.com/anotherhadi/spilltea/internal/ui/diff"
	findingsUI "github.com/anotherhadi/spilltea/internal/ui/findings"
	historyUI "github.com/anotherhadi/spilltea/internal/ui/history"
	interceptUI "github.com/anotherhadi/spilltea/internal/ui/intercept"
	replayUI "github.com/anotherhadi/spilltea/internal/ui/replay"
	"github.com/anotherhadi/spilltea/internal/util"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Broker messages must always re-register their watchers
	switch msg := msg.(type) {
	case notificationsUI.NotificationMsg:
		var cmd tea.Cmd
		m.notifications, cmd = m.notifications.Update(msg)
		return m, cmd
	case notificationsUI.DismissMsg:
		var cmd tea.Cmd
		m.notifications, cmd = m.notifications.Update(msg)
		return m, cmd
	case intercept.RequestArrivedMsg:
		updated, cmd := m.intercept.Update(msg)
		m.intercept = updated.(interceptUI.Model)
		if m.page == pageIntercept {
			m.intercept.ClearUnread()
		}
		return m, tea.Batch(cmd, intercept.WaitForRequest(m.broker))
	case intercept.ResponseArrivedMsg:
		updated, cmd := m.intercept.Update(msg)
		m.intercept = updated.(interceptUI.Model)
		return m, tea.Batch(cmd, intercept.WaitForResponse(m.broker))

	case plugins.PluginNotifMsg:
		cmd := plugins.WaitForNotif(m.pluginManager)
		kind := notificationsUI.KindInfo
		switch msg.Kind {
		case "success":
			kind = notificationsUI.KindSuccess
		case "warning":
			kind = notificationsUI.KindWarning
		case "error":
			kind = notificationsUI.KindError
		}
		notifCmd := func() tea.Msg {
			return notificationsUI.NotificationMsg{
				Title: msg.Title,
				Body:  msg.Body,
				Kind:  kind,
			}
		}
		return m, tea.Batch(cmd, notifCmd)

	case plugins.PluginQuitMsg:
		log.Printf("plugin quit: %s", msg.Reason)
		m.pluginManager.RunOnQuit()
		return m, tea.Quit
	}

	if m.copyAs.IsOpen() {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			m.copyAs.SetSize(ws.Width, ws.Height)
			m.resizeChildren()
			return m, nil
		}
		var cmd tea.Cmd
		m.copyAs, cmd = m.copyAs.Update(msg)
		return m, cmd
	}

	if m.copy.IsOpen() {
		if ws, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = ws.Width
			m.height = ws.Height
			m.copy.SetSize(ws.Width, ws.Height)
			m.resizeChildren()
			return m, nil
		}
		var cmd tea.Cmd
		m.copy, cmd = m.copy.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeChildren()

	case proxyPkg.ErrMsg:
		if msg.Err != nil {
			log.Printf("proxy error: %v", msg.Err)
			m.fatalErr = msg.Err
			return m, tea.Quit
		}
		return m, nil

	case tickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, tickCmd())
		if m.page == pageHistory {
			cmds = append(cmds, m.history.RefreshCmd())
		}
		cmds = append(cmds, findingsUI.RefreshCmd(m.database))
		return m, tea.Batch(cmds...)

	case findingsUI.FindingsLoadedMsg:
		updated, cmd := m.findingsPage.Update(msg)
		m.findingsPage = updated.(findingsUI.Model)
		if m.page == pageFindings {
			m.findingsPage.ClearUnread()
		}
		return m, cmd

	case replayUI.SendToReplayMsg:
		updated, cmd := m.replay.Update(msg)
		m.replay = updated.(replayUI.Model)
		if config.Global.Replay.SwitchToPageOnSend {
			m.page = pageReplay
			m.resizeChildren()
		} else {
			return m, tea.Batch(cmd, func() tea.Msg {
				return notificationsUI.NotificationMsg{
					Title: "Replay",
					Body:  "Request queued in replay",
					Kind:  notificationsUI.KindInfo,
				}
			})
		}
		return m, cmd

	case diffUI.SendToDiffMsg:
		updated, cmd := m.diff.Update(msg)
		m.diff = updated.(diffUI.Model)
		return m, cmd

	case diffUI.DiffReadyMsg:
		m.page = pageDiff
		m.resizeChildren()
		return m, nil

	case historyUI.EntriesLoadedMsg:
		updated, cmd := m.history.Update(msg)
		m.history = updated.(historyUI.Model)
		return m, cmd

	case tea.KeyPressMsg:
		if key.Matches(msg, keys.Keys.Global.Quit) && !m.activeIsEditing() {
			m.pluginManager.RunOnQuit()
			return m, tea.Quit
		}

		if key.Matches(msg, keys.Keys.Global.OpenLogs) {
			logPath := filepath.Join(filepath.Dir(m.projectPath), "logs.log")
			return m, tea.ExecProcess(exec.Command(util.ResolveEditor(), logPath), nil) // #nosec G204 G702 -- editor from trusted config/$EDITOR, logPath is a fixed path
		}

		if !m.activeIsEditing() {
			switch {
			case key.Matches(msg, keys.Keys.Global.CopyAs):
				var raw, scheme string
				var responseFocused bool
				switch m.page {
				case pageIntercept:
					raw = m.intercept.CurrentRaw()
					scheme = m.intercept.CurrentScheme()
					responseFocused = m.intercept.IsResponseFocused()
				case pageHistory:
					raw = m.history.CurrentRaw()
					scheme = m.history.CurrentScheme()
					responseFocused = m.history.IsResponseFocused()
				case pageReplay:
					raw = m.replay.CurrentRaw()
					scheme = m.replay.CurrentScheme()
					responseFocused = m.replay.IsResponseFocused()
				}
				if raw != "" && !responseFocused {
					m.copyAs.SetSize(m.width, m.height)
					m.copyAs.Open(copyasUI.OpenMsg{RawRequest: raw, Scheme: scheme})
				}
				return m, nil

			case key.Matches(msg, keys.Keys.Global.Copy):
				if m.page == pageFindings {
					if md := m.findingsPage.CurrentMarkdown(); md != "" {
						return m, tea.Batch(
							tea.SetClipboard(md),
							func() tea.Msg {
								return notificationsUI.NotificationMsg{
									Title: "Copied",
									Body:  "Finding copied to clipboard",
									Kind:  notificationsUI.KindSuccess,
								}
							},
						)
					}
					return m, nil
				}
				var raw, scheme string
				var responseFocused bool
				switch m.page {
				case pageIntercept:
					raw = m.intercept.CurrentRaw()
					scheme = m.intercept.CurrentScheme()
					responseFocused = m.intercept.IsResponseFocused()
				case pageHistory:
					raw = m.history.CurrentRaw()
					scheme = m.history.CurrentScheme()
					responseFocused = m.history.IsResponseFocused()
				case pageReplay:
					raw = m.replay.CurrentRaw()
					scheme = m.replay.CurrentScheme()
					responseFocused = m.replay.IsResponseFocused()
				}
				if raw != "" {
					m.copy.SetSize(m.width, m.height)
					m.copy.Open(copyUI.OpenMsg{RawRequest: raw, Scheme: scheme, ShowURL: !responseFocused})
				}
				return m, nil

			case key.Matches(msg, keys.Keys.Global.ToggleSidebar):
				m.cycleSidebarState()
				m.resizeChildren()

			default:
				if p, ok := pageShortcuts[msg.String()]; ok {
					prev := m.page
					m.page = p
					if p == pageHistory && prev != pageHistory {
						return m, m.history.RefreshCmd()
					}
					if p == pageFindings {
						m.findingsPage.ClearUnread()
						return m, findingsUI.RefreshCmd(m.database)
					}
					if p == pageIntercept {
						m.intercept.ClearUnread()
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m, cmd = m.updateActivePage(msg)
	return m, cmd
}

func (m Model) activeIsEditing() bool {
	for _, e := range pageRegistry {
		if e.id == m.page && e.isEditing != nil {
			return e.isEditing(&m)
		}
	}
	return false
}

func (m Model) updateActivePage(msg tea.Msg) (Model, tea.Cmd) {
	for _, e := range pageRegistry {
		if e.id == m.page && e.update != nil {
			cmd := e.update(&m, msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) resizeChildren() {
	sidebarW := m.getSidebarWidth()
	h := m.height
	for _, e := range pageRegistry {
		if e.resize == nil {
			continue
		}
		e.resize(m, m.width-sidebarW, h)
	}
	m.notifications.SetSize(m.width, m.height)
}
