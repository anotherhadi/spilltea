package project

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"
	"time"
)

const MaxSightings = 20

type Status string

const (
	StatusNew      Status = ""
	StatusRelevant Status = "relevant"
	StatusNoise    Status = "noise"
)

// Sighting is a line of findings.jsonl.
type Sighting struct {
	Key      string    `json:"key"`
	Detector string    `json:"detector"`
	Title    string    `json:"title"`
	Detail   string    `json:"detail,omitempty"`
	Entry    int       `json:"entry"`
	Time     time.Time `json:"time"`
}

type Triage struct {
	Status Status `json:"status,omitempty"`
	Note   string `json:"note,omitempty"`
}

type Finding struct {
	Key      string
	Detector string
	Title    string
	Detail   string
	Entries  []int
	First    time.Time
	Last     time.Time
	Triage
}

func (f Finding) Full() bool { return len(f.Entries) >= MaxSightings }

type findings struct {
	loaded  bool
	file    *os.File
	list    []*Finding
	byKey   map[string]*Finding
	byEntry map[int][]*Finding
	triage  map[string]Triage
}

func (p *Project) loadFindings() error {
	f := &p.findings
	if f.loaded {
		return nil
	}
	*f = findings{loaded: true, byKey: map[string]*Finding{}, byEntry: map[int][]*Finding{}, triage: map[string]Triage{}}
	if b, err := os.ReadFile(p.dir("findings.json")); err == nil {
		json.Unmarshal(b, &f.triage)
	}
	r, err := os.Open(p.dir("findings.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer r.Close()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	for sc.Scan() {
		var s Sighting
		if json.Unmarshal(sc.Bytes(), &s) == nil && s.Key != "" {
			p.addSighting(s)
		}
	}
	return sc.Err()
}

func (p *Project) addSighting(s Sighting) bool {
	f := &p.findings
	fd := f.byKey[s.Key]
	if fd == nil {
		fd = &Finding{Key: s.Key, Detector: s.Detector, Title: s.Title, Detail: s.Detail, First: s.Time, Triage: f.triage[s.Key]}
		f.byKey[s.Key] = fd
		f.list = append(f.list, fd)
	} else if fd.Status != StatusNew || fd.Full() || slices.Contains(fd.Entries, s.Entry) {
		return false
	}
	fd.Entries = append(fd.Entries, s.Entry)
	fd.Last = s.Time
	f.byEntry[s.Entry] = append(f.byEntry[s.Entry], fd)
	return true
}

// AddSighting does nothing once the finding is triaged.
func (p *Project) AddSighting(s Sighting) (bool, error) {
	p.fmu.Lock()
	defer p.fmu.Unlock()
	if err := p.loadFindings(); err != nil {
		return false, err
	}
	if !p.addSighting(s) {
		return false, nil
	}
	f := &p.findings
	if f.file == nil {
		w, err := openLines(p.dir("findings.jsonl"))
		if err != nil {
			return true, err
		}
		f.file = w
	}
	b, err := json.Marshal(s)
	if err != nil {
		return true, err
	}
	_, err = f.file.Write(append(b, '\n'))
	return true, err
}

// IDs come back after a Forget: a sighting must not stick to a new entry.
func (p *Project) forgetSightings(drop map[int]bool) error {
	p.fmu.Lock()
	defer p.fmu.Unlock()
	b, err := os.ReadFile(p.dir("findings.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var kept []byte
	for _, line := range splitLines(b) {
		var s Sighting
		if json.Unmarshal(line, &s) == nil && drop[s.Entry] {
			continue
		}
		kept = append(append(kept, line...), '\n')
	}
	if len(kept) == len(b) {
		return nil
	}
	if p.findings.file != nil {
		p.findings.file.Close()
		p.findings.file = nil
	}
	p.findings.loaded = false
	return writeAtomic(p.dir("findings.jsonl"), kept)
}

func (p *Project) Findings() ([]Finding, error) {
	p.fmu.Lock()
	defer p.fmu.Unlock()
	err := p.loadFindings()
	out := make([]Finding, len(p.findings.list))
	for i, f := range p.findings.list {
		out[i] = *f
		out[i].Entries = slices.Clone(f.Entries)
	}
	return out, err
}

func (p *Project) FindingsOf(id int) []Finding {
	p.fmu.Lock()
	defer p.fmu.Unlock()
	if p.loadFindings() != nil {
		return nil
	}
	var out []Finding
	for _, f := range p.findings.byEntry[id] {
		out = append(out, Finding{Key: f.Key, Detector: f.Detector, Title: f.Title, Triage: f.Triage})
	}
	return out
}

func (p *Project) SetTriage(key string, t Triage) error {
	p.fmu.Lock()
	defer p.fmu.Unlock()
	if err := p.loadFindings(); err != nil {
		return err
	}
	f := &p.findings
	if t == (Triage{}) {
		delete(f.triage, key)
	} else {
		f.triage[key] = t
	}
	if fd := f.byKey[key]; fd != nil {
		fd.Triage = t
	}
	b, err := json.MarshalIndent(f.triage, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(p.dir("findings.json"), b)
}
