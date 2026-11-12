// Package curl turns a request into a curl command.
package curl

import (
	"slices"
	"strings"

	"github.com/anotherhadi/spilltea/internal/message"
)

// Format puts one option per line. Host is already in the URL, curl computes
// Content-Length itself, and Accept-Encoding becomes --compressed.
func Format(req *message.Request) string {
	parts := []string{"curl"}
	if req.Method != "GET" || len(req.Body) > 0 {
		parts = append(parts, "-X "+quote(req.Method))
	}
	u := req.URL.String()
	parts = append(parts, quote(u))
	if strings.ContainsAny(u, "[]{}") {
		parts = append(parts, "--globoff")
	}
	if strings.HasPrefix(req.Proto, "HTTP/2") {
		parts = append(parts, "--http2")
	}
	names := make([]string, 0, len(req.Header))
	for k := range req.Header {
		names = append(names, k)
	}
	slices.Sort(names)
	compressed := false
	for _, k := range names {
		switch strings.ToLower(k) {
		case "host", "content-length":
			continue
		case "accept-encoding":
			compressed = true
			continue
		}
		for _, v := range req.Header[k] {
			parts = append(parts, "-H "+quote(k+": "+v))
		}
	}
	if compressed {
		parts = append(parts, "--compressed")
	}
	if len(req.Body) > 0 {
		// --data-binary would read a file from a body starting with @
		parts = append(parts, "--data-raw "+quote(string(req.Body)))
	}
	return strings.Join(parts, " \\\n  ")
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
