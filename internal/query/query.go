// Package query is spilltea's HTTPQL: Caido's fields, plus these.
//
//	row.note     text     the entry's note
//	row.color    text     its PwnFox color: red, blue...
//	row.asset    boolean  image, stylesheet, script...
//	row.edited   boolean  changed in the intercept
//	row.source   text     proxy, or the tool it was imported from
//	row.finding  text     detectors and titles of its findings, but noise
//	resp.mime    text     the response's media type: text/html
package query

import (
	"net/url"
	"slices"
	"strconv"

	"github.com/anotherhadi/httpql"

	"github.com/anotherhadi/spilltea/internal/project"
)

type Record struct {
	httpql.Record
	Note     string
	Color    string
	Asset    bool
	Edited   bool
	Source   string
	Mime     string
	Findings func() []string
}

var Schema = func() *httpql.Schema[*Record] {
	fs := httpql.Fields(func(r *Record) *httpql.Record { return &r.Record })
	fs["row.note"] = httpql.Text(func(r *Record) string { return r.Note })
	fs["row.color"] = httpql.Text(func(r *Record) string { return r.Color })
	fs["row.asset"] = httpql.Bool(func(r *Record) bool { return r.Asset })
	fs["row.edited"] = httpql.Bool(func(r *Record) bool { return r.Edited })
	fs["row.source"] = httpql.Text(func(r *Record) string { return r.Source })
	fs["resp.mime"] = httpql.Text(func(r *Record) string { return r.Mime })
	fs["row.finding"] = httpql.Texts(func(r *Record) []string {
		var out []string
		if r.Findings != nil {
			out = r.Findings()
		}
		if len(out) == 0 {
			return []string{""} // so that ne:"" matches entries with findings
		}
		return out
	})
	return &httpql.Schema[*Record]{Fields: fs, Refused: map[string]string{
		"source": "use row.source: proxy, httpx, katana, urlfinder, import",
	}}
}()

func (r *Record) SetURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		return
	}
	r.Host, r.Path, r.Query, r.TLS = u.Hostname(), u.EscapedPath(), u.RawQuery, u.Scheme == "https"
	if r.Path == "" {
		r.Path = "/"
	}
	r.Port, _ = strconv.Atoi(u.Port())
	if r.Port == 0 {
		r.Port = 80
		if r.TLS {
			r.Port = 443
		}
	}
}

// New only reads the raw messages from disk if the query needs them.
func New(p *project.Project, e project.Entry, m project.Meta) *Record {
	r := &Record{
		Record: httpql.Record{
			ID: e.ID, Method: e.Method, Created: e.Time, Code: e.Status, Len: e.Len, Roundtrip: e.MS,
			Marked: m.Marked,
			ReqRaw: func() string {
				b, _ := p.ReadRequest(e.ID)
				return string(b)
			},
			RespRaw: func() string {
				b, _ := p.ReadResponse(e.ID)
				return string(b)
			},
		},
		Note: m.Note, Color: e.Color, Asset: e.Asset, Edited: e.Edited, Source: e.Source, Mime: e.Mime,
		Findings: func() []string {
			var out []string
			for _, f := range p.FindingsOf(e.ID) {
				if f.Status == project.StatusNoise {
					continue
				}
				if !slices.Contains(out, f.Detector) {
					out = append(out, f.Detector)
				}
				out = append(out, f.Title)
			}
			return out
		},
	}
	if r.Source == "" {
		r.Source = "proxy"
	}
	r.SetURL(e.URL)
	return r
}
