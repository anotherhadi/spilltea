package replay

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	"github.com/anotherhadi/spilltea/internal/util"
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
					m.responseViewport.SetYOffset(m.responseViewport.YOffset() - 1)
				}
			case tea.MouseWheelDown:
				if msg.Mod.Contains(tea.ModShift) {
					m.requestViewport.ScrollRight(6)
					m.responseViewport.ScrollRight(6)
				} else {
					m.responseViewport.SetYOffset(m.responseViewport.YOffset() + 1)
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
		if m.cursor > 0 {
			m.cursor--
			m.refreshListViewport()
			m.refreshBody()
		}

	case key.Matches(msg, g.Down):
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.refreshListViewport()
			m.refreshBody()
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
			return m, util.OpenExternalEditor(m.entries[m.cursor].RequestRaw)
		}

	case key.Matches(msg, r.UndoEdits):
		if len(m.entries) > 0 {
			m.entries[m.cursor].RequestRaw = m.entries[m.cursor].OriginalRaw
			m.refreshBody()
		}

	case key.Matches(msg, g.ScrollUp):
		step := m.responseViewport.Height() / 2
		if step < 1 {
			step = 1
		}
		m.responseViewport.SetYOffset(m.responseViewport.YOffset() - step)

	case key.Matches(msg, g.ScrollDown):
		step := m.responseViewport.Height() / 2
		if step < 1 {
			step = 1
		}
		m.responseViewport.SetYOffset(m.responseViewport.YOffset() + step)

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
		m.responseViewport.SetContent(lipgloss.Place(m.responseViewport.Width(), m.responseViewport.Height(), lipgloss.Center, lipgloss.Center, style.S.Faint.Render("  (ﾉ◕ヮ◕)ﾉ*:･ﾟ\n   sending...")))
	} else if e.ResponseRaw != "" {
		m.responseViewport.SetContent(style.HighlightHTTP(e.ResponseRaw))
	} else {
		m.responseViewport.SetContent(lipgloss.Place(m.responseViewport.Width(), m.responseViewport.Height(), lipgloss.Center, lipgloss.Center, style.S.Faint.Render("    ( •_•)>⌐■\npress send to fire")))
	}
	m.responseViewport.SetYOffset(0)
	m.responseViewport.SetXOffset(0)
}

func doSend(entry Entry) (responseRaw string, statusCode int, err error) {
	lines := strings.Split(strings.ReplaceAll(entry.RequestRaw, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return "", 0, fmt.Errorf("empty request")
	}

	parts := strings.SplitN(lines[0], " ", 3)
	if len(parts) < 2 {
		return "", 0, fmt.Errorf("invalid request line")
	}
	method := strings.TrimSpace(parts[0])
	path := strings.TrimSpace(parts[1])

	headers := make(http.Header)
	host := entry.Host
	i := 1
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" {
			i++
			break
		}
		if kv := strings.SplitN(line, ": ", 2); len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			if strings.ToLower(k) == "host" {
				host = v
			} else {
				headers.Add(k, v)
			}
		}
		i++
	}

	var bodyBytes []byte
	if i < len(lines) {
		b := strings.Join(lines[i:], "\n")
		b = strings.TrimRight(b, "\n")
		bodyBytes = []byte(b)
	}

	scheme := entry.Scheme
	if scheme == "" {
		scheme = "https"
	}
	urlStr := scheme + "://" + host + path

	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, urlStr, bodyReader)
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

	respBody, _ := io.ReadAll(resp.Body)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %d %s\n", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode))
	sortedKeys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, k := range sortedKeys {
		for _, v := range resp.Header[k] {
			fmt.Fprintf(&sb, "%s: %s\n", k, v)
		}
	}
	sb.WriteString("\n")
	sb.Write(respBody)

	return sb.String(), resp.StatusCode, nil
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
	method, host, path := parseFirstLine(msg.RequestRaw, msg.Host)
	scheme := msg.Scheme
	if scheme == "" {
		scheme = util.InferScheme(host)
	}
	return Entry{
		Scheme:      scheme,
		Host:        host,
		Path:        path,
		Method:      method,
		OriginalRaw: msg.RequestRaw,
		RequestRaw:  msg.RequestRaw,
	}
}

func parseFirstLine(raw, fallbackHost string) (method, host, path string) {
	host = fallbackHost
	path = "/"
	lines := strings.SplitN(raw, "\n", 2)
	if len(lines) == 0 {
		return
	}
	parts := strings.Fields(lines[0])
	if len(parts) >= 1 {
		method = parts[0]
	}
	if len(parts) >= 2 {
		path = parts[1]
	}
	if len(lines) > 1 {
		for _, line := range strings.Split(lines[1], "\n") {
			if strings.HasPrefix(strings.ToLower(line), "host:") {
				host = strings.TrimSpace(line[5:])
				break
			}
		}
	}
	return
}
