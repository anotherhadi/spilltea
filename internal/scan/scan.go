// Package scan hands the recorded messages to handlers, in the background.
package scan

import (
	"sync"
	"sync/atomic"

	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
)

// Handle is called from the scanner's goroutine. resp may be nil.
type Handler interface {
	Handle(e project.Entry, req *message.Request, resp *message.Response)
}

type job struct {
	e    project.Entry
	req  *message.Request
	resp *message.Response
}

const (
	queueSize  = 256
	queueBytes = 64 << 20
)

type Scanner struct {
	hs    []Handler
	jobs  chan job
	bytes atomic.Int64
	stop  chan struct{}
	done  chan struct{}
	once  sync.Once
}

func New(hs ...Handler) *Scanner {
	s := &Scanner{hs: hs, jobs: make(chan job, queueSize), stop: make(chan struct{}), done: make(chan struct{})}
	go s.run()
	return s
}

// Scan returns false when the queue is full.
func (s *Scanner) Scan(e project.Entry, req *message.Request, resp *message.Response) bool {
	n := size(req, resp)
	if s.bytes.Add(n) > queueBytes {
		s.bytes.Add(-n)
		return false
	}
	select {
	case s.jobs <- job{e, req, resp}:
		return true
	default:
		s.bytes.Add(-n)
		return false
	}
}

func size(req *message.Request, resp *message.Response) int64 {
	var n int
	if req != nil {
		n += len(req.Body)
	}
	if resp != nil {
		n += len(resp.Body)
	}
	return int64(n)
}

func (s *Scanner) Close() {
	s.once.Do(func() { close(s.stop) })
	<-s.done
}

func (s *Scanner) run() {
	defer close(s.done)
	for {
		select {
		case j := <-s.jobs:
			for _, h := range s.hs {
				h.Handle(j.e, j.req, j.resp)
			}
			s.bytes.Add(-size(j.req, j.resp))
		case <-s.stop:
			return
		}
	}
}
