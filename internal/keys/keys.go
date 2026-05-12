package keys

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/anotherhadi/spilltea/internal/config"
)

type KeyMap struct {
	Global    GlobalKeyMap
	Intercept InterceptKeyMap
	Home      HomeKeyMap
	History   HistoryKeyMap
	Replay    ReplayKeyMap
	Diff      DiffKeyMap
	Findings  FindingsKeyMap
	Plugins   PluginsKeyMap
}

var Keys *KeyMap

func Init(cfg *config.Config) {
	kb := cfg.Keybindings
	Keys = &KeyMap{
		Global:    newGlobalKeyMap(kb.Global),
		Intercept: newInterceptKeyMap(kb.Intercept),
		Home:      newHomeKeyMap(kb.Home),
		History:   newHistoryKeyMap(kb.History),
		Replay:    newReplayKeyMap(kb.Replay),
		Diff:      newDiffKeyMap(kb.Diff),
		Findings:  newFindingsKeyMap(kb.Findings),
		Plugins:   newPluginsKeyMap(kb.Plugins),
	}
}

func parseKeys(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if k := strings.TrimSpace(p); k != "" {
			out = append(out, k)
		}
	}
	return out
}

func binding(s, help string) key.Binding {
	keys := parseKeys(s)
	display := strings.Join(keys, "/")
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(display, help))
}

// ChunkByWidth splits bindings into columns sized to fit the terminal width.
func ChunkByWidth(bindings []key.Binding, termWidth int) [][]key.Binding {
	cols := termWidth / 26
	if cols < 2 {
		cols = 2
	} else if cols > 7 {
		cols = 7
	}
	perCol := (len(bindings) + cols - 1) / cols
	var out [][]key.Binding
	for i := 0; i < len(bindings); i += perCol {
		end := i + perCol
		if end > len(bindings) {
			end = len(bindings)
		}
		out = append(out, bindings[i:end])
	}
	return out
}
