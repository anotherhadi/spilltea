package entries

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/spilltea/internal/project"
)

func TestMouseOpensOnlyWhenAlreadyFocused(t *testing.T) {
	p, err := project.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(p, Meta{}, nil, nil)
	m.SetEntries([]project.Entry{{ID: 1, Method: "GET", URL: "http://x/a"}})
	// row 0, just below the column header
	click := tea.MouseClickMsg{Y: 1, Button: tea.MouseLeft}

	if cmd := m.Mouse(click, false); cmd != nil {
		t.Fatalf("a click arriving from another pane must not open the row, got %T", cmd())
	}
	cmd := m.Mouse(click, true)
	if cmd == nil {
		t.Fatal("a click on the already-focused, already-selected row must open it")
	}
	if _, ok := cmd().(OpenMsg); !ok {
		t.Fatalf("expected OpenMsg, got %T", cmd())
	}
}
