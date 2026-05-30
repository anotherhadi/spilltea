package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type ReplayKeyMap struct {
	Send      key.Binding
	Edit      key.Binding
	EditExt   key.Binding
	UndoEdits key.Binding
	Delete    key.Binding
	DeleteAll key.Binding
	Filter    key.Binding
}

func newReplayKeyMap(cfg config.ReplayKeys) ReplayKeyMap {
	return ReplayKeyMap{
		Send:      binding(cfg.Send, "send"),
		Edit:      binding(cfg.Edit, "edit"),
		EditExt:   binding(cfg.EditExt, "edit in $EDITOR"),
		UndoEdits: binding(cfg.UndoEdits, "undo edits"),
		Delete:    binding(cfg.Delete, "delete"),
		DeleteAll: binding(cfg.DeleteAll, "delete all"),
		Filter:    binding(cfg.Filter, "filter"),
	}
}

func (r ReplayKeyMap) Bindings() []key.Binding {
	return []key.Binding{r.Send, r.Edit, r.EditExt, r.UndoEdits, r.Delete, r.DeleteAll, r.Filter}
}
