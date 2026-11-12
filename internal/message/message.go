// Package message converts HTTP messages to and from the raw text spilltea
// shows, stores and edits.
package message

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// PwnFoxHeader holds the color of the PwnFox Firefox container.
const PwnFoxHeader = "X-PwnFox-Color"

type Request struct {
	Method string
	URL    *url.URL
	Proto  string // "HTTP/1.1", "HTTP/2.0"
	Header http.Header
	Body   []byte
}

type Response struct {
	Proto  string
	Status int
	Header http.Header
	Body   []byte
}

func proto(p string) string {
	if p == "" {
		return "HTTP/1.1"
	}
	return p
}

func target(u *url.URL) string {
	t := u.EscapedPath()
	if t == "" {
		t = "/"
	}
	if u.RawQuery != "" || u.ForceQuery {
		t += "?" + u.RawQuery
	}
	return t
}

// headerLines writes headers in a stable order so two messages diff line by
// line: the ones in first, then the rest sorted.
func headerLines(b *bytes.Buffer, h http.Header, first ...string) {
	names := make([]string, 0, len(h))
	for k := range h {
		names = append(names, k)
	}
	rank := func(k string) int {
		for i, f := range first {
			if strings.EqualFold(f, k) {
				return i
			}
		}
		return len(first)
	}
	slices.SortStableFunc(names, func(a, c string) int {
		if ra, rc := rank(a), rank(c); ra != rc {
			return ra - rc
		}
		return strings.Compare(a, c)
	})
	for _, k := range names {
		for _, v := range h[k] {
			fmt.Fprintf(b, "%s: %s\r\n", k, v)
		}
	}
}

func (r *Request) Bytes() []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s %s\r\n", r.Method, target(r.URL), proto(r.Proto))
	h := r.Header.Clone()
	if h == nil {
		h = http.Header{}
	}
	if h.Get("Host") == "" {
		h.Set("Host", r.URL.Host)
	}
	headerLines(&b, h, "Host")
	b.WriteString("\r\n")
	b.Write(r.Body)
	return b.Bytes()
}

func (r *Response) Bytes() []byte {
	var b bytes.Buffer
	b.Write(r.Head())
	b.Write(r.Body)
	return b.Bytes()
}

// Head includes the blank line.
func (r *Response) Head() []byte {
	var b bytes.Buffer
	text := http.StatusText(r.Status)
	fmt.Fprintf(&b, "%s %d %s\r\n", proto(r.Proto), r.Status, text)
	headerLines(&b, r.Header)
	b.WriteString("\r\n")
	return b.Bytes()
}

// Split cuts at the first blank line ("\r\n\r\n" or "\n\n"); body is nil if
// there is none.
func Split(raw []byte) (head, body []byte) {
	i, j := bytes.Index(raw, []byte("\r\n\r\n")), bytes.Index(raw, []byte("\n\n"))
	switch {
	case i >= 0 && (j < 0 || i <= j):
		return raw[:i], raw[i+4:]
	case j >= 0:
		return raw[:j], raw[j+2:]
	}
	return raw, nil
}

func readHead(head []byte) (first string, h http.Header, err error) {
	lines := strings.Split(strings.ReplaceAll(string(head), "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	if len(lines) == 0 {
		return "", nil, errors.New("empty message")
	}
	h = http.Header{}
	for _, l := range lines[1:] {
		name, value, ok := strings.Cut(l, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return "", nil, fmt.Errorf("%q is not a header", l)
		}
		h.Add(textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name)), strings.TrimSpace(value))
	}
	return lines[0], h, nil
}

// ParseRequest sends a path-only target to the Host header, using scheme.
func ParseRequest(raw []byte, scheme string) (*Request, error) {
	head, body := Split(raw)
	first, h, err := readHead(head)
	if err != nil {
		return nil, err
	}
	f := strings.Fields(first)
	if len(f) != 3 || !strings.HasPrefix(f[2], "HTTP/") {
		return nil, fmt.Errorf("%q is not a request line", first)
	}
	r := &Request{Method: f[0], Proto: f[2], Header: h, Body: body}
	if strings.HasPrefix(f[1], "http://") || strings.HasPrefix(f[1], "https://") {
		r.URL, err = url.Parse(f[1])
		if err == nil && r.URL.Host == "" {
			err = fmt.Errorf("%q has no host", f[1])
		}
		if err == nil {
			// RFC 9112: the target's host wins over the Host header
			h.Set("Host", r.URL.Host)
		}
	} else {
		// without a leading "/" the target would be glued to the host:
		// "GET @evil.test/" would go to evil.test
		if !strings.HasPrefix(f[1], "/") && f[1] != "*" {
			return nil, fmt.Errorf("%q is not a path", f[1])
		}
		host := h.Get("Host")
		if host == "" {
			return nil, errors.New("no Host header")
		}
		if strings.ContainsAny(host, "/?#@\\ ") {
			return nil, fmt.Errorf("%q is not a host", host)
		}
		if f[1] == "*" {
			r.URL = &url.URL{Scheme: scheme, Host: host, Path: "*"}
		} else {
			r.URL, err = url.Parse(scheme + "://" + host + f[1])
		}
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func ParseResponse(raw []byte) (*Response, error) {
	head, body := Split(raw)
	first, h, err := readHead(head)
	if err != nil {
		return nil, err
	}
	f := strings.Fields(first)
	if len(f) < 2 || !strings.HasPrefix(f[0], "HTTP/") {
		return nil, fmt.Errorf("%q is not a status line", first)
	}
	code, err := strconv.Atoi(f[1])
	if err != nil || code < 100 || code > 999 {
		return nil, fmt.Errorf("%q is not a status code", f[1])
	}
	return &Response{Proto: f[0], Status: code, Header: h, Body: body}, nil
}

// FixLength resets Content-Length: an edited message is sent whole, neither
// chunked nor compressed.
func FixLength(h http.Header, body []byte, hadBody bool) {
	h.Del("Transfer-Encoding")
	if len(body) > 0 || hadBody || h.Get("Content-Length") != "" {
		h.Set("Content-Length", strconv.Itoa(len(body)))
	}
}

// Text is the message as the editor shows it: "\n" in the head, body untouched.
func Text(raw []byte) string {
	head, body := Split(raw)
	s := strings.ReplaceAll(string(head), "\r\n", "\n")
	if body == nil {
		return s
	}
	return s + "\n\n" + string(body)
}

// Ext returns the last segment's extension, lowercased, without the dot.
func Ext(u *url.URL) string {
	p := u.Path
	seg := p[strings.LastIndex(p, "/")+1:]
	if i := strings.LastIndex(seg, "."); i >= 0 {
		return strings.ToLower(seg[i+1:])
	}
	return ""
}

// Mime strips the parameters of a Content-Type (; charset=...).
func Mime(contentType string) string {
	t, _, _ := strings.Cut(contentType, ";")
	return strings.ToLower(strings.TrimSpace(t))
}
