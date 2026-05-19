package replay

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

type SendToReplayMsg struct {
	Scheme     string
	Host       string
	RequestRaw string
}

type Entry struct {
	DBID        int64
	Scheme      string
	Host        string
	Path        string
	Method      string
	OriginalRaw string
	RequestRaw  string // current (possibly edited) request
	ResponseRaw string // filled after send
	StatusCode  int    // 0 = not sent yet
	Sending     bool
	Err         error
}

type panel int

const (
	panelList panel = iota
	panelRequest
	panelResponse
)

type Model struct {
	entries      []Entry
	cursor       int
	editing      bool
	focusedPanel panel
	database     *db.DB

	listViewport     viewport.Model
	requestViewport  viewport.Model
	responseViewport viewport.Model
	textarea         textarea.Model
	pager            paginator.Model
	help             help.Model

	width  int
	height int
}

func New() Model {
	ta := style.NewTextarea(false)
	ta.Blur()
	return Model{
		listViewport:     style.NewViewport(),
		requestViewport:  style.NewViewport(),
		responseViewport: style.NewViewport(),
		textarea:         ta,
		pager:            style.NewPaginator(),
		help:             style.NewHelp(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) IsEditing() bool { return m.editing }

func (m Model) IsResponseFocused() bool {
	return m.focusedPanel == panelResponse
}

func (m Model) CurrentRaw() string {
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		return ""
	}
	if m.focusedPanel == panelResponse {
		return m.entries[m.cursor].ResponseRaw
	}
	return m.entries[m.cursor].RequestRaw
}

func (m Model) CurrentScheme() string {
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		return "https"
	}
	if s := m.entries[m.cursor].Scheme; s != "" {
		return s
	}
	return "https"
}

func (m *Model) SetDB(d *db.DB) {
	m.database = d
	if d == nil {
		return
	}
	entries, err := d.ListReplayEntries()
	if err != nil {
		return
	}
	for _, dbe := range entries {
		m.entries = append(m.entries, entryFromDB(dbe))
	}
	m.pager.SetTotalPages(len(m.entries))
	if len(m.entries) > 0 {
		m.cursor = len(m.entries) - 1
	}
	m.refreshListViewport()
	m.refreshBody()
}

func entryFromDB(dbe db.ReplayEntry) Entry {
	var err error
	if dbe.ErrorMsg != "" {
		err = fmt.Errorf("%s", dbe.ErrorMsg)
	}
	return Entry{
		DBID:        dbe.ID,
		Scheme:      dbe.Scheme,
		Host:        dbe.Host,
		Path:        dbe.Path,
		Method:      dbe.Method,
		OriginalRaw: dbe.OriginalRaw,
		RequestRaw:  dbe.RequestRaw,
		ResponseRaw: dbe.ResponseRaw,
		StatusCode:  dbe.StatusCode,
		Err:         err,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.recalcSizes()
}

func (m *Model) recalcSizes() {
	m.help.SetWidth(m.width - 2)

	listH, bodyH := style.SplitH(m.height, m.renderStatusBar(), 0.35)

	listInner := m.width - 2
	if listInner < 0 {
		listInner = 0
	}
	listVH := style.PanelContentH(listH) - 1 // -1 for the pager dots row
	if listVH < 0 {
		listVH = 0
	}
	m.listViewport.SetWidth(listInner)
	m.listViewport.SetHeight(listVH)
	m.pager.PerPage = listVH
	if m.pager.PerPage < 1 {
		m.pager.PerPage = 1
	}

	leftW, rightW := m.bodyHalfWidths()
	leftInner := leftW - 2
	rightInner := rightW - 2
	if leftInner < 0 {
		leftInner = 0
	}
	if rightInner < 0 {
		rightInner = 0
	}
	bodyVH := style.PanelContentH(bodyH)

	m.requestViewport.SetWidth(leftInner)
	m.requestViewport.SetHeight(bodyVH)
	m.responseViewport.SetWidth(rightInner)
	m.responseViewport.SetHeight(bodyVH)
	m.textarea.SetWidth(leftInner)
	m.textarea.SetHeight(bodyVH)

	m.refreshListViewport()
	m.refreshBody()
}

func (m *Model) bodyHalfWidths() (left, right int) {
	left = m.width / 2
	right = m.width - left
	return
}

type replayKeyMap struct{ width int }

func (replayKeyMap) ShortHelp() []key.Binding {
	g := keys.Keys.Global
	r := keys.Keys.Replay
	return []key.Binding{g.Up, g.Down, g.CycleFocus, r.Send, r.Edit, g.Help}
}

func (m replayKeyMap) FullHelp() [][]key.Binding {
	g := keys.Keys.Global
	pageGlobals := []key.Binding{g.Up, g.Down, g.CycleFocus, g.ScrollUp, g.ScrollDown, g.Left, g.Right, g.Escape, g.Copy, g.CopyAs, g.SendToDiff}
	all := append(keys.Keys.Replay.Bindings(), pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
