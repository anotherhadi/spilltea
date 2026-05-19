package docs

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/util"
)

func (e Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	g := keys.Keys.Global
	d := keys.Keys.Docs

	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		util.HandleMouseWheel(msg, &e.viewport)

	case tea.KeyPressMsg:
		if e.searching {
			switch {
			case key.Matches(msg, d.SearchReset):
				e.searching = false
				e.searchInput.Blur()
				e.searchInput.SetValue("")
				e.matches = nil
				e.matchIndex = 0
				e.SetSize(e.width, e.height)
			case msg.String() == "enter":
				e.searching = false
				e.searchInput.Blur()
				e.SetSize(e.width, e.height)
			default:
				var cmd tea.Cmd
				e.searchInput, cmd = e.searchInput.Update(msg)
				e.applySearch()
				return e, cmd
			}
			return e, nil
		}

		switch {
		case key.Matches(msg, d.Search):
			e.searching = true
			e.searchInput.SetValue("")
			e.searchInput.Focus()
			e.SetSize(e.width, e.height)
		case key.Matches(msg, d.SearchReset):
			e.matches = nil
			e.matchIndex = 0
			e.rebuildViewportContent()
		case key.Matches(msg, d.SearchNext):
			e.searchNext()
		case key.Matches(msg, d.SearchPrev):
			e.searchPrev()
		case key.Matches(msg, g.Up):
			e.viewport.SetYOffset(e.viewport.YOffset() - 1)
		case key.Matches(msg, g.Down):
			e.viewport.SetYOffset(e.viewport.YOffset() + 1)
		case key.Matches(msg, g.ScrollUp):
			util.ScrollViewport(&e.viewport, -1)
		case key.Matches(msg, g.ScrollDown):
			util.ScrollViewport(&e.viewport, 1)
		case key.Matches(msg, g.Help):
			e.help.ShowAll = !e.help.ShowAll
			e.SetSize(e.width, e.height)
		}
	}
	return e, nil
}
