package findings

import (
	"log"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case FindingsLoadedMsg:
		if msg.Err != nil {
			log.Printf("findings load error: %v", msg.Err)
			return m, nil
		}
		m.findings = msg.Findings
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
		m.refreshBody()
		return m, nil

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() - 1)
		case tea.MouseWheelDown:
			m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() + 1)
		}
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
		case key.Matches(msg, f.Dismiss):
			if len(m.findings) > 0 && m.database != nil {
				if err := m.database.DismissFinding(m.findings[m.cursor].ID); err != nil {
					log.Printf("dismiss finding: %v", err)
					return m, nil
				}
				return m, RefreshCmd(m.database)
			}
		case key.Matches(msg, g.ScrollUp):
			step := m.bodyViewport.Height() / 2
			if step < 1 {
				step = 1
			}
			m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() - step)
		case key.Matches(msg, g.ScrollDown):
			step := m.bodyViewport.Height() / 2
			if step < 1 {
				step = 1
			}
			m.bodyViewport.SetYOffset(m.bodyViewport.YOffset() + step)
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
