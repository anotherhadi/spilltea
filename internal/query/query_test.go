package query

import (
	"testing"
	"time"

	"github.com/anotherhadi/spilltea/internal/project"
)

func TestFields(t *testing.T) {
	p, _ := project.Open(t.TempDir())
	e, _ := p.Add(project.Entry{Method: "POST", URL: "https://api.test/login?x=1", Time: time.Now(),
		Status: 302, Color: "red", Mime: "text/html"}, []byte("POST /login?x=1 HTTP/1.1\r\nHost: api.test\r\n\r\npassword=hunter2"), nil)
	r := New(p, e, project.Meta{Marked: true, Note: "check the redirect"})
	for q, want := range map[string]bool{
		`row.color.eq:"red" AND row.marked.eq:true`: true,
		`row.note.cont:"redirect"`:                  true,
		`resp.mime.eq:"text/html"`:                  true,
		`req.port.eq:443 AND req.query.eq:"x=1"`:    true,
		`"hunter2"`:                                 true,
		`row.asset.eq:true`:                         false,
		`req.created_at.lt:"2000-01-01"`:            false,
		`row.finding.ne:""`:                         false,
		`row.finding.eq:""`:                         true,
	} {
		parsed, err := Schema.Parse(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if got := parsed.Match(r); got != want {
			t.Errorf("%s = %v", q, got)
		}
	}
}

func TestFinding(t *testing.T) {
	p, _ := project.Open(t.TempDir())
	e, _ := p.Add(project.Entry{Method: "GET", URL: "https://api.test/app.js"}, []byte("GET /app.js HTTP/1.1\r\n\r\n"), nil)
	other, _ := p.Add(project.Entry{Method: "GET", URL: "https://api.test/"}, []byte("GET / HTTP/1.1\r\n\r\n"), nil)
	p.AddSighting(project.Sighting{Key: "secrets:aws:1", Detector: "secrets", Title: "aws-access-token", Entry: e.ID})
	p.AddSighting(project.Sighting{Key: "cookies:x", Detector: "cookies", Title: "sid: no HttpOnly", Entry: other.ID})
	p.SetTriage("cookies:x", project.Triage{Status: project.StatusNoise})
	for q, want := range map[string][2]bool{
		`row.finding.ne:""`:               {true, false},
		`row.finding.eq:"secrets"`:        {true, false},
		`row.finding.cont:"aws"`:          {true, false},
		`row.finding.ncont:"aws"`:         {false, true},
		`preset:"x" OR row.asset.eq:true`: {false, false},
	} {
		parsed, err := Schema.ParsePresets(q, map[string]string{"x": "row.finding.eq:\"nope\""})
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		for i, en := range []project.Entry{e, other} {
			if got := parsed.Match(New(p, en, project.Meta{})); got != want[i] {
				t.Errorf("%s on #%d = %v", q, en.ID, got)
			}
		}
	}
}
