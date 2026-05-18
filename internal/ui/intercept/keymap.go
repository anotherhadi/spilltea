package intercept

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/icons"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/style"
)

func newHelp() help.Model { return style.NewHelp() }

type interceptKeyMap struct{ width int }

func iconBinding(b key.Binding, icon string) key.Binding {
	h := b.Help()
	return key.NewBinding(key.WithKeys(b.Keys()...), key.WithHelp(h.Key, icon+h.Desc))
}

func (interceptKeyMap) ShortHelp() []key.Binding {
	ic := keys.Keys.Intercept
	i := icons.I
	return []key.Binding{
		iconBinding(ic.Forward, i.Forward),
		iconBinding(ic.Drop, i.Drop),
		iconBinding(ic.Edit, i.Edit),
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
