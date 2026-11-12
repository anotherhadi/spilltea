package reader

import "testing"

// Show must not render synchronously: holding j/up/down in a list calls
// Show on every keypress, and a synchronous render (disk-sized bodies,
// full-message highlighting) would queue up behind the keys.
func TestShowIsAsync(t *testing.T) {
	m := New(false, true, nil)
	cmd := m.Show([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\nbody"), nil)
	if cmd == nil {
		t.Fatal("Show did not return a command")
	}
	if got := m.view.Lines(); got != nil {
		t.Fatalf("content set before the command ran: %v", got)
	}
	msg := cmd()
	if !m.Update(msg) {
		t.Fatal("Update did not handle its own renderedMsg")
	}
	lines := m.view.Lines()
	if len(lines) == 0 || lines[0] != "GET / HTTP/1.1" {
		t.Fatalf("lines = %v", lines)
	}
}

// A Show superseded by a later one before its debounce elapsed must be
// cancelled outright: computeContent (uncancellable once started) never
// runs for it, which is the whole point when j/up/down is held.
func TestShowCancelsSupersededRender(t *testing.T) {
	m := New(false, true, nil)
	old := m.Show([]byte("GET /old HTTP/1.1\r\nHost: x\r\n\r\n"), nil)
	newCmd := m.Show([]byte("GET /new HTTP/1.1\r\nHost: x\r\n\r\n"), nil)

	if msg := old(); msg != nil {
		t.Fatalf("superseded render was not cancelled, got %T", msg)
	}
	if !m.Update(newCmd()) {
		t.Fatal("Update did not handle the new renderedMsg")
	}
	lines := m.view.Lines()
	if len(lines) == 0 || lines[0] != "GET /new HTTP/1.1" {
		t.Fatalf("lines = %v", lines)
	}
}

// Clear must invalidate any render still in flight, or it could resurrect
// the cleared content once that command finally returns.
func TestClearDropsPendingRender(t *testing.T) {
	m := New(false, true, nil)
	cmd := m.Show([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"), nil)
	m.Clear()
	m.Update(cmd())
	if got := m.view.Lines(); len(got) != 0 {
		t.Fatalf("a render in flight before Clear resurrected content: %v", got)
	}
}
