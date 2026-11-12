package ui

import (
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/foam/notify"
)

// historyPath holds the used HTTPQL filters, one per line, oldest first.
func historyPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "spilltea", "filters")
}

func loadHistory() []string {
	b, err := os.ReadFile(historyPath())
	if err != nil {
		return nil
	}
	var h []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			h = append(h, l)
		}
	}
	return h
}

func saveHistory(h []string) tea.Cmd {
	return func() tea.Msg {
		p := historyPath()
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return notify.Msg{Level: notify.WarningLevel, Text: "filter history: " + err.Error()}
		}
		if err := os.WriteFile(p, []byte(strings.Join(h, "\n")+"\n"), 0o600); err != nil {
			return notify.Msg{Level: notify.WarningLevel, Text: "filter history: " + err.Error()}
		}
		return nil
	}
}
