package history

import (
	"fmt"
	"log"
	"net/http"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	ilovetui "github.com/anotherhadi/ilovetui"
	"github.com/anotherhadi/spilltea/internal/db"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
	diffUI "github.com/anotherhadi/spilltea/internal/ui/diff"
	replayUI "github.com/anotherhadi/spilltea/internal/ui/replay"
	"github.com/anotherhadi/spilltea/internal/util"
)

type EntriesLoadedMsg struct {
	Entries []db.Entry
}

func LoadEntriesCmd(database *db.DB) tea.Cmd {
	return func() tea.Msg {
		if database == nil {
			return EntriesLoadedMsg{}
		}
		entries, _ := database.ListEntries()
		return EntriesLoadedMsg{Entries: entries}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case EntriesLoadedMsg:
		// Ignore background reloads while a search is active (but not during a mode switch reset).
		if m.searchKind != searchKindOff && (m.searchAccepted || m.searchInput.Value() != "") {
			return m, nil
		}
		// Remember the selected entry's ID so we can re-anchor after the list is
		// reloaded (new entries are prepended; a pure index-based cursor would
		// silently jump to a different entry).
		var selectedID int64
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			selectedID = m.entries[m.cursor].ID
		}
		m.entries = msg.Entries
		entryChanged := true
		if selectedID != 0 {
			for i, e := range m.entries {
				if e.ID == selectedID {
					m.cursor = i
					entryChanged = false
					break
				}
			}
		}
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
			entryChanged = true
		}
		if m.cursor < 0 {
			m.cursor = 0
			entryChanged = true
		}
		m.pager.SetTotalPages(len(m.entries))
		m.refreshListViewport()
		m.refreshBody()
		if entryChanged {
			m.bodyViewport.SetYOffset(0)
			m.bodyViewport.SetXOffset(0)
		}

	case SearchResultMsg:
		m.entries = msg.Entries
		m.cursor = 0
		m.searchErr = ""
		m.pager.SetTotalPages(len(m.entries))
		m.refreshListViewport()
		m.refreshBody()
		m.bodyViewport.SetYOffset(0)
		m.bodyViewport.SetXOffset(0)
		if m.searchKind == searchKindSQL {
			m.acceptSearch()
		}

	case SearchErrMsg:
		m.searchErr = msg.Err.Error()
		m.entries = nil
		m.pager.SetTotalPages(0)
		m.refreshListViewport()
		m.refreshBody()
		m.bodyViewport.SetYOffset(0)
		m.bodyViewport.SetXOffset(0)

	case tea.MouseWheelMsg:
		util.HandleMouseWheel(msg, &m.bodyViewport)

	case tea.KeyPressMsg:
		h := keys.Keys.History
		g := keys.Keys.Global

		if m.searchKind != searchKindOff && !m.searchAccepted {
			// Actively typing: only search navigation + accept/cancel.
			switch {
			case key.Matches(msg, g.Escape):
				return m, m.clearSearch()

			case msg.String() == "enter":
				if m.searchKind == searchKindSQL {
					return m, SQLCmd(m.database, m.searchInput.Value())
				}
				m.acceptSearch()

			case key.Matches(msg, g.Up):
				if m.cursor > 0 {
					m.cursor--
					m.refreshListViewport()
					m.refreshBody()
					m.bodyViewport.SetYOffset(0)
					m.bodyViewport.SetXOffset(0)
				}

			case key.Matches(msg, g.Down):
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					m.refreshListViewport()
					m.refreshBody()
					m.bodyViewport.SetYOffset(0)
					m.bodyViewport.SetXOffset(0)
				}

			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				if m.searchKind == searchKindFulltext {
					return m, tea.Batch(cmd, SearchCmd(m.database, m.searchInput.Value()))
				}
				return m, cmd
			}
			return m, nil
		}

		if m.searchKind != searchKindOff && m.searchAccepted {
			// Filter accepted: Escape clears, all other shortcuts fall through.
			if key.Matches(msg, g.Escape) {
				return m, m.clearSearch()
			}
		}

		switch {
		case key.Matches(msg, keys.Keys.History.Filter):
			prev := m.searchKind
			m.searchKind = searchKindFulltext
			m.searchAccepted = false
			m.searchInput.Placeholder = "filter requests..."
			m.searchErr = ""
			m.searchInput.Focus()
			m.recalcSizes()
			if prev != searchKindFulltext {
				m.searchInput.SetValue("")
				return m, LoadEntriesCmd(m.database)
			}

		case key.Matches(msg, keys.Keys.History.SqlQuery):
			prev := m.searchKind
			m.searchKind = searchKindSQL
			m.searchAccepted = false
			m.searchInput.Placeholder = "status_code = 200 AND host LIKE '%.api.%'"
			m.searchErr = ""
			m.searchInput.Focus()
			m.recalcSizes()
			if prev != searchKindSQL {
				m.searchInput.SetValue("")
				return m, LoadEntriesCmd(m.database)
			}

		case key.Matches(msg, g.Up):
			if m.cursor > 0 {
				m.cursor--
				m.refreshListViewport()
				m.refreshBody()
				m.bodyViewport.SetYOffset(0)
				m.bodyViewport.SetXOffset(0)
			}

		case key.Matches(msg, g.Down):
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.refreshListViewport()
				m.refreshBody()
				m.bodyViewport.SetYOffset(0)
				m.bodyViewport.SetXOffset(0)
			}

		case key.Matches(msg, g.CycleFocus):
			if m.focusedPanel == panelRequest {
				m.focusedPanel = panelResponse
			} else {
				m.focusedPanel = panelRequest
			}
			m.refreshBody()
			m.bodyViewport.SetYOffset(0)
			m.bodyViewport.SetXOffset(0)

		case key.Matches(msg, g.SendToReplay):
			if len(m.entries) > 0 {
				e := m.entries[m.cursor]
				scheme := util.InferScheme(e.Host)
				return m, func() tea.Msg {
					return replayUI.SendToReplayMsg{
						Scheme:     scheme,
						Host:       e.Host,
						RequestRaw: e.RequestRaw,
					}
				}
			}

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
				return m, func() tea.Msg {
					return diffUI.SendToDiffMsg{Label: label, Raw: raw}
				}
			}

		case key.Matches(msg, h.Flag):
			if len(m.entries) > 0 && m.database != nil {
				if err := m.database.ToggleFlag(m.entries[m.cursor].ID); err != nil {
					log.Printf("history: toggle flag: %v", err)
				}
				return m, m.RefreshCmd()
			}

		case key.Matches(msg, h.DeleteEntry):
			if len(m.entries) > 0 {
				id := m.entries[m.cursor].ID
				if m.database != nil {
					if err := m.database.DeleteEntry(id); err != nil {
						log.Printf("history: delete entry: %v", err)
					}
				}
				return m, LoadEntriesCmd(m.database)
			}

		case key.Matches(msg, h.DeleteAll):
			if m.database != nil {
				if m.searchKind != searchKindOff {
					hasUnflagged := false
					for _, e := range m.entries {
						if !e.Flagged {
							hasUnflagged = true
							break
						}
					}
					for _, e := range m.entries {
						if hasUnflagged && e.Flagged {
							continue
						}
						if err := m.database.DeleteEntry(e.ID); err != nil {
							log.Printf("history: delete entry: %v", err)
						}
					}
				} else {
					if err := m.database.DeleteAllExceptFlagged(); err != nil {
						log.Printf("history: delete all unflagged: %v", err)
					}
				}
			}
			return m, m.clearSearch()

		case key.Matches(msg, g.ScrollUp):
			util.ScrollViewport(&m.bodyViewport, -1)

		case key.Matches(msg, g.ScrollDown):
			util.ScrollViewport(&m.bodyViewport, 1)

		case key.Matches(msg, g.Left):
			m.bodyViewport.ScrollLeft(6)

		case key.Matches(msg, g.Right):
			m.bodyViewport.ScrollRight(6)

		case key.Matches(msg, g.GotoTop):
			m.cursor = 0
			m.pager.Page = 0
			m.refreshListViewport()
			m.refreshBody()
			m.bodyViewport.SetYOffset(0)
			m.bodyViewport.SetXOffset(0)

		case key.Matches(msg, g.GotoBottom):
			m.cursor = util.CursorGotoBottom(len(m.entries))
			m.refreshListViewport()
			m.refreshBody()
			m.bodyViewport.SetYOffset(0)
			m.bodyViewport.SetXOffset(0)

		case key.Matches(msg, g.PrevPage):
			m.cursor = util.CursorMovePage(m.cursor, len(m.entries), m.pager.PerPage, false)
			m.refreshListViewport()
			m.refreshBody()
			m.bodyViewport.SetYOffset(0)
			m.bodyViewport.SetXOffset(0)

		case key.Matches(msg, g.NextPage):
			m.cursor = util.CursorMovePage(m.cursor, len(m.entries), m.pager.PerPage, true)
			m.refreshListViewport()
			m.refreshBody()
			m.bodyViewport.SetYOffset(0)
			m.bodyViewport.SetXOffset(0)

		case key.Matches(msg, keys.Keys.Global.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.recalcSizes()
		}
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
		m.bodyViewport.SetContent("")
		return
	}
	e := m.entries[m.cursor]
	var raw string
	if m.focusedPanel == panelResponse {
		raw = e.ResponseRaw
	} else {
		raw = e.RequestRaw
	}
	if raw == "" {
		w, h := m.bodyViewport.Width(), m.bodyViewport.Height()
		m.bodyViewport.SetContent(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, ilovetui.S.Faint.Render(util.EmptyState(w, "(˘･_･˘)", "no response stored"))))
		return
	}
	m.bodyViewport.SetContent(style.HighlightHTTP(raw))
}
