package findings

import (
	"log"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/util"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case FindingsLoadedMsg:
		if msg.Err != nil {
			log.Printf("findings load error: %v", msg.Err)
			return m, nil
		}
		var prevID int64
		if len(m.findings) > 0 && m.cursor < len(m.findings) {
			prevID = m.findings[m.cursor].ID
		}
		m.findings = msg.Findings
		if len(m.findings) > m.knownCount {
			m.hasUnread = true
		}
		if m.cursor >= len(m.findings) {
			m.cursor = max(0, len(m.findings)-1)
		}
		if len(m.findings) == 0 {
			m.pager.Page = 0
			m.pager.TotalPages = 0
		} else {
			m.pager.SetTotalPages(len(m.findings))
		}
		m.refreshListViewport()
		var newID int64
		if len(m.findings) > 0 && m.cursor < len(m.findings) {
			newID = m.findings[m.cursor].ID
		}
		if newID != prevID {
			m.refreshBody()
		} else {
			m.refreshBodyKeepScroll()
		}
		return m, nil

	case tea.MouseWheelMsg:
		util.HandleMouseWheel(msg, &m.bodyViewport)
		return m, nil

	case tea.KeyPressMsg:
		g := keys.Keys.Global
		f := keys.Keys.Findings

		switch {
		case key.Matches(msg, g.Up):
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.pager.Page*m.pager.PerPage {
					m.pager.PrevPage()
				}
				m.refreshListViewport()
				m.refreshBody()
			}
		case key.Matches(msg, g.Down):
			if m.cursor < len(m.findings)-1 {
				m.cursor++
				if m.cursor >= (m.pager.Page+1)*m.pager.PerPage {
					m.pager.NextPage()
				}
				m.refreshListViewport()
				m.refreshBody()
			}
		case key.Matches(msg, f.Flag):
			if len(m.findings) > 0 && m.database != nil {
				if err := m.database.ToggleFindingFlag(m.findings[m.cursor].ID); err != nil {
					log.Printf("findings: toggle flag: %v", err)
					return m, nil
				}
				return m, RefreshCmd(m.database)
			}
		case key.Matches(msg, f.Dismiss):
			if len(m.findings) > 0 && m.database != nil {
				if err := m.database.DismissFinding(m.findings[m.cursor].ID); err != nil {
					log.Printf("dismiss finding: %v", err)
					return m, nil
				}
				return m, RefreshCmd(m.database)
			}
		case key.Matches(msg, f.DismissAll):
			if m.database != nil {
				if err := m.database.DismissAllFindings(); err != nil {
					log.Printf("findings: dismiss all: %v", err)
					return m, nil
				}
				return m, RefreshCmd(m.database)
			}
		case key.Matches(msg, g.ScrollUp):
			util.ScrollViewport(&m.bodyViewport, -1)
		case key.Matches(msg, g.ScrollDown):
			util.ScrollViewport(&m.bodyViewport, 1)
		case key.Matches(msg, g.GotoTop):
			m.cursor = 0
			m.pager.Page = 0
			m.refreshListViewport()
			m.refreshBody()

		case key.Matches(msg, g.GotoBottom):
			m.cursor = util.CursorGotoBottom(len(m.findings))
			m.pager.Page = util.CursorGotoBottom(m.pager.TotalPages)
			m.refreshListViewport()
			m.refreshBody()

		case key.Matches(msg, g.PrevPage):
			m.cursor = util.CursorMovePage(m.cursor, len(m.findings), m.pager.PerPage, false)
			m.pager.Page = m.cursor / m.pager.PerPage
			m.refreshListViewport()
			m.refreshBody()

		case key.Matches(msg, g.NextPage):
			m.cursor = util.CursorMovePage(m.cursor, len(m.findings), m.pager.PerPage, true)
			m.pager.Page = m.cursor / m.pager.PerPage
			m.refreshListViewport()
			m.refreshBody()

		case key.Matches(msg, g.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.recalcSizes()
		}
	}
	return m, nil
}

func (m *Model) refreshListViewport() {
	m.listViewport.SetContent(m.renderList())
}
