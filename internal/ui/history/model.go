package history

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/util"
)

type panel int

const (
	panelRequest panel = iota
	panelResponse
)

type Model struct {
	database     *db.DB
	entries      []db.Entry
	cursor       int
	focusedPanel panel

	listViewport viewport.Model
	bodyViewport viewport.Model
	pager        paginator.Model
	help         help.Model

	searchInput    textinput.Model
	searchKind     searchKind
	searchAccepted bool
	searchErr      string

	width  int
	height int
}

func New() Model {
	ti := textinput.New()
	ti.Prompt = ""
	return Model{
		listViewport: ilovetui.NewViewport(),
		bodyViewport: ilovetui.NewViewport(),
		pager:        ilovetui.NewPaginator(),
		help:         ilovetui.NewHelp(),
		searchInput:  ti,
	}
}

func (m Model) IsEditing() bool {
	return m.searchKind != searchKindOff && !m.searchAccepted
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

func (m Model) IsResponseFocused() bool {
	return m.focusedPanel == panelResponse
}

func (m Model) CurrentScheme() string {
	if len(m.entries) == 0 || m.cursor >= len(m.entries) {
		return "https"
	}
	return util.InferScheme(m.entries[m.cursor].Host)
}

// RefreshCmd returns the appropriate load command given the current search state.
// The app model should call this instead of LoadEntriesCmd directly so that
// background refreshes re-run the active search rather than resetting it.
func (m Model) RefreshCmd() tea.Cmd {
	switch m.searchKind {
	case searchKindFulltext:
		return SearchCmd(m.database, m.searchInput.Value())
	case searchKindSQL:
		return nil
	default:
		return LoadEntriesCmd(m.database)
	}
}

func (m *Model) clearSearch() tea.Cmd {
	m.searchKind = searchKindOff
	m.searchAccepted = false
	m.searchErr = ""
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	m.recalcSizes()
	return LoadEntriesCmd(m.database)
}

func (m *Model) acceptSearch() {
	m.searchAccepted = true
	m.searchInput.Blur()
	m.recalcSizes()
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) SetDB(d *db.DB) {
	m.database = d
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.recalcSizes()
}

func (m *Model) recalcSizes() {
	m.help.SetWidth(m.width - 2)
	// 2 (padding) + 2 (prefix char + space)
	m.searchInput.SetWidth(m.width - 4)

	listH, bodyH := ilovetui.SplitH(m.height, m.renderStatusBar(), 0.35)

	inner := m.width - 2
	if inner < 0 {
		inner = 0
	}

	listVH := ilovetui.ContentHeight(listH) - 1 // -1 for the pager dots row
	if listVH < 0 {
		listVH = 0
	}
	m.listViewport.SetWidth(inner)
	m.listViewport.SetHeight(listVH)
	m.pager.PerPage = listVH
	if m.pager.PerPage < 1 {
		m.pager.PerPage = 1
	}

	bodyVH := ilovetui.ContentHeight(bodyH)
	m.bodyViewport.SetWidth(inner)
	m.bodyViewport.SetHeight(bodyVH)

	m.refreshListViewport()
	m.refreshBody()
}

type historyKeyMap struct{ width int }

func (historyKeyMap) ShortHelp() []key.Binding {
	h := keys.Keys.History
	g := keys.Keys.Global
	return []key.Binding{
		g.Up, g.Down, g.CycleFocus,
		h.DeleteEntry, h.DeleteAll,
		g.Help,
	}
}

func (m historyKeyMap) FullHelp() [][]key.Binding {
	h := keys.Keys.History
	g := keys.Keys.Global
	pageGlobals := []key.Binding{g.Up, g.Down, g.CycleFocus, g.ScrollUp, g.ScrollDown, g.Left, g.Right, g.Escape, g.SendToReplay, g.SendToDiff, g.Copy, g.CopyAs}
	all := []key.Binding{h.Flag, h.DeleteEntry, h.DeleteAll, h.Filter, h.SqlQuery}
	all = append(all, pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
