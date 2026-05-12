package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type PluginsKeyMap struct {
	Toggle     key.Binding
	EditConfig key.Binding
	Filter     key.Binding
}

func newPluginsKeyMap(cfg config.PluginsKeys) PluginsKeyMap {
	return PluginsKeyMap{
		Toggle:     binding(cfg.Toggle, "toggle"),
		EditConfig: binding(cfg.EditConfig, "edit config"),
		Filter:     binding(cfg.Filter, "filter"),
	}
}

func (p PluginsKeyMap) Bindings() []key.Binding {
	return []key.Binding{p.Toggle, p.EditConfig, p.Filter}
}
