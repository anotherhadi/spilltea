package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type HistoryKeyMap struct {
	DeleteEntry key.Binding
	DeleteAll   key.Binding
	Filter      key.Binding
	SqlQuery    key.Binding
	Flag        key.Binding
}

func newHistoryKeyMap(cfg config.HistoryKeys) HistoryKeyMap {
	return HistoryKeyMap{
		DeleteEntry: binding(cfg.DeleteEntry, "delete entry"),
		DeleteAll:   binding(cfg.DeleteAll, "delete all"),
		Filter:      binding(cfg.Filter, "filter"),
		SqlQuery:    binding(cfg.SqlQuery, "sql query"),
		Flag:        binding(cfg.Flag, "flag"),
	}
}

func (h HistoryKeyMap) Bindings() []key.Binding {
	return []key.Binding{h.DeleteEntry, h.DeleteAll, h.Flag}
}
