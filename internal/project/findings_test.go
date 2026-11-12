package project

import (
	"os"
	"testing"
	"time"
)

func TestFindings(t *testing.T) {
	root := t.TempDir()
	p, _ := Open(root)
	see := func(key string, entry int) bool { return sight(t, p, key, entry) }
	if !see("a", 1) || !see("a", 2) || see("a", 2) || !see("b", 2) {
		t.Fatal("a new entry is a change, the same one is not")
	}
	for i := 3; i < MaxSightings+5; i++ {
		see("a", i)
	}
	if err := p.SetTriage("b", Triage{Status: StatusNoise, Note: "test key"}); err != nil {
		t.Fatal(err)
	}
	if see("b", 9) {
		t.Fatal("a triaged finding is not updated")
	}
	if got := p.FindingsOf(2); len(got) != 2 || got[0].Key != "a" || got[1].Key != "b" {
		t.Fatalf("FindingsOf(2) = %+v", got)
	}
	p.Close()

	q, _ := Open(root)
	fs, err := q.Findings()
	if err != nil || len(fs) != 2 {
		t.Fatalf("%v, %v", fs, err)
	}
	if a := fs[0]; a.Key != "a" || len(a.Entries) != MaxSightings || !a.Full() || a.Entries[1] != 2 || a.Status != StatusNew {
		t.Fatalf("a: %+v", a)
	}
	if b := fs[1]; b.Status != StatusNoise || b.Note != "test key" || len(b.Entries) != 1 {
		t.Fatalf("b: %+v", b)
	}
	if err := q.SetTriage("b", Triage{}); err != nil {
		t.Fatal(err)
	}
	if !sight(t, q, "b", 9) {
		t.Fatal("back to new, it is updated again")
	}
}

func sight(t *testing.T, p *Project, key string, entry int) bool {
	t.Helper()
	ok, err := p.AddSighting(Sighting{Key: key, Detector: "secrets", Title: "t " + key, Entry: entry, Time: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestForgetSightings(t *testing.T) {
	root := t.TempDir()
	p, _ := Open(root)
	var ids []int
	for range 3 {
		e, _ := p.Add(Entry{Method: "GET", URL: "https://a.test/"}, []byte("GET / HTTP/1.1\r\n\r\n"), nil)
		ids = append(ids, e.ID)
	}
	sight(t, p, "a", ids[0])
	sight(t, p, "a", ids[2])
	sight(t, p, "b", ids[2])
	if err := p.Forget([]int{ids[2]}); err != nil {
		t.Fatal(err)
	}
	if fs, _ := p.Findings(); len(fs) != 1 || fs[0].Key != "a" || len(fs[0].Entries) != 1 {
		t.Fatalf("%+v", fs)
	}
	p.Close()

	q, _ := Open(root)
	e, _ := q.Add(Entry{Method: "GET", URL: "https://a.test/new"}, []byte("GET /new HTTP/1.1\r\n\r\n"), nil)
	if e.ID != ids[2] {
		t.Fatalf("the ID is not reused: %d", e.ID)
	}
	if got := q.FindingsOf(e.ID); len(got) != 0 {
		t.Fatalf("the new entry has the findings of the forgotten one: %+v", got)
	}
}

func TestTruncatedSighting(t *testing.T) {
	root := t.TempDir()
	p, _ := Open(root)
	sight(t, p, "a", 1)
	p.Close()
	f, _ := os.OpenFile(p.dir("findings.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"key":"b","detector":"secrets","tit`)
	f.Close()
	q, _ := Open(root)
	sight(t, q, "c", 2)
	q.Close()
	r, _ := Open(root)
	if fs, _ := r.Findings(); len(fs) != 2 || fs[1].Key != "c" {
		t.Fatalf("%+v", fs)
	}
}
