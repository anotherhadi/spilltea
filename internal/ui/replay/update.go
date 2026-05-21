package replay

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/andybalholm/brotli"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	diffUI "github.com/anotherhadi/spilltea/internal/ui/diff"
	"github.com/anotherhadi/spilltea/internal/util"
	"github.com/klauspost/compress/zstd"
)

type sentMsg struct {
	index       int
	responseRaw string
	statusCode  int
	err         error
}

func sendCmd(entry Entry, index int) tea.Cmd {
	return func() tea.Msg {
		raw, code, err := doSend(entry)
		return sentMsg{index: index, responseRaw: raw, statusCode: code, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Route non-key messages to textarea when editing so internal
	// textarea messages (e.g. clipboard paste) are handled correctly.
	if m.editing {
		if _, ok := msg.(tea.KeyPressMsg); !ok {
			var taCmd tea.Cmd
			m.textarea, taCmd = m.textarea.Update(msg)
			cmds = append(cmds, taCmd)
		}
	}

	switch msg := msg.(type) {
	case SendToReplayMsg:
		entry := entryFromMsg(msg)
		if m.database != nil {
			id, err := m.database.InsertReplayEntry(entryToDB(entry))
			if err == nil {
				entry.DBID = id
			}
		}
		m.entries = append(m.entries, entry)
		m.cursor = len(m.entries) - 1
		m.pager.SetTotalPages(len(m.entries))
		m.refreshListViewport()
		m.refreshBody()

	case sentMsg:
		if msg.index >= 0 && msg.index < len(m.entries) {
			e := &m.entries[msg.index]
			e.Sending = false
			e.StatusCode = msg.statusCode
			e.ResponseRaw = msg.responseRaw
			if msg.err != nil {
				e.Err = msg.err
				e.ResponseRaw = "Error: " + msg.err.Error()
			}
			if m.database != nil && e.DBID != 0 {
				m.database.UpdateReplayEntry(entryToDB(*e))
			}
		}
		m.refreshListViewport()
		m.refreshBody()

	case util.EditorFinishedMsg:
		if msg.Err == nil && msg.Content != "" && len(m.entries) > 0 {
			m.entries[m.cursor].RequestRaw = msg.Content
			m.refreshBody()
		}

	case tea.MouseWheelMsg:
		if !m.editing {
			switch msg.Button {
			case tea.MouseWheelUp:
				if msg.Mod.Contains(tea.ModShift) {
					m.requestViewport.ScrollLeft(6)
					m.responseViewport.ScrollLeft(6)
				} else {
					m.scrollFocusedViewportVertical(-1)
				}
			case tea.MouseWheelDown:
				if msg.Mod.Contains(tea.ModShift) {
					m.requestViewport.ScrollRight(6)
					m.responseViewport.ScrollRight(6)
				} else {
					m.scrollFocusedViewportVertical(1)
				}
			case tea.MouseWheelLeft:
				m.requestViewport.ScrollLeft(6)
				m.responseViewport.ScrollLeft(6)
			case tea.MouseWheelRight:
				m.requestViewport.ScrollRight(6)
				m.responseViewport.ScrollRight(6)
			}
		}

	case tea.KeyPressMsg:
		if m.editing {
			return m.updateEditMode(msg)
		}
		return m.updateNormalMode(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateNormalMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	g := keys.Keys.Global
	r := keys.Keys.Replay
	switch {
	case key.Matches(msg, g.Up):
		if m.focusedPanel == panelList {
			if m.cursor > 0 {
				m.cursor--
				m.refreshListViewport()
				m.refreshBody()
			}
		} else {
			m.scrollFocusedViewportVertical(-1)
		}

	case key.Matches(msg, g.Down):
		if m.focusedPanel == panelList {
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.refreshListViewport()
				m.refreshBody()
			}
		} else {
			m.scrollFocusedViewportVertical(1)
		}

	case key.Matches(msg, g.CycleFocus):
		switch m.focusedPanel {
		case panelList:
			m.focusedPanel = panelRequest
		case panelRequest:
			m.focusedPanel = panelResponse
		default:
			m.focusedPanel = panelList
		}

	case key.Matches(msg, r.Send):
		if len(m.entries) > 0 && !m.entries[m.cursor].Sending {
			m.entries[m.cursor].Sending = true
			m.entries[m.cursor].ResponseRaw = ""
			m.entries[m.cursor].Err = nil
			m.refreshListViewport()
			m.refreshBody()
			return m, sendCmd(m.entries[m.cursor], m.cursor)
		}

	case key.Matches(msg, r.Edit):
		if len(m.entries) > 0 {
			m.textarea.SetValue(m.entries[m.cursor].RequestRaw)
			m.editing = true
			m.textarea.Focus()
		}

	case key.Matches(msg, r.EditExt):
		if len(m.entries) > 0 {
			if m.focusedPanel == panelResponse {
				return m, util.OpenExternalEditorView(m.entries[m.cursor].ResponseRaw)
			}
			return m, util.OpenExternalEditor(m.entries[m.cursor].RequestRaw)
		}

	case key.Matches(msg, r.UndoEdits):
		if len(m.entries) > 0 {
			m.entries[m.cursor].RequestRaw = m.entries[m.cursor].OriginalRaw
			m.refreshBody()
		}

	case key.Matches(msg, g.ScrollUp):
		vp := m.focusedViewport()
		util.ScrollViewport(&vp, -1)
		m.setFocusedViewport(vp)

	case key.Matches(msg, g.ScrollDown):
		vp := m.focusedViewport()
		util.ScrollViewport(&vp, 1)
		m.setFocusedViewport(vp)

	case key.Matches(msg, g.Left):
		m.requestViewport.ScrollLeft(6)
		m.responseViewport.ScrollLeft(6)

	case key.Matches(msg, g.Right):
		m.requestViewport.ScrollRight(6)
		m.responseViewport.ScrollRight(6)

	case key.Matches(msg, r.Delete):
		if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			if m.database != nil && e.DBID != 0 {
				m.database.DeleteReplayEntry(e.DBID)
			}
			m.entries = append(m.entries[:m.cursor], m.entries[m.cursor+1:]...)
			if m.cursor >= len(m.entries) && m.cursor > 0 {
				m.cursor--
			}
			m.pager.SetTotalPages(len(m.entries))
			m.refreshListViewport()
			m.refreshBody()
		}

	case key.Matches(msg, r.DeleteAll):
		if m.database != nil {
			m.database.DeleteAllReplayEntries()
		}
		m.entries = nil
		m.cursor = 0
		m.pager.SetTotalPages(0)
		m.refreshListViewport()
		m.refreshBody()

	case key.Matches(msg, keys.Keys.Global.GotoTop):
		m.cursor = 0
		m.pager.Page = 0
		m.refreshListViewport()
		m.refreshBody()

	case key.Matches(msg, keys.Keys.Global.GotoBottom):
		m.cursor = util.CursorGotoBottom(len(m.entries))
		m.refreshListViewport()
		m.refreshBody()

	case key.Matches(msg, keys.Keys.Global.PrevPage):
		m.cursor = util.CursorMovePage(m.cursor, len(m.entries), m.pager.PerPage, false)
		m.refreshListViewport()
		m.refreshBody()

	case key.Matches(msg, keys.Keys.Global.NextPage):
		m.cursor = util.CursorMovePage(m.cursor, len(m.entries), m.pager.PerPage, true)
		m.refreshListViewport()
		m.refreshBody()

	case key.Matches(msg, g.SendToDiff):
		if len(m.entries) > 0 {
			e := m.entries[m.cursor]
			var raw, label string
			if m.focusedPanel == panelResponse {
				raw = e.ResponseRaw
				label = fmt.Sprintf("%d %s", e.StatusCode, http.StatusText(e.StatusCode))
			} else {
				raw = e.RequestRaw
				label = e.Method + " " + e.Host + e.Path
			}
			if raw != "" {
				return m, func() tea.Msg {
					return diffUI.SendToDiffMsg{Label: label, Raw: raw}
				}
			}
		}

	case key.Matches(msg, g.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.recalcSizes()
	}

	return m, nil
}

func (m Model) updateEditMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Keys.Global.Escape):
		if len(m.entries) > 0 {
			m.entries[m.cursor].RequestRaw = m.textarea.Value()
		}
		m.editing = false
		m.textarea.Blur()
		m.refreshBody()

	default:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

// focusedViewport returns the viewport that should receive scroll events.
// When the list is focused, scroll targets the request panel.
func (m *Model) focusedViewport() viewport.Model {
	if m.focusedPanel == panelResponse {
		return m.responseViewport
	}
	return m.requestViewport
}

func (m *Model) setFocusedViewport(vp viewport.Model) {
	if m.focusedPanel == panelResponse {
		m.responseViewport = vp
	} else {
		m.requestViewport = vp
	}
}

func (m *Model) scrollFocusedViewportVertical(delta int) {
	vp := m.focusedViewport()
	vp.SetYOffset(vp.YOffset() + delta)
	m.setFocusedViewport(vp)
}

func (m *Model) refreshListViewport() {
	if m.pager.PerPage > 0 {
		if len(m.entries) == 0 {
			m.pager.Page = 0
			m.pager.TotalPages = 0
		} else {
			m.pager.Page = m.cursor / m.pager.PerPage
			m.pager.SetTotalPages(len(m.entries))
		}
	}
	m.listViewport.SetContent(m.renderList())
}

func (m *Model) refreshBody() {
	if len(m.entries) == 0 {
		m.requestViewport.SetContent("")
		m.responseViewport.SetContent("")
		return
	}
	e := m.entries[m.cursor]
	m.requestViewport.SetContent(style.HighlightHTTP(e.RequestRaw))
	m.requestViewport.SetYOffset(0)
	m.requestViewport.SetXOffset(0)

	if e.Sending {
		m.responseViewport.SetContent(lipgloss.Place(m.responseViewport.Width(), m.responseViewport.Height(), lipgloss.Center, lipgloss.Center, style.S.Faint.Render(util.CenterLines("(ﾉ◕ヮ◕)ﾉ*:･ﾟ", "sending..."))))
	} else if e.ResponseRaw != "" {
		m.responseViewport.SetContent(style.HighlightHTTP(e.ResponseRaw))
	} else {
		m.responseViewport.SetContent(lipgloss.Place(m.responseViewport.Width(), m.responseViewport.Height(), lipgloss.Center, lipgloss.Center, style.S.Faint.Render(util.CenterLines("( •_•)>⌐■", "press send to fire"))))
	}
	m.responseViewport.SetYOffset(0)
	m.responseViewport.SetXOffset(0)
}

func doSend(entry Entry) (responseRaw string, statusCode int, err error) {
	parsed := util.ParseRawRequest(entry.RequestRaw)
	if parsed.Method == "" {
		return "", 0, fmt.Errorf("empty request")
	}

	host := parsed.Host
	if host == "" {
		host = entry.Host
	}
	headers := make(http.Header)
	for _, h := range parsed.Headers {
		if strings.EqualFold(h.Key, "host") {
			continue
		}
		headers.Add(h.Key, h.Value)
	}

	scheme := entry.Scheme
	if scheme == "" {
		scheme = "https"
	}
	urlStr := scheme + "://" + host + parsed.Path

	var bodyReader io.Reader
	if parsed.Body != "" {
		bodyReader = strings.NewReader(parsed.Body)
	}

	req, err := http.NewRequest(parsed.Method, urlStr, bodyReader)
	if err != nil {
		return "", 0, err
	}
	req.Header = headers

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	limit := int64(config.Global.App.MaxBodySizeMB) * 1024 * 1024
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, limit))

	if enc := resp.Header.Get("Content-Encoding"); enc != "" {
		if decoded, decErr := decodeBody(enc, respBody); decErr == nil {
			respBody = decoded
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("Content-Length")
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %d %s\n", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode))
	for _, line := range util.SortedHeaderLines(resp.Header) {
		sb.WriteString(line)
	}
	sb.WriteString("\n")
	sb.Write(respBody)

	return sb.String(), resp.StatusCode, nil
}

func decodeBody(encoding string, body []byte) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip":
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(body)))
	case "deflate":
		r, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	case "zstd":
		r, err := zstd.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}
	return nil, fmt.Errorf("unsupported encoding: %s", encoding)
}

func entryToDB(e Entry) db.ReplayEntry {
	errMsg := ""
	if e.Err != nil {
		errMsg = e.Err.Error()
	}
	return db.ReplayEntry{
		ID:          e.DBID,
		Timestamp:   time.Now(),
		Scheme:      e.Scheme,
		Host:        e.Host,
		Path:        e.Path,
		Method:      e.Method,
		OriginalRaw: e.OriginalRaw,
		RequestRaw:  e.RequestRaw,
		ResponseRaw: e.ResponseRaw,
		StatusCode:  e.StatusCode,
		ErrorMsg:    errMsg,
	}
}

func entryFromMsg(msg SendToReplayMsg) Entry {
	parsed := util.ParseRawRequest(msg.RequestRaw)
	host := parsed.Host
	if host == "" {
		host = msg.Host
	}
	scheme := msg.Scheme
	if scheme == "" {
		scheme = util.InferScheme(host)
	}
	return Entry{
		Scheme:      scheme,
		Host:        host,
		Path:        parsed.Path,
		Method:      parsed.Method,
		OriginalRaw: msg.RequestRaw,
		RequestRaw:  msg.RequestRaw,
	}
}
