package scan

import (
	"testing"

	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
)

type blocker struct{ release chan struct{} }

func (b blocker) Handle(project.Entry, *message.Request, *message.Response) { <-b.release }

func TestQueueBytes(t *testing.T) {
	b := blocker{make(chan struct{})}
	s := New(b)
	big := &message.Response{Body: make([]byte, queueBytes/2+1)}
	if !s.Scan(project.Entry{ID: 1}, nil, big) {
		t.Fatal("the first one fits")
	}
	if s.Scan(project.Entry{ID: 2}, nil, big) {
		t.Fatal("over the byte budget")
	}
	if !s.Scan(project.Entry{ID: 3}, nil, &message.Response{Body: []byte("small")}) {
		t.Fatal("a small one still fits")
	}
	close(b.release)
	s.Close()
}
