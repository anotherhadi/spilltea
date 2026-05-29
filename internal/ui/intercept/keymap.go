package intercept

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/keys"
)

type interceptKeyMap struct{ width int }

func (interceptKeyMap) ShortHelp() []key.Binding {
	ic := keys.Keys.Intercept
	return []key.Binding{
		ic.Forward,
		ic.Drop,
		ic.Edit,
		keys.Keys.Global.Help,
	}
}

func (m interceptKeyMap) FullHelp() [][]key.Binding {
	g := keys.Keys.Global
	pageGlobals := []key.Binding{g.Up, g.Down, g.CycleFocus, g.ScrollUp, g.ScrollDown, g.Left, g.Right, g.Escape, g.SendToReplay, g.SendToDiff, g.Copy, g.CopyAs}
	all := append(keys.Keys.Intercept.Bindings(), pageGlobals...)
	all = append(all, g.CommonBindings()...)
	return keys.ChunkByWidth(all, m.width)
}
