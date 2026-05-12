package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type HomeKeyMap struct {
	Open   key.Binding
	Delete key.Binding
	Filter key.Binding
}

func newHomeKeyMap(cfg config.HomeKeys) HomeKeyMap {
	return HomeKeyMap{
		Open:   binding(cfg.Open, "open"),
		Delete: binding(cfg.Delete, "delete project"),
		Filter: binding(cfg.Filter, "filter"),
	}
}

func (h HomeKeyMap) Bindings() []key.Binding {
	return []key.Binding{h.Open, h.Delete, h.Filter}
}
