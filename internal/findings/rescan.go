package findings

import (
	"errors"

	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
)

// Rescan runs the detectors on the stored entries. Bodies that were not
// stored, like those of the assets, are not scanned.
func Rescan(p *project.Project, ds []Detector, es []project.Entry) (changed int, err error) {
	var errs []error
	h := NewHandler(p, ds, func(err error) {
		if err != nil {
			errs = append(errs, err)
		} else {
			changed++
		}
	})
	for _, e := range es {
		raw, err := p.ReadRequest(e.ID)
		if err != nil {
			continue
		}
		req, err := message.ParseRequest(raw, "https")
		if err != nil {
			continue
		}
		if u, err := req.URL.Parse(e.URL); err == nil {
			req.URL = u
		}
		var resp *message.Response
		if raw, err := p.ReadResponse(e.ID); err == nil && raw != nil {
			resp, _ = message.ParseResponse(raw)
		}
		h.Handle(e, req, resp)
	}
	return changed, errors.Join(errs...)
}
