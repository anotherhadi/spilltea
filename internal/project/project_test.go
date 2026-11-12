package project

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAddLoadForget(t *testing.T) {
	root := filepath.Join(t.TempDir(), "p")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := p.Add(Entry{Method: "GET", URL: "https://h/"}, []byte("GET / HTTP/1.1\r\n\r\n"), []byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	e, _ := p.Add(Entry{Method: "POST", URL: "https://h/x", Kept: KeptStatus, Status: 302}, []byte("POST /x HTTP/1.1\r\n\r\n"), nil)
	if e.ID != 21 {
		t.Fatalf("id %d", e.ID)
	}
	if err := p.SetMeta(3, Meta{Marked: true, Note: "idor?"}); err != nil {
		t.Fatal(err)
	}
	p.Close()

	q, _ := Open(root)
	es, meta, err := q.Load()
	if err != nil || len(es) != 21 || meta[3].Note != "idor?" {
		t.Fatalf("Load: %d entries, %v, %v", len(es), meta, err)
	}
	if b, _ := q.ReadResponse(21); b != nil {
		t.Fatal("a status-only entry has no response file")
	}
	if fi, _ := os.Stat(root); fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode %v", fi.Mode())
	}
	if err := q.Forget([]int{3, 21}); err != nil {
		t.Fatal(err)
	}
	if e, err := q.Add(Entry{Method: "GET"}, []byte("x"), nil); err != nil || e.ID != 22 {
		t.Fatalf("after Forget: %d, %v", e.ID, err)
	}
	es, meta, _ = q.Load()
	if len(es) != 20 || len(meta) != 0 {
		t.Fatalf("after Forget: %d entries, meta %v", len(es), meta)
	}
	if _, err := os.Stat(q.RequestPath(3)); !os.IsNotExist(err) {
		t.Fatal("the files of a forgotten entry are left")
	}
}

func TestCutLine(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "index.jsonl"), []byte("{\"id\":1,\"method\":\"GET\"}\n{\"id\":2,\"meth"), 0o600)
	p, _ := Open(root)
	es, _, err := p.Load()
	if err != nil || len(es) != 1 {
		t.Fatalf("%v %v", es, err)
	}
	if _, err := p.Add(Entry{Method: "GET"}, []byte("x"), nil); err != nil {
		t.Fatal(err)
	}
	if es, _, _ := p.Load(); len(es) != 2 || es[1].ID != 2 {
		t.Fatalf("the entry after a cut line: %+v", es)
	}
	if _, err := OpenExisting(filepath.Join(root, "nope")); err == nil {
		t.Fatal("OpenExisting on a missing folder")
	}
}

func TestAddWithoutLoad(t *testing.T) {
	root := t.TempDir()
	p, _ := Open(root)
	p.Add(Entry{}, []byte("a"), nil)
	p.Close()
	q, _ := Open(root)
	if e, _ := q.Add(Entry{}, []byte("b"), nil); e.ID != 2 {
		t.Fatalf("id %d: numbers go on from the index", e.ID)
	}
}

// Two spilltea on a project (the TUI, spilltea import) keep each other's
// files: the second takes the next free number.
func TestAddTwoProcesses(t *testing.T) {
	root := t.TempDir()
	a, _ := Open(root)
	b, _ := Open(root)
	a.Load()
	b.Load()
	ea, _ := a.Add(Entry{Method: "GET", URL: "https://h/a"}, []byte("A"), nil)
	eb, _ := b.Add(Entry{Method: "GET", URL: "https://h/b"}, []byte("B"), nil)
	if ea.ID == eb.ID {
		t.Fatalf("both are #%d", ea.ID)
	}
	if r, _ := a.ReadRequest(ea.ID); string(r) != "A" {
		t.Fatalf("#%d holds %q", ea.ID, r)
	}
	a.Close()
	b.Close()
	es, _, _ := a.Load()
	if len(es) != 2 {
		t.Fatalf("%d entries", len(es))
	}
}

func TestAddAfterClose(t *testing.T) {
	p, _ := Open(t.TempDir())
	p.Close()
	if _, err := p.Add(Entry{Method: "GET"}, []byte("x"), nil); err == nil {
		t.Fatal("Add after Close")
	}
}
