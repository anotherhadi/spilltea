package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/foam/keys"

	"github.com/anotherhadi/spilltea/internal/ui/reader"
)

func (m *Model) reader(p pane) *reader.Model {
	switch p {
	case paneHistRequest:
		return m.histReq
	case paneHistResponse:
		return m.histRes
	case paneSiteRequest:
		return m.siteReq
	case paneSiteResponse:
		return m.siteRes
	}
	return nil
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	if !minSize.Fits(m.w, m.h) {
		// window too small: only quitting works
		if b, ok := m.reg.Match(m.scopes()[:1], msg); ok && b.ID == "app.quit" {
			return b.Run()
		}
		return nil
	}
	if m.overlays.Top() != nil {
		return m.overlays.Key(m.reg, msg)
	}
	switch p := m.focus[m.page]; p {
	case paneDetail:
		// vim goes first while busy, and for printable keys: here f, x, s
		// belong to vim, the intercept uses ctrl+f, ctrl+x, ctrl+s
		if (m.inter.Busy() || keys.Printable(msg)) && m.inter.Key(msg) {
			return nil
		}
	case paneHistory, paneSite, paneSeen:
		if l := m.list(); l.Capturing() {
			if ok, cmd := l.Input(msg); ok {
				return cmd
			}
		}
	case paneTree:
		if m.tree.Capturing() && m.tree.Input(msg) {
			return nil
		}
	case paneHistRequest, paneHistResponse, paneSiteRequest, paneSiteResponse:
		if r := m.reader(p); r.Capturing() {
			if ok, cmd := r.Input(msg); ok {
				return cmd
			}
		}
	}
	if b, ok := m.reg.Match(m.scopes(), msg); ok {
		return b.Run()
	}
	if m.focus[m.page] == paneDetail {
		m.inter.Key(msg)
	}
	return nil
}

func (m *Model) paste(s string) tea.Cmd {
	if o := m.overlays.Top(); o != nil {
		return o.Update(tea.PasteMsg{Content: s})
	}
	switch p := m.focus[m.page]; p {
	case paneDetail:
		m.inter.Paste(s)
	case paneHistory, paneSite, paneSeen:
		return m.list().Paste(s)
	case paneTree:
		m.tree.Paste(s)
	case paneHistRequest, paneHistResponse, paneSiteRequest, paneSiteResponse:
		m.reader(p).Paste(s)
	}
	return nil
}

func (m *Model) mouse(msg tea.MouseMsg) tea.Cmd {
	if !minSize.Fits(m.w, m.h) {
		return nil
	}
	if m.overlays.Top() != nil {
		return m.overlays.Mouse(msg)
	}
	mm := msg.Mouse()
	if _, click := msg.(tea.MouseClickMsg); click {
		for _, t := range m.toastRects {
			if t.Contains(mm.X, mm.Y) {
				m.notes.DismissID(t.id)
				return nil
			}
		}
		if mm.Y == 0 {
			if p, ok := m.tabAt(mm.X); ok {
				m.setPage(p)
				return nil
			}
		}
	}
	for _, p := range pagePanes[m.page] {
		r := m.rects[p]
		if r.Empty() || !r.Contains(mm.X, mm.Y) {
			continue
		}
		wasFocused := m.focus[m.page] == p
		if _, click := msg.(tea.MouseClickMsg); click {
			m.setFocus(p)
		}
		mm.X, mm.Y = mm.X-r.X-1, mm.Y-r.Y-1
		var local tea.MouseMsg
		switch msg.(type) {
		case tea.MouseClickMsg:
			local = tea.MouseClickMsg(mm)
		case tea.MouseWheelMsg:
			local = tea.MouseWheelMsg(mm)
		default:
			return nil
		}
		switch p {
		case paneRequests, paneResponses:
			q := &m.inter.Requests
			if p == paneResponses {
				q = &m.inter.Responses
			}
			q.List().Mouse(local)
		case paneDetail:
			m.inter.Mouse(local)
		case paneHistory:
			return m.hist.Mouse(local, wasFocused)
		case paneSite:
			return m.site.Mouse(local, wasFocused)
		case paneTree:
			return m.tree.Mouse(local, wasFocused)
		case paneHistRequest, paneHistResponse, paneSiteRequest, paneSiteResponse:
			m.reader(p).Mouse(local)
		case paneDiffA, paneDiffB:
			m.diff.Mouse(local)
		case paneFindings:
			return m.finds.Mouse(local, wasFocused)
		case paneFinding:
			return m.findView.Mouse(local)
		case paneSeen:
			return m.seen.Mouse(local, wasFocused)
		}
		return nil
	}
	return nil
}
