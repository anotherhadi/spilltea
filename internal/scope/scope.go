// Package scope decides what spilltea works on. A pattern is
// host[:port][/path], with * matching any run of characters: *.example.com,
// api.example.com:8443, example.com/api. Hosts are case-insensitive, paths
// are prefixes.
package scope

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type pattern struct {
	host *regexp.Regexp
	port int // 0: any
	path string
	re   *regexp.Regexp // only when the path has a *
}

type Scope struct {
	include, exclude []pattern
}

func New(include, exclude []string) (*Scope, error) {
	s := &Scope{}
	for _, list := range []struct {
		name string
		in   []string
		out  *[]pattern
	}{{"scope.include", include, &s.include}, {"scope.exclude", exclude, &s.exclude}} {
		for i, p := range list.in {
			pat, err := parse(p)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", list.name, i, err)
			}
			*list.out = append(*list.out, pat)
		}
	}
	return s, nil
}

func glob(s string, anchorEnd bool) *regexp.Regexp {
	parts := strings.Split(s, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	end := ""
	if anchorEnd {
		end = "$"
	}
	return regexp.MustCompile("(?i)^" + strings.Join(parts, ".*") + end)
}

func parse(s string) (pattern, error) {
	s = strings.TrimSpace(s)
	if !utf8.ValidString(s) {
		return pattern{}, fmt.Errorf("%q is not valid UTF-8", s)
	}
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	host, path, _ := strings.Cut(s, "/")
	if host == "" {
		return pattern{}, fmt.Errorf("%q has no host", s)
	}
	var p pattern
	if h, port, ok := strings.Cut(host, ":"); ok {
		n, err := strconv.Atoi(port)
		if err != nil || n <= 0 || n > 65535 {
			return pattern{}, fmt.Errorf("%q: bad port %q", s, port)
		}
		host, p.port = h, n
	}
	p.host = glob(strings.ToLower(host), true)
	if path != "" {
		p.path = "/" + path
		if strings.Contains(path, "*") {
			p.re = glob(p.path, false)
		}
	}
	return p, nil
}

func (p pattern) hostMatch(host string, port int) bool {
	return p.host.MatchString(strings.ToLower(host)) && (p.port == 0 || p.port == port)
}

func (p pattern) match(host string, port int, path string) bool {
	if !p.hostMatch(host, port) {
		return false
	}
	switch {
	case p.path == "":
		return true
	case p.re != nil:
		return p.re.MatchString(path)
	}
	return strings.HasPrefix(path, p.path)
}

func (s *Scope) Empty() bool { return len(s.include) == 0 && len(s.exclude) == 0 }

// Host reports whether host:port may have anything in scope: only those hosts
// get decrypted. An exclude with a path does not exclude the whole host.
func (s *Scope) Host(host string, port int) bool {
	for _, p := range s.exclude {
		if p.path == "" && p.hostMatch(host, port) {
			return false
		}
	}
	if len(s.include) == 0 {
		return true
	}
	for _, p := range s.include {
		if p.hostMatch(host, port) {
			return true
		}
	}
	return false
}

func (s *Scope) URL(u *url.URL) bool {
	host, port := u.Hostname(), Port(u)
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	for _, p := range s.exclude {
		if p.match(host, port, path) {
			return false
		}
	}
	if len(s.include) == 0 {
		return true
	}
	for _, p := range s.include {
		if p.match(host, port, path) {
			return true
		}
	}
	return false
}

// Port falls back to the scheme's default port.
func Port(u *url.URL) int {
	if n, err := strconv.Atoi(u.Port()); err == nil {
		return n
	}
	if u.Scheme == "http" {
		return 80
	}
	return 443
}
