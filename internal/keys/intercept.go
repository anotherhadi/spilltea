package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type InterceptKeyMap struct {
	Forward         key.Binding
	ForwardAll      key.Binding
	Drop            key.Binding
	DropAll         key.Binding
	AutoForward     key.Binding
	CaptureResponse key.Binding
	UndoEdits       key.Binding
	Edit            key.Binding
	EditExternal    key.Binding
}

func newInterceptKeyMap(cfg config.InterceptKeys) InterceptKeyMap {
	return InterceptKeyMap{
		Forward:         binding(cfg.Forward, "forward"),
		ForwardAll:      binding(cfg.ForwardAll, "forward all"),
		Drop:            binding(cfg.Drop, "drop"),
		DropAll:         binding(cfg.DropAll, "drop all"),
		AutoForward:     binding(cfg.AutoForward, "auto forward"),
		CaptureResponse: binding(cfg.CaptureResponse, "capture response"),
		UndoEdits:       binding(cfg.UndoEdits, "undo edits"),
		Edit:            binding(cfg.Edit, "edit"),
		EditExternal:    binding(cfg.EditExternal, "edit in $EDITOR"),
	}
}

func (ic InterceptKeyMap) Bindings() []key.Binding {
	return []key.Binding{
		ic.Forward, ic.ForwardAll,
		ic.Drop, ic.DropAll,
		ic.Edit, ic.EditExternal, ic.UndoEdits,
		ic.AutoForward, ic.CaptureResponse,
	}
}
