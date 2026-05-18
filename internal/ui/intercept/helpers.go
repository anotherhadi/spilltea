package intercept

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/anotherhadi/spilltea/internal/intercept"
	"github.com/anotherhadi/spilltea/internal/style"
)

func formatRawRequest(req *intercept.PendingRequest) string {
	r := req.Flow.Request
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s %s %s\n", r.Method, r.URL.RequestURI(), r.Proto)

	keys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range r.Header[k] {
			fmt.Fprintf(&sb, "%s: %s\n", k, v)
		}
	}

	sb.WriteString("\n")
	if len(r.Body) > 0 {
		sb.Write(r.Body)
	}
	return sb.String()
}

func formatRawResponse(resp *intercept.PendingResponse) string {
	r := resp.Flow.Response
	if r == nil {
		return "(no response)"
	}
	var sb strings.Builder

	proto := resp.Flow.Request.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	fmt.Fprintf(&sb, "%s %d %s\n", proto, r.StatusCode, http.StatusText(r.StatusCode))

	keys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range r.Header[k] {
			fmt.Fprintf(&sb, "%s: %s\n", k, v)
		}
	}

	sb.WriteString("\n")
	if len(r.Body) > 0 {
		sb.Write(r.Body)
	}
	return sb.String()
}

func parseRawRequest(content string, req *intercept.PendingRequest) {
	r := req.Flow.Request
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return
	}

	parts := strings.SplitN(lines[0], " ", 3)
	if len(parts) >= 1 {
		r.Method = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		if u, err := url.ParseRequestURI(strings.TrimSpace(parts[1])); err == nil {
			r.URL.Path = u.Path
			r.URL.RawQuery = u.RawQuery
		}
	}
	if len(parts) >= 3 {
		r.Proto = strings.TrimSpace(parts[2])
	}

	r.Header = make(http.Header)
	i := 1
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" {
			i++
			break
		}
		if kv := strings.SplitN(line, ": ", 2); len(kv) == 2 {
			r.Header.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
		}
		i++
	}

	if i < len(lines) {
		body := strings.Join(lines[i:], "\n")
		body = strings.TrimRight(body, "\n")
		if body != "" {
			r.Body = []byte(body)
		} else {
			r.Body = nil
		}
	}
}

func parseRawResponse(content string, resp *intercept.PendingResponse) {
	r := resp.Flow.Response
	if r == nil {
		return
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return
	}

	parts := strings.SplitN(lines[0], " ", 3)
	if len(parts) >= 2 {
		if code, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			r.StatusCode = code
		}
	}

	r.Header = make(http.Header)
	i := 1
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" {
			i++
			break
		}
		if kv := strings.SplitN(line, ": ", 2); len(kv) == 2 {
			r.Header.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
		}
		i++
	}

	if i < len(lines) {
		body := strings.Join(lines[i:], "\n")
		body = strings.TrimRight(body, "\n")
		if body != "" {
			r.Body = []byte(body)
		} else {
			r.Body = nil
		}
	}
	r.Header.Set("Content-Length", strconv.Itoa(len(r.Body)))
}

func (m *Model) currentLabel() string {
	if m.captureResponse && m.focusedPanel == panelResponses {
		if len(m.responseQueue) == 0 {
			return ""
		}
		resp := m.responseQueue[m.responseCursor]
		code := 0
		if resp.Flow.Response != nil {
			code = resp.Flow.Response.StatusCode
		}
		return fmt.Sprintf("%d %s %s", code, http.StatusText(code), resp.Flow.Request.URL.RequestURI())
	}
	if len(m.queue) == 0 {
		return ""
	}
	req := m.queue[m.cursor]
	return req.Flow.Request.Method + " " + req.Flow.Request.URL.RequestURI()
}

func (m *Model) removeFromQueue(index int) {
	m.queue = append(m.queue[:index], m.queue[index+1:]...)
	if m.cursor >= len(m.queue) && m.cursor > 0 {
		m.cursor--
	}
	m.refreshListViewport()
	m.refreshBody()
}

func (m *Model) removeFromResponseQueue(index int) {
	m.responseQueue = append(m.responseQueue[:index], m.responseQueue[index+1:]...)
	if m.responseCursor >= len(m.responseQueue) && m.responseCursor > 0 {
		m.responseCursor--
	}
	m.refreshResponseListViewport()
	m.refreshBody()
}

func (m *Model) applyAndDecide(d intercept.Decision) {
	if len(m.queue) == 0 {
		return
	}
	req := m.queue[m.cursor]
	if d == intercept.Forward {
		if edited, ok := m.pendingEdits[req]; ok {
			parseRawRequest(edited, req)
		}
	}
	delete(m.pendingEdits, req)
	m.broker.Decide(req, d)
	m.removeFromQueue(m.cursor)
}

func (m *Model) applyAndDecideResponse(d intercept.Decision) {
	if len(m.responseQueue) == 0 {
		return
	}
	resp := m.responseQueue[m.responseCursor]
	if d == intercept.Forward {
		if edited, ok := m.pendingResponseEdits[resp]; ok {
			parseRawResponse(edited, resp)
		}
	}
	delete(m.pendingResponseEdits, resp)
	m.broker.DecideResponse(resp, d)
	m.removeFromResponseQueue(m.responseCursor)
}

