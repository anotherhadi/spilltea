package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type DocsKeyMap struct {
	Search      key.Binding
	SearchReset key.Binding
	SearchNext  key.Binding
	SearchPrev  key.Binding
}

func newDocsKeyMap(cfg config.DocsKeys) DocsKeyMap {
	return DocsKeyMap{
		Search:      binding(cfg.Search, "search"),
		SearchReset: binding(cfg.SearchReset, "reset search"),
		SearchNext:  binding(cfg.SearchNext, "next match"),
		SearchPrev:  binding(cfg.SearchPrev, "prev match"),
	}
}

func (d DocsKeyMap) Bindings() []key.Binding {
	return []key.Binding{d.Search, d.SearchReset, d.SearchNext, d.SearchPrev}
}
