// Package project stores the history as plain files:
//
//	<project>/
//	├── config.yaml     # optional, written by the user: scope, rules
//	├── index.jsonl     # one line per entry, appended as they come
//	├── meta.json       # marks and notes
//	├── findings.jsonl
//	├── findings.json
//	├── requests/<id>   # raw request, as sent
//	└── responses/<id>  # raw response, as much as history.responses keeps
package project

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"
)

// Kept is how much of a response is stored.
type Kept string

const (
	KeptNone    Kept = ""
	KeptStatus  Kept = "status"
	KeptHeaders Kept = "headers"
	KeptFull    Kept = "full"
)

type Entry struct {
	ID     int       `json:"id"`
	Time   time.Time `json:"time"`
	Method string    `json:"method"`
	URL    string    `json:"url"`
	// Response fields, as far as it is kept; Status is 0 without one.
	Status int    `json:"status,omitempty"`
	Len    int64  `json:"len,omitempty"` // response body length
	Mime   string `json:"mime,omitempty"`
	MS     int64  `json:"ms,omitempty"` // round trip
	Kept   Kept   `json:"kept,omitempty"`
	Color  string `json:"color,omitempty"`
	Asset  bool   `json:"asset,omitempty"`
	Edited bool   `json:"edited,omitempty"` // changed in the intercept
	Error  string `json:"error,omitempty"`  // "dropped", or why there is no response
	// Source is the tool an imported entry comes from; "" for the proxy.
	Source string `json:"source,omitempty"`
}

type Meta struct {
	Marked bool   `json:"marked,omitempty"`
	Note   string `json:"note,omitempty"`
}

type Project struct {
	Root string

	mu     sync.Mutex
	index  *os.File
	next   int
	loaded bool // next is up to date
	meta   map[int]Meta
	closed bool

	fmu      sync.Mutex
	findings findings
}

func Open(root string) (*Project, error) {
	p := &Project{Root: root}
	for _, d := range []string{root, p.dir("requests"), p.dir("responses")} {
		// requests contain cookies and tokens
		if err := os.MkdirAll(d, 0o700); err != nil {
			if pe := (*fs.PathError)(nil); errors.As(err, &pe) {
				err = pe.Err
			}
			return nil, fmt.Errorf("project %s: %w", root, err)
		}
	}
	return p, nil
}

// OpenExisting fails on a missing folder, so a typo in the path doesn't
// create an empty project.
func OpenExisting(root string) (*Project, error) {
	fi, err := os.Stat(root)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("project %s does not exist (createProject is false)", root)
	case err != nil:
		return nil, fmt.Errorf("project %s: %w", root, err)
	case !fi.IsDir():
		return nil, fmt.Errorf("project %s is not a folder", root)
	}
	return Open(root)
}

func (p *Project) dir(name string) string { return filepath.Join(p.Root, name) }

func (p *Project) RequestPath(id int) string {
	return filepath.Join(p.dir("requests"), strconv.Itoa(id))
}

func (p *Project) ResponsePath(id int) string {
	return filepath.Join(p.dir("responses"), strconv.Itoa(id))
}

// Load skips truncated lines (spilltea killed mid-write).
func (p *Project) Load() ([]Entry, map[int]Meta, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.load()
}

func (p *Project) load() ([]Entry, map[int]Meta, error) {
	p.loaded = true
	var out []Entry
	f, err := os.Open(p.dir("index.jsonl"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		err = nil
	case err != nil:
		return nil, nil, err
	default:
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 16<<20)
		for sc.Scan() {
			var e Entry
			if json.Unmarshal(sc.Bytes(), &e) == nil && e.ID > 0 {
				out = append(out, e)
				p.next = max(p.next, e.ID)
			}
		}
		err = sc.Err()
		f.Close()
	}
	p.meta = map[int]Meta{}
	if b, rerr := os.ReadFile(p.dir("meta.json")); rerr == nil {
		var m map[string]Meta
		if json.Unmarshal(b, &m) == nil {
			for k, v := range m {
				if id, err := strconv.Atoi(k); err == nil {
					p.meta[id] = v
				}
			}
		}
	}
	return out, p.meta, err
}

