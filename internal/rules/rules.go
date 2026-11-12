// Package rules applies the config to in-scope traffic: regexp replacements,
// added headers, User-Agent suffix.
package rules

import (
	"regexp"
	"strings"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/message"
)

type rule struct {
	body    bool
	re      *regexp.Regexp // nil: replace is added as a header
	replace string
}

type Rules struct {
	req, resp []rule
	headers   [][2]string
	uaSuffix  string
}

func New(cfg *config.Config) (*Rules, error) {
	r := &Rules{uaSuffix: strings.TrimSpace(cfg.HTTP.UserAgentSuffix)}
	for _, c := range cfg.Rules {
		ru := rule{body: c.Part == "body", replace: c.Replace}
		if c.Match != "" {
			re, err := regexp.Compile(c.Match)
			if err != nil {
				return nil, err
			}
			ru.re = re
		}
		if c.On == "response" {
			r.resp = append(r.resp, ru)
		} else {
			r.req = append(r.req, ru)
		}
	}
	for _, h := range cfg.HTTP.Headers {
		name, value, _ := strings.Cut(h, ":")
		r.headers = append(r.headers, [2]string{strings.TrimSpace(name), strings.TrimSpace(value)})
	}
	return r, nil
}

// apply expects a head with "\n" line endings.
func apply(rules []rule, head string, body []byte) (string, []byte, bool) {
	changed := false
	for _, ru := range rules {
		switch {
		case ru.body:
			if out := ru.re.ReplaceAll(body, []byte(ru.replace)); string(out) != string(body) {
				body, changed = out, true
			}
		case ru.re == nil:
			head, changed = head+"\n"+ru.replace, true
		default:
			if out := ru.re.ReplaceAllString(head, ru.replace); out != head {
				head, changed = out, true
			}
		}
	}
	return head, body, changed
}

func headOf(raw []byte) string {
	head, _ := message.Split(raw)
	return strings.TrimRight(strings.ReplaceAll(string(head), "\r\n", "\n"), "\n")
}

// Request edits req in place. If a rule makes the request unparsable, req is
// left untouched and the error is returned.
func (r *Rules) Request(req *message.Request) (bool, error) {
	changed := false
	if len(r.req) > 0 {
		head, body, ok := apply(r.req, headOf(req.Bytes()), req.Body)
		if ok {
			out, err := message.ParseRequest([]byte(head+"\n\n"+string(body)), req.URL.Scheme)
			if err != nil {
				return false, err
			}
			*req, changed = *out, true
		}
	}
	for _, h := range r.headers {
		if req.Header.Get(h[0]) != h[1] {
			req.Header.Set(h[0], h[1])
			changed = true
		}
	}
	if ua := req.Header.Get("User-Agent"); r.uaSuffix != "" && ua != "" && !strings.HasSuffix(ua, " "+r.uaSuffix) {
		req.Header.Set("User-Agent", ua+" "+r.uaSuffix)
		changed = true
	}
	return changed, nil
}

func (r *Rules) Response(resp *message.Response) (bool, error) {
	if len(r.resp) == 0 {
		return false, nil
	}
	head, body, ok := apply(r.resp, headOf(resp.Head()), resp.Body)
	if !ok {
		return false, nil
	}
	out, err := message.ParseResponse([]byte(head + "\n\n" + string(body)))
	if err != nil {
		return false, err
	}
	*resp = *out
	return true, nil
}
