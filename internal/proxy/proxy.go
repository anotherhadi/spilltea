package proxy

import (
	"fmt"
	"io"
	"net/http"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/plugins"
	goproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
)

type ErrMsg struct{ Err error }

func StartCmd(broker *intercept.Broker, mgr *plugins.Manager) tea.Cmd {
	return func() tea.Msg {
		if err := Start(broker, mgr); err != nil {
			return ErrMsg{Err: err}
		}
		return ErrMsg{}
	}
}

type interceptAddon struct {
	goproxy.BaseAddon
	broker  *intercept.Broker
	plugins *plugins.Manager
}

// ClientConnected disables upstream cert fetching so the upstream TCP/TLS
// connection is established only after Hold() returns, not during CONNECT.
// Without this, the upstream connection sits idle while the TUI holds the
// request, and the server closes it (keep-alive timeout) → unexpected EOF.
func (a *interceptAddon) ClientConnected(clientConn *goproxy.ClientConn) {
	clientConn.UpstreamCert = false
}

func (a *interceptAddon) Request(f *goproxy.Flow) {
	if a.plugins != nil {
		switch a.plugins.RunSyncOnRequest(f) {
		case intercept.Drop:
			f.Response = dropResponse()
			go a.plugins.RunAsyncOnRequest(f)
			return
		case intercept.Forward:
			go a.plugins.RunAsyncOnRequest(f)
			return
		}
	}

	if a.broker.Hold(f) == intercept.Drop {
		f.Response = dropResponse()
	}

	if a.plugins != nil {
		go a.plugins.RunAsyncOnRequest(f)
	}
}

func (a *interceptAddon) Response(f *goproxy.Flow) {
	if f.Response != nil {
		if len(f.Response.Body) == 0 && f.Response.BodyReader != nil {
			body, _ := io.ReadAll(f.Response.BodyReader)
			f.Response.Body = body
			f.Response.BodyReader = nil
		}
		f.Response.ReplaceToDecodedBody()
	}

	if a.plugins != nil {
		switch a.plugins.RunSyncOnResponse(f) {
		case intercept.Drop:
			a.broker.SaveEntry(f)
			f.Response = dropResponse()
			go a.plugins.RunAsyncOnResponse(f)
			return
		case intercept.Forward:
			a.broker.SaveEntry(f)
			go a.plugins.RunAsyncOnResponse(f)
			return
		}
	}

	decision := a.broker.HoldResponse(f)
	a.broker.SaveEntry(f)
	if decision == intercept.Drop {
		f.Response = dropResponse()
	}

	if a.plugins != nil {
		go a.plugins.RunAsyncOnResponse(f)
	}
}

func Start(broker *intercept.Broker, mgr *plugins.Manager) error {
	cfg := config.Global.App
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	caPath := config.ExpandPath(cfg.CertDir)

	if err := os.MkdirAll(caPath, 0o700); err != nil {
		return fmt.Errorf("ca dir: %w", err)
	}

	opts := &goproxy.Options{
		Addr:              addr,
		StreamLargeBodies: 1024 * 1024 * 5,
		CaRootPath:        caPath,
	}

	p, err := goproxy.NewProxy(opts)
	if err != nil {
		return err
	}

	p.AddAddon(&interceptAddon{broker: broker, plugins: mgr})
	return p.Start()
}

func dropResponse() *goproxy.Response {
	return &goproxy.Response{
		StatusCode: 502,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       []byte("Dropped by spilltea"),
	}
}
