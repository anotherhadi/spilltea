package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type FindingsKeyMap struct {
	Dismiss    key.Binding
	DismissAll key.Binding
	Flag       key.Binding
}

func newFindingsKeyMap(cfg config.FindingsKeys) FindingsKeyMap {
	return FindingsKeyMap{
		Dismiss:    binding(cfg.Dismiss, "dismiss"),
		DismissAll: binding(cfg.DismissAll, "dismiss all"),
		Flag:       binding(cfg.Flag, "flag"),
	}
}

func (f FindingsKeyMap) Bindings() []key.Binding {
	return []key.Binding{f.Flag, f.Dismiss, f.DismissAll}
}
