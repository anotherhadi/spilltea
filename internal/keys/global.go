package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type GlobalKeyMap struct {
	Quit          key.Binding
	OpenLogs      key.Binding
	ToggleSidebar key.Binding
	Help          key.Binding
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	CycleFocus    key.Binding
	CopyAs   key.Binding
	Copy          key.Binding
	Escape        key.Binding
	SendToReplay  key.Binding
	ScrollUp      key.Binding
	ScrollDown    key.Binding
	SendToDiff    key.Binding
}

func newGlobalKeyMap(cfg config.GlobalKeys) GlobalKeyMap {
	return GlobalKeyMap{
		Quit:          binding(cfg.Quit, "quit"),
		OpenLogs:      binding(cfg.OpenLogs, "open logs"),
		ToggleSidebar: binding(cfg.ToggleSidebar, "toggle sidebar"),
		Help:          binding(cfg.Help, "help"),
		Up:            binding(cfg.Up, "up"),
		Down:          binding(cfg.Down, "down"),
		Left:          binding(cfg.Left, "scroll left"),
		Right:         binding(cfg.Right, "scroll right"),
		CycleFocus:    binding(cfg.CycleFocus, "cycle focus"),
		CopyAs:   binding(cfg.CopyAs, "copy as..."),
		Copy:          binding(cfg.Copy, "copy..."),
		Escape:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		SendToReplay:  binding(cfg.SendToReplay, "send to replay"),
		ScrollUp:      binding(cfg.ScrollUp, "scroll up"),
		ScrollDown:    binding(cfg.ScrollDown, "scroll down"),
		SendToDiff:    binding(cfg.SendToDiff, "send to diff"),
	}
}

func (g GlobalKeyMap) Bindings() []key.Binding {
	return []key.Binding{
		g.Up, g.Down, g.Left, g.Right, g.CycleFocus,
		g.Quit, g.Escape, g.Help,
		g.OpenLogs, g.ToggleSidebar, g.CopyAs, g.Copy,
		g.SendToReplay, g.SendToDiff,
		g.ScrollUp, g.ScrollDown,
	}
}
