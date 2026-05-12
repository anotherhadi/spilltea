package intercept

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/style"
)

type panel int

const (
	panelRequests  panel = iota
	panelResponses
)

type Model struct {
	broker *intercept.Broker
	queue  []*intercept.PendingRequest
	cursor int

	captureResponse bool
	focusedPanel    panel
	responseQueue   []*intercept.PendingResponse
	responseCursor  int

	editing             bool
	interceptEnabled    bool
	pendingEdits         map[*intercept.PendingRequest]string
	pendingResponseEdits map[*intercept.PendingResponse]string

	listViewport     viewport.Model
	responseViewport viewport.Model
	bodyViewport     viewport.Model
	textarea         textarea.Model
	pager                paginator.Model
	responsePager        paginator.Model
	help                 help.Model

	width  int
	height int
}

func New(broker *intercept.Broker) Model {
	cfg := config.Global
	ta := style.NewTextarea(false)
	ta.Blur()

	lv := style.NewViewport()
	rv := style.NewViewport()
	bv := style.NewViewport()
	p := style.NewPaginator()
	rp := style.NewPaginator()

	broker.SetCaptureResponse(cfg.Intercept.DefaultCaptureResponse)

	return Model{
		broker:           broker,
		interceptEnabled: cfg.Intercept.DefaultInterceptEnabled,
		captureResponse: cfg.Intercept.DefaultCaptureResponse,
		listViewport:    lv,
		responseViewport: rv,
		bodyViewport:    bv,
		textarea:        ta,
		pager:                p,
		responsePager:        rp,
		help:                 newHelp(),
		pendingEdits:         make(map[*intercept.PendingRequest]string),
		pendingResponseEdits: make(map[*intercept.PendingResponse]string),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) IsEditing() bool { return m.editing }

func (m Model) CurrentScheme() string {
	if len(m.queue) == 0 {
		return "https"
	}
	scheme := m.queue[m.cursor].Flow.Request.URL.Scheme
	if scheme == "" {
		return "https"
	}
	return scheme
}

func (m Model) CurrentRaw() string {
	if m.captureResponse && m.focusedPanel == panelResponses {
		if len(m.responseQueue) == 0 {
			return ""
		}
		resp := m.responseQueue[m.responseCursor]
		if edited, ok := m.pendingResponseEdits[resp]; ok {
			return edited
		}
		return formatRawResponse(resp)
	}
	if len(m.queue) == 0 {
		return ""
	}
	req := m.queue[m.cursor]
	if edited, ok := m.pendingEdits[req]; ok {
		return edited
	}
	return formatRawRequest(req)
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.recalcSizes()
}

