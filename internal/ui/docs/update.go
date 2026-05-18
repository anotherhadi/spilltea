package docs

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
)

func (e Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	g := keys.Keys.Global
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			e.viewport.SetYOffset(e.viewport.YOffset() - 1)
		case tea.MouseWheelDown:
			e.viewport.SetYOffset(e.viewport.YOffset() + 1)
		}

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, g.Up):
			e.viewport.SetYOffset(e.viewport.YOffset() - 1)
		case key.Matches(msg, g.Down):
			e.viewport.SetYOffset(e.viewport.YOffset() + 1)
		case key.Matches(msg, g.ScrollUp):
			step := e.viewport.Height() / 2
			if step < 1 {
				step = 1
			}
			e.viewport.SetYOffset(e.viewport.YOffset() - step)
		case key.Matches(msg, g.ScrollDown):
			step := e.viewport.Height() / 2
			if step < 1 {
				step = 1
			}
			e.viewport.SetYOffset(e.viewport.YOffset() + step)
		case key.Matches(msg, g.Help):
			e.help.ShowAll = !e.help.ShowAll
			e.SetSize(e.width, e.height)
		}
	}
	return e, nil
}
