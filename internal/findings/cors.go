package findings

import (
	"strings"

	"github.com/anotherhadi/spilltea/internal/message"
)

type CORS struct{}

func (CORS) Name() string { return "cors" }

// The allowed origin is usually the app's own front: a lead, not a proof.
func (CORS) Detect(req *message.Request, resp *message.Response) []Hit {
	if req == nil || resp == nil {
		return nil
	}
	allowed := strings.TrimSpace(resp.Header.Get("Access-Control-Allow-Origin"))
	creds := strings.EqualFold(strings.TrimSpace(resp.Header.Get("Access-Control-Allow-Credentials")), "true")
	if allowed == "" || allowed == "*" || !creds {
		return nil
	}
	host := req.URL.Host
	title := "credentialed CORS"
	if allowed == "null" {
		title = "null origin trusted with credentials"
	}
	detail := "Access-Control-Allow-Origin: " + short(allowed) + "\nAccess-Control-Allow-Credentials: true"
	if o := req.Header.Get("Origin"); o != "" {
		detail += "\nrequest Origin: " + short(o)
	}
	return []Hit{{Key: host + ":" + title, Title: title, Detail: host + "\n" + detail}}
}
