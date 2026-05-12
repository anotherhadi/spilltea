package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type DiffKeyMap struct {
	Clear key.Binding
}

func newDiffKeyMap(cfg config.DiffKeys) DiffKeyMap {
	return DiffKeyMap{
		Clear: binding(cfg.Clear, "clear"),
	}
}

func (d DiffKeyMap) Bindings() []key.Binding {
	return []key.Binding{d.Clear}
}
