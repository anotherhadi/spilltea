package findings

import (
	"net/http"
	"strings"
	"time"

	"github.com/anotherhadi/spilltea/internal/message"
)

type Cookies struct{}

func (Cookies) Name() string { return "cookies" }

func (Cookies) Detect(req *message.Request, resp *message.Response) []Hit {
	if req == nil || resp == nil {
		return nil
	}
	host := req.URL.Hostname()
	var hits []Hit
	for _, line := range resp.Header.Values("Set-Cookie") {
		c, err := http.ParseSetCookie(line)
		if err != nil || c.Value == "" || c.MaxAge < 0 || !c.Expires.IsZero() && c.Expires.Before(time.Now()) {
			continue
		}
		var issues []string
		if req.URL.Scheme == "https" && !c.Secure {
			issues = append(issues, "no Secure")
		}
		if !c.HttpOnly {
			issues = append(issues, "no HttpOnly")
		}
		if c.SameSite == http.SameSiteNoneMode {
			issues = append(issues, "SameSite=None")
		}
		if len(issues) == 0 {
			continue
		}
		domain := host
		if c.Domain != "" {
			domain = strings.TrimPrefix(c.Domain, ".")
		}
		what := strings.Join(issues, ", ")
		hits = append(hits, Hit{
			Key:    domain + ":" + c.Name + ":" + what,
			Title:  c.Name + ": " + what,
			Detail: domain + "\nSet-Cookie: " + short(line),
		})
	}
	return hits
}
