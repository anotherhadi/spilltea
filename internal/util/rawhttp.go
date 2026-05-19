package util

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// RawRequest holds a parsed raw HTTP request string.
type RawRequest struct {
	Method  string
	Path    string
	Proto   string
	Host    string
	Headers []RawHeader
	Body    string
}

// RawHeader is a single header key/value pair preserving insertion order.
type RawHeader struct {
	Key   string
	Value string
}

// ParseRawRequest parses a raw HTTP request string (as produced by
// FormatRawRequest). The Host header, if present, is extracted into Host
// but also kept in Headers.
func ParseRawRequest(raw string) RawRequest {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var r RawRequest
	if len(lines) == 0 {
		return r
	}

	parts := strings.SplitN(lines[0], " ", 3)
	if len(parts) >= 1 {
		r.Method = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		r.Path = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		r.Proto = strings.TrimSpace(parts[2])
	}

	i := 1
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" {
			i++
			break
		}
		if kv := strings.SplitN(line, ": ", 2); len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			r.Headers = append(r.Headers, RawHeader{k, v})
			if strings.EqualFold(k, "host") {
				r.Host = v
			}
		}
		i++
	}

	if i < len(lines) {
		r.Body = strings.TrimRight(strings.Join(lines[i:], "\n"), "\n")
	}
	return r
}

// SortedHeaderLines returns header lines sorted by key name, formatted as
// "Key: Value\n" strings. Useful for deterministic serialisation.
func SortedHeaderLines(h http.Header) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		for _, v := range h[k] {
			out = append(out, fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	return out
}