// Add assigns e an ID and stores it; resp may be nil. Safe to call from
// several goroutines.
func (p *Project) Add(e Entry, req, resp []byte) (Entry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return e, errors.New("project closed")
	}
	if !p.loaded {
		if _, _, err := p.load(); err != nil {
			return e, err
		}
	}
	if p.index == nil {
		f, err := openLines(p.dir("index.jsonl"))
		if err != nil {
			return e, err
		}
		p.index = f
	}
	// another process may write to the project (spilltea import while the
	// TUI runs): whoever creates the request file first owns the ID
	for {
		p.next++
		e.ID = p.next
		f, err := os.OpenFile(p.RequestPath(e.ID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return e, err
		}
		_, err = f.Write(req)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return e, err
		}
		break
	}
	if resp != nil {
		if err := os.WriteFile(p.ResponsePath(e.ID), resp, 0o600); err != nil {
			return e, err
		}
	}
	b, err := json.Marshal(e)
	if err != nil {
		return e, err
	}
	_, err = p.index.Write(append(b, '\n'))
	return e, err
}

func (p *Project) ReadRequest(id int) ([]byte, error) { return os.ReadFile(p.RequestPath(id)) }

// ReadResponse returns nil, without an error, when no response is stored.
func (p *Project) ReadResponse(id int) ([]byte, error) {
	b, err := os.ReadFile(p.ResponsePath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// SetMeta removes the entry from meta.json when m is empty.
func (p *Project) SetMeta(id int, m Meta) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.meta == nil {
		p.meta = map[int]Meta{}
	}
	if m == (Meta{}) {
		delete(p.meta, id)
	} else {
		p.meta[id] = m
	}
	return p.saveMeta()
}

func (p *Project) saveMeta() error {
	out := make(map[string]Meta, len(p.meta))
	for id, m := range p.meta {
		out[strconv.Itoa(id)] = m
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(p.dir("meta.json"), b)
}

func openLines(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	// a truncated line must not swallow the next one
	if fi, err := f.Stat(); err == nil && fi.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, fi.Size()-1); err == nil && last[0] != '\n' {
			f.Write([]byte{'\n'})
		}
	}
	return f, nil
}

func writeAtomic(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".spilltea-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Forget removes the index.jsonl line, the files and the metadata.
func (p *Project) Forget(ids []int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	drop := map[int]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	b, err := os.ReadFile(p.dir("index.jsonl"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var kept []byte
	for _, line := range splitLines(b) {
		var e Entry
		if json.Unmarshal(line, &e) == nil && drop[e.ID] {
			continue
		}
		kept = append(append(kept, line...), '\n')
	}
	if p.index != nil {
		p.index.Close()
		p.index = nil // reopened by the next Add, after the rewrite
	}
	var errs []error
	if err := writeAtomic(p.dir("index.jsonl"), kept); err != nil {
		errs = append(errs, err)
	}
	for _, id := range ids {
		for _, f := range []string{p.RequestPath(id), p.ResponsePath(id)} {
			if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
		}
		delete(p.meta, id)
	}
	errs = append(errs, p.saveMeta(), p.forgetSightings(drop))
	return errors.Join(errs...)
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	for len(b) > 0 {
		i := slices.Index(b, '\n')
		if i < 0 {
			out = append(out, b)
			break
		}
		if i > 0 {
			out = append(out, b[:i])
		}
		b = b[i+1:]
	}
	return out
}

func (p *Project) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	var errs []error
	if p.index != nil {
		errs = append(errs, p.index.Close())
		p.index = nil
	}
	p.fmu.Lock()
	defer p.fmu.Unlock()
	if p.findings.file != nil {
		errs = append(errs, p.findings.file.Close())
		p.findings.file = nil
	}
	return errors.Join(errs...)
}
