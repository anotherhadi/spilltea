package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type FindingsKeyMap struct {
	Dismiss key.Binding
}

func newFindingsKeyMap(cfg config.FindingsKeys) FindingsKeyMap {
	return FindingsKeyMap{
		Dismiss: binding(cfg.Dismiss, "dismiss"),
	}
}

func (f FindingsKeyMap) Bindings() []key.Binding {
	return []key.Binding{f.Dismiss}
}
