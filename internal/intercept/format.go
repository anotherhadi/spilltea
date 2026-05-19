package intercept

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/anotherhadi/spilltea/internal/util"
	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

// FormatRawRequest serialises a flow's request to a raw HTTP string.
func FormatRawRequest(f *proxy.Flow) string {
	r := f.Request
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s %s\n", r.Method, r.URL.RequestURI(), r.Proto)
	for _, line := range util.SortedHeaderLines(r.Header) {
		sb.WriteString(line)
	}
	sb.WriteString("\n")
	if len(r.Body) > 0 {
		sb.Write(r.Body)
	}
	return sb.String()
}

// FormatRawResponse serialises a flow's response to a raw HTTP string.
func FormatRawResponse(f *proxy.Flow) string {
	r := f.Response
	if r == nil {
		return "(no response)"
	}
	var sb strings.Builder
	proto := f.Request.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	fmt.Fprintf(&sb, "%s %d %s\n", proto, r.StatusCode, http.StatusText(r.StatusCode))
	for _, line := range util.SortedHeaderLines(r.Header) {
		sb.WriteString(line)
	}
	sb.WriteString("\n")
	if len(r.Body) > 0 {
		sb.Write(r.Body)
	}
	return sb.String()
}
