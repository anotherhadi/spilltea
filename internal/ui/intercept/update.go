package intercept

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/util"
	diffUI "github.com/anotherhadi/spilltea/internal/ui/diff"
	replayUI "github.com/anotherhadi/spilltea/internal/ui/replay"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case intercept.RequestArrivedMsg:
		if !m.interceptEnabled {
			m.broker.Decide(msg.Req, intercept.Forward)
			break
		}
		wasEmpty := len(m.queue) == 0
		m.queue = append(m.queue, msg.Req)
		m.refreshListViewport()
		if wasEmpty && (!m.captureResponse || m.focusedPanel == panelRequests) {
			m.refreshBody()
		}

	case intercept.ResponseArrivedMsg:
		wasEmpty := len(m.responseQueue) == 0
		m.responseQueue = append(m.responseQueue, msg.Resp)
		m.refreshResponseListViewport()
		if wasEmpty && m.captureResponse && m.focusedPanel == panelResponses {
			m.refreshBody()
		}

	case util.EditorFinishedMsg:
		if msg.Err == nil && msg.Content != "" {
			m.textarea.SetValue(msg.Content)
			m.refreshBodyViewport()
		}

	case tea.MouseWheelMsg:
		if !m.editing {
			switch msg.Button {
			case tea.MouseWheelUp:
				if msg.Mod.Contains(tea.ModShift) {
					m.bodyViewport.ScrollLeft(6)
				} else {
					m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() - 1)
				}
			case tea.MouseWheelDown:
				if msg.Mod.Contains(tea.ModShift) {
					m.bodyViewport.ScrollRight(6)
				} else {
					m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() + 1)
				}
			case tea.MouseWheelLeft:
				m.bodyViewport.ScrollLeft(6)
			case tea.MouseWheelRight:
				m.bodyViewport.ScrollRight(6)
			}
		}

	case tea.KeyPressMsg:
		if m.editing {
			return m.updateEditMode(msg, &cmds)
		}
		return m.updateNormalMode(msg, &cmds)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateNormalMode(msg tea.KeyPressMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	onResponses := m.captureResponse && m.focusedPanel == panelResponses

	switch {
	case key.Matches(msg, keys.Keys.Global.Up):
		if onResponses {
			if m.responseCursor > 0 {
				m.responseCursor--
				m.refreshResponseListViewport()
				m.refreshBody()
			}
		} else {
			if m.cursor > 0 {
				m.cursor--
				m.refreshListViewport()
				m.refreshBody()
			}
		}

	case key.Matches(msg, keys.Keys.Global.Down):
		if onResponses {
			if m.responseCursor < len(m.responseQueue)-1 {
				m.responseCursor++
				m.refreshResponseListViewport()
				m.refreshBody()
			}
		} else {
			if m.cursor < len(m.queue)-1 {
				m.cursor++
				m.refreshListViewport()
				m.refreshBody()
			}
		}

	case key.Matches(msg, keys.Keys.Global.CycleFocus):
		if m.captureResponse {
			if m.focusedPanel == panelRequests {
				m.focusedPanel = panelResponses
			} else {
				m.focusedPanel = panelRequests
			}
			m.refreshBody()
		}

	case key.Matches(msg, keys.Keys.Global.ScrollUp):
		step := m.bodyViewport.Height() / 2
		if step < 1 {
			step = 1
		}
		m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() - step)

	case key.Matches(msg, keys.Keys.Global.ScrollDown):
		step := m.bodyViewport.Height() / 2
		if step < 1 {
			step = 1
		}
		m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() + step)

	case key.Matches(msg, keys.Keys.Global.Left):
		m.bodyViewport.ScrollLeft(6)

	case key.Matches(msg, keys.Keys.Global.Right):
		m.bodyViewport.ScrollRight(6)

	case key.Matches(msg, keys.Keys.Global.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Keys.Intercept.UndoEdits):
		if onResponses {
			if len(m.responseQueue) > 0 {
				delete(m.pendingResponseEdits, m.responseQueue[m.responseCursor])
				m.refreshBody()
			}
		} else {
			if len(m.queue) > 0 {
				delete(m.pendingEdits, m.queue[m.cursor])
				m.refreshBody()
			}
		}

	case key.Matches(msg, keys.Keys.Intercept.ToggleIntercept):
		m.interceptEnabled = !m.interceptEnabled
		if !m.interceptEnabled {
			for len(m.queue) > 0 {
				m.applyAndDecide(intercept.Forward)
			}
		}

	case key.Matches(msg, keys.Keys.Intercept.CaptureResponse):
		m.captureResponse = !m.captureResponse
		m.broker.SetCaptureResponse(m.captureResponse)
		if !m.captureResponse {
			for len(m.responseQueue) > 0 {
				m.broker.DecideResponse(m.responseQueue[0], intercept.Forward)
				m.responseQueue = m.responseQueue[1:]
			}
			m.responseCursor = 0
			m.focusedPanel = panelRequests
		}
		m.recalcSizes()

	case key.Matches(msg, keys.Keys.Global.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.recalcSizes()

	case key.Matches(msg, keys.Keys.Intercept.Forward):
		if onResponses {
			m.applyAndDecideResponse(intercept.Forward)
		} else {
			m.applyAndDecide(intercept.Forward)
		}

	case key.Matches(msg, keys.Keys.Intercept.ForwardAll):
		if onResponses {
			for len(m.responseQueue) > 0 {
				m.applyAndDecideResponse(intercept.Forward)
			}
		} else {
			for len(m.queue) > 0 {
				m.applyAndDecide(intercept.Forward)
			}
		}

	case key.Matches(msg, keys.Keys.Intercept.Drop):
		if onResponses {
			m.applyAndDecideResponse(intercept.Drop)
		} else {
			m.applyAndDecide(intercept.Drop)
		}

	case key.Matches(msg, keys.Keys.Intercept.DropAll):
		if onResponses {
			for len(m.responseQueue) > 0 {
				m.applyAndDecideResponse(intercept.Drop)
			}
		} else {
			for len(m.queue) > 0 {
				m.applyAndDecide(intercept.Drop)
			}
		}

	case key.Matches(msg, keys.Keys.Intercept.Edit):
		hasItem := (!onResponses && len(m.queue) > 0) || (onResponses && len(m.responseQueue) > 0)
		if hasItem {
			raw := m.CurrentRaw()
			if len(raw) > maxInlineEditBytes {
				return m, util.OpenExternalEditor(raw)
			}
			m.loadIntoTextarea()
			m.editing = true
			m.textarea.Focus()
		}

	case key.Matches(msg, keys.Keys.Intercept.EditExternal):
		if !onResponses && len(m.queue) > 0 {
			return m, util.OpenExternalEditor(formatRawRequest(m.queue[m.cursor]))
		}
		if onResponses && len(m.responseQueue) > 0 {
			return m, util.OpenExternalEditor(formatRawResponse(m.responseQueue[m.responseCursor]))
		}

	case key.Matches(msg, keys.Keys.Global.SendToReplay):
		if !onResponses && len(m.queue) > 0 {
			req := m.queue[m.cursor]
			raw := m.CurrentRaw()
			scheme := req.Flow.Request.URL.Scheme
			if scheme == "" {
				scheme = "https"
			}
			return m, func() tea.Msg {
				return replayUI.SendToReplayMsg{
					Scheme:     scheme,
					Host:       req.Flow.Request.URL.Host,
					RequestRaw: raw,
				}
			}
		}

	case key.Matches(msg, keys.Keys.Global.SendToDiff):
		raw := m.CurrentRaw()
		if raw != "" {
			label := m.currentLabel()
			return m, func() tea.Msg {
				return diffUI.SendToDiffMsg{Label: label, Raw: raw}
			}
		}
	}

	return m, tea.Batch(*cmds...)
}

func (m Model) updateEditMode(msg tea.KeyPressMsg, cmds *[]tea.Cmd) (tea.Model, tea.Cmd) {
	onResponses := m.captureResponse && m.focusedPanel == panelResponses

	switch {
	case key.Matches(msg, keys.Keys.Global.Escape):
		m.saveCurrentEdit()
		m.editing = false
		m.textarea.Blur()
		m.refreshBodyViewport()

	case key.Matches(msg, keys.Keys.Intercept.UndoEdits):
		if onResponses {
			if len(m.responseQueue) > 0 {
				delete(m.pendingResponseEdits, m.responseQueue[m.responseCursor])
				m.textarea.SetValue(formatRawResponse(m.responseQueue[m.responseCursor]))
			}
		} else {
			if len(m.queue) > 0 {
				delete(m.pendingEdits, m.queue[m.cursor])
				m.textarea.SetValue(formatRawRequest(m.queue[m.cursor]))
			}
		}

	default:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		*cmds = append(*cmds, cmd)
	}

	return m, tea.Batch(*cmds...)
}