func (m *Model) listHalfWidths() (leftW, rightW int) {
	leftW = m.width / 2
	rightW = m.width - leftW
	return
}

func (m *Model) recalcSizes() {
	m.help.SetWidth(m.width - 2)

	listH, bodyH := style.SplitH(m.height, m.renderStatusBar(), 0.35)

	bodyInner := m.width - 2
	if bodyInner < 0 {
		bodyInner = 0
	}
	bodyVH := style.PanelContentH(bodyH)

	m.textarea.SetWidth(bodyInner)
	m.textarea.SetHeight(bodyVH)
	m.bodyViewport.SetWidth(bodyInner)
	m.bodyViewport.SetHeight(bodyVH)

	listVH := style.PanelContentH(listH) - 1 // -1 for the pager dots row
	if listVH < 0 {
		listVH = 0
	}

	if m.captureResponse {
		leftW, rightW := m.listHalfWidths()
		leftInner := leftW - 2
		rightInner := rightW - 2
		if leftInner < 0 {
			leftInner = 0
		}
		if rightInner < 0 {
			rightInner = 0
		}

		m.listViewport.SetWidth(leftInner)
		m.listViewport.SetHeight(listVH)
		m.pager.PerPage = listVH
		if m.pager.PerPage < 1 {
			m.pager.PerPage = 1
		}

		m.responseViewport.SetWidth(rightInner)
		m.responseViewport.SetHeight(listVH)
		m.responsePager.PerPage = listVH
		if m.responsePager.PerPage < 1 {
			m.responsePager.PerPage = 1
		}
	} else {
		listInner := m.width - 2
		if listInner < 0 {
			listInner = 0
		}

		m.listViewport.SetWidth(listInner)
		m.listViewport.SetHeight(listVH)
		m.pager.PerPage = listVH
		if m.pager.PerPage < 1 {
			m.pager.PerPage = 1
		}
	}

	m.refreshListViewport()
	m.refreshResponseListViewport()
	m.refreshBody()
}

func (m *Model) refreshListViewport() {
	if m.pager.PerPage > 0 {
		if len(m.queue) == 0 {
			m.pager.Page = 0
			m.pager.TotalPages = 0
		} else {
			m.pager.Page = m.cursor / m.pager.PerPage
			m.pager.SetTotalPages(len(m.queue))
		}
	}
	m.listViewport.SetContent(m.renderList())
}

func (m *Model) refreshResponseListViewport() {
	if m.responsePager.PerPage > 0 {
		if len(m.responseQueue) == 0 {
			m.responsePager.Page = 0
			m.responsePager.TotalPages = 0
		} else {
			m.responsePager.Page = m.responseCursor / m.responsePager.PerPage
			m.responsePager.SetTotalPages(len(m.responseQueue))
		}
	}
	m.responseViewport.SetContent(m.renderResponseList())
}

// saveCurrentEdit must only be called when exiting edit mode.
func (m *Model) saveCurrentEdit() {
	if m.captureResponse && m.focusedPanel == panelResponses {
		if len(m.responseQueue) > 0 {
			m.pendingResponseEdits[m.responseQueue[m.responseCursor]] = m.textarea.Value()
		}
	} else {
		if len(m.queue) > 0 {
			m.pendingEdits[m.queue[m.cursor]] = m.textarea.Value()
		}
	}
}

const maxInlineEditBytes = 32 * 1024

func (m *Model) loadIntoTextarea() {
	if m.captureResponse && m.focusedPanel == panelResponses {
		if len(m.responseQueue) == 0 {
			return
		}
		resp := m.responseQueue[m.responseCursor]
		if edited, ok := m.pendingResponseEdits[resp]; ok {
			m.textarea.SetValue(edited)
		} else {
			m.textarea.SetValue(formatRawResponse(resp))
		}
	} else {
		if len(m.queue) == 0 {
			return
		}
		req := m.queue[m.cursor]
		if edited, ok := m.pendingEdits[req]; ok {
			m.textarea.SetValue(edited)
		} else {
			m.textarea.SetValue(formatRawRequest(req))
		}
	}
}

// refreshBody does not touch the textarea - it is only loaded when entering edit mode.
func (m *Model) refreshBody() {
	var raw string
	if m.captureResponse && m.focusedPanel == panelResponses {
		if len(m.responseQueue) == 0 {
			m.bodyViewport.SetContent("")
			return
		}
		resp := m.responseQueue[m.responseCursor]
		if edited, ok := m.pendingResponseEdits[resp]; ok {
			raw = edited
		} else {
			raw = formatRawResponse(resp)
		}
	} else {
		if len(m.queue) == 0 {
			m.bodyViewport.SetContent("")
			return
		}
		req := m.queue[m.cursor]
		if edited, ok := m.pendingEdits[req]; ok {
			raw = edited
		} else {
			raw = formatRawRequest(req)
		}
	}
	m.bodyViewport.SetContent(style.HighlightHTTP(raw))
	m.bodyViewport.SetYOffset(0)
	m.bodyViewport.SetXOffset(0)
}

func (m *Model) refreshBodyViewport() {
	m.bodyViewport.SetContent(style.HighlightHTTP(m.textarea.Value()))
}
