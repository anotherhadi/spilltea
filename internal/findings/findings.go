// Package findings runs detectors on the traffic and stores what they find.
package findings

import (
	"cmp"
	"time"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
)

type Hit struct {
	Key    string
	Title  string
	Detail string
}

// resp may be nil.
type Detector interface {
	Name() string
	Detect(req *message.Request, resp *message.Response) []Hit
}

func Detectors(cfg config.Findings) ([]Detector, error) {
	var out []Detector
	if cfg.Secrets {
		d, err := NewSecrets()
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if cfg.Cookies {
		out = append(out, Cookies{})
	}
	if cfg.CORS {
		out = append(out, CORS{})
	}
	if cfg.Disclosure {
		out = append(out, Disclosure{})
	}
	if cfg.Reflection {
		out = append(out, Reflection{})
	}
	return out, nil
}

type Handler struct {
	p       *project.Project
	ds      []Detector
	changed func(error)
}

func NewHandler(p *project.Project, ds []Detector, changed func(error)) *Handler {
	return &Handler{p: p, ds: ds, changed: changed}
}

func (h *Handler) Handle(e project.Entry, req *message.Request, resp *message.Response) {
	found := false
	for _, d := range h.ds {
		for _, hit := range d.Detect(req, resp) {
			ok, err := h.p.AddSighting(project.Sighting{Key: d.Name() + ":" + hit.Key, Detector: d.Name(),
				Title: hit.Title, Detail: hit.Detail, Entry: e.ID, Time: cmp.Or(e.Time, time.Now())})
			if err != nil {
				h.changed(err)
			}
			found = found || ok
		}
	}
	if found {
		h.changed(nil)
	}
}
