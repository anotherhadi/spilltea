package replay

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
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
	allEntries   []Entry
	entries      []Entry // filtered view; equals allEntries when no filter
	entryIndices []int   // maps entries[i] -> allEntries[i]; nil when no filter
	cursor       int
	editing      bool
	focusedPanel panel
	database     *db.DB

	filterInput    textinput.Model
	filterActive   bool
	filterAccepted bool

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
	ta := ilovetui.NewTextarea(false)
	ta.Blur()
	ti := textinput.New()
	ti.Prompt = ""
	return Model{
		listViewport:     ilovetui.NewViewport(),
		requestViewport:  ilovetui.NewViewport(),
		responseViewport: ilovetui.NewViewport(),
		textarea:         ta,
		filterInput:      ti,
		pager:            ilovetui.NewPaginator(),
		help:             ilovetui.NewHelp(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) IsEditing() bool { return m.editing || (m.filterActive && !m.filterAccepted) }

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
		m.allEntries = append(m.allEntries, entryFromDB(dbe))
	}
	m.entries = m.allEntries
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
	m.filterInput.SetWidth(m.width - 4)

	listH, bodyH := ilovetui.SplitH(m.height, m.renderStatusBar(), 0.35)

	listInner := m.width - 2
	if listInner < 0 {
		listInner = 0
	}
	listVH := ilovetui.ContentHeight(listH) - 1 // -1 for the pager dots row
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
	bodyVH := ilovetui.ContentHeight(bodyH)

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

func (m *Model) currentAllIdx() int {
	if m.entryIndices != nil && m.cursor < len(m.entryIndices) {
		return m.entryIndices[m.cursor]
	}
	if m.cursor < len(m.allEntries) {
		return m.cursor
	}
	return -1
}

func matchesFilter(e Entry, term string) bool {
	return strings.Contains(strings.ToLower(e.Host), term) ||
		strings.Contains(strings.ToLower(e.Path), term) ||
		strings.Contains(strings.ToLower(strings.TrimSpace(e.Method)), term)
}

func (m *Model) applyFilter(preferAllIdx int) {
	term := strings.ToLower(m.filterInput.Value())
	if !m.filterActive || term == "" {
		m.entries = m.allEntries
		m.entryIndices = nil
		if preferAllIdx >= 0 && preferAllIdx < len(m.allEntries) {
			m.cursor = preferAllIdx
		}
	} else {
		m.entries = nil
		m.entryIndices = nil
		newCursor := 0
		found := false
		for i, e := range m.allEntries {
			if matchesFilter(e, term) {
				if i == preferAllIdx && !found {
					newCursor = len(m.entries)
					found = true
				}
				m.entries = append(m.entries, e)
				m.entryIndices = append(m.entryIndices, i)
			}
		}
		if found {
			m.cursor = newCursor
		} else {
			m.cursor = 0
		}
	}
	if len(m.entries) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	m.pager.SetTotalPages(len(m.entries))
	m.refreshListViewport()
	m.refreshBody()
}

func (m *Model) clearFilter() {
	prevAllIdx := m.currentAllIdx()
	m.filterActive = false
	m.filterAccepted = false
	m.filterInput.SetValue("")
	m.filterInput.Blur()
	m.recalcSizes()
	m.applyFilter(prevAllIdx)
}

type replayKeyMap struct{ width int }

func (replayKeyMap) ShortHelp() []key.Binding {
	g := keys.Keys.Global
	r := keys.Keys.Replay
	return []key.Binding{g.Up, g.Down, g.CycleFocus, r.Send, r.Edit, r.Filter, g.Help}
}

func (m replayKeyMap) FullHelp() [][]key.Binding {
	g := keys.Keys.Global
	pageGlobals := []key.Binding{g.Up, g.Down, g.CycleFocus, g.ScrollUp, g.ScrollDown, g.Left, g.Right, g.Escape, g.Copy, g.CopyAs, g.SendToDiff}
	all := append(keys.Keys.Replay.Bindings(), pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
