package util

import "strings"

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// InferScheme returns "http" for port 80, "https" otherwise.
func InferScheme(host string) string {
	if strings.HasSuffix(host, ":80") {
		return "http"
	}
	return "https"
}
