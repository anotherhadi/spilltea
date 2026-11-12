package findings

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/anotherhadi/spilltea/internal/message"
)

type pattern struct {
	title string
	re    *regexp.Regexp
}

var stackTraces = []pattern{
	{"Java stack trace", regexp.MustCompile(`\bat [\w$.]+\([\w$]+\.java:\d+\)|java\.lang\.\w+(Exception|Error)\b`)},
	{"Python traceback", regexp.MustCompile(`Traceback \(most recent call last\)`)},
	{"PHP error", regexp.MustCompile(`(?i)\b(fatal error|parse error|warning|notice)\b(</b>)?:.{1,300} on line (<b>)?\d+`)},
	{".NET stack trace", regexp.MustCompile(`Server Error in '[^']*' Application|\bat [\w.]+\(.*\) in .{1,200}:line \d+`)},
	{"Node.js stack trace", regexp.MustCompile(`\bat (\S+ \()?/[^\s():]+\.[cm]?js:\d+:\d+\)?`)},
	{"Ruby stack trace", regexp.MustCompile(`\.rb:\d+:in [` + "`" + `']`)},
	{"Go panic", regexp.MustCompile(`goroutine \d+ \[running\]:`)},
	{"Django debug page", regexp.MustCompile(`You're seeing this error because you have <code>DEBUG = True</code>`)},
}

var sqlErrors = []pattern{
	{"MySQL error", regexp.MustCompile(`(?i)you have an error in your SQL syntax|\bmysqli?_\w+\(\)|MySqlException`)},
	{"PostgreSQL error", regexp.MustCompile(`(?i)\bpg_\w+\(\): |PSQLException|ERROR:\s+syntax error at or near`)},
	{"SQL Server error", regexp.MustCompile(`(?i)unclosed quotation mark after the character string|Microsoft OLE DB Provider for SQL Server|SqlException`)},
	{"Oracle error", regexp.MustCompile(`\bORA-\d{5}\b`)},
	{"SQLite error", regexp.MustCompile(`(?i)sqlite3?\.OperationalError|SQLite3::|SQLITE_ERROR`)},
	{"SQL error", regexp.MustCompile(`SQLSTATE\[\w+\]`)},
}

var versionHeaders = []string{"Server", "X-Powered-By", "X-AspNet-Version", "X-AspNetMvc-Version", "X-Generator"}

type Disclosure struct{}

func (Disclosure) Name() string { return "disclosure" }

func (Disclosure) Detect(req *message.Request, resp *message.Response) []Hit {
	if req == nil || resp == nil {
		return nil
	}
	host := req.URL.Host
	var hits []Hit
	for _, h := range versionHeaders {
		v := strings.TrimSpace(resp.Header.Get(h))
		if strings.ContainsAny(v, "0123456789") {
			hits = append(hits, Hit{Key: host + ":" + h + ":" + v, Title: h + ": " + short(v), Detail: host})
		}
	}
	if len(resp.Body) == 0 || binary(resp) {
		return hits
	}
	mime := message.Mime(resp.Header.Get("Content-Type"))
	if strings.HasSuffix(req.URL.Path, ".map") && resp.Status == 200 && sourceMap(resp.Body) {
		return append(hits, Hit{Key: host + ":source map", Title: "source map", Detail: host + "\nsources and their content may be in it"})
	}
	if strings.Contains(mime, "javascript") || mime == "text/css" || strings.HasSuffix(req.URL.Path, ".js") {
		return hits
	}
	where := host + req.URL.EscapedPath()
	for _, p := range sqlErrors {
		if loc := p.re.FindIndex(resp.Body); loc != nil {
			hits = append(hits, Hit{Key: where + ":" + p.title, Title: p.title, Detail: where + "\n" + around(resp.Body, loc)})
		}
	}
	for _, p := range stackTraces {
		if loc := p.re.FindIndex(resp.Body); loc != nil {
			hits = append(hits, Hit{Key: host + ":" + p.title, Title: p.title, Detail: where + "\n" + around(resp.Body, loc)})
		}
	}
	return hits
}

func sourceMap(b []byte) bool {
	b = bytes.TrimSpace(b[:min(len(b), 4096)])
	return bytes.HasPrefix(b, []byte("{")) && bytes.Contains(b, []byte(`"version"`)) &&
		(bytes.Contains(b, []byte(`"mappings"`)) || bytes.Contains(b, []byte(`"sources"`)))
}

func around(b []byte, loc []int) string {
	start, end := max(loc[0]-80, 0), min(loc[1]+80, len(b))
	return short(strings.Join(strings.Fields(string(b[start:end])), " "))
}
