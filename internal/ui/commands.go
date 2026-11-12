package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/foam/commands"
	"github.com/anotherhadi/foam/confirm"
	"github.com/anotherhadi/foam/geom"
	"github.com/anotherhadi/foam/help"
	"github.com/anotherhadi/foam/notify"
	"github.com/anotherhadi/foam/overlay"
	"github.com/anotherhadi/foam/palette"
	"github.com/anotherhadi/foam/prompt"

	pfindings "github.com/anotherhadi/spilltea/internal/findings"
	"github.com/anotherhadi/spilltea/internal/proxy"
	"github.com/anotherhadi/spilltea/internal/ui/intercept"
	"github.com/anotherhadi/spilltea/internal/ui/picker"
)

func (m *Model) global() []commands.Command {
	toPage := func(p page) func() tea.Cmd { return func() tea.Cmd { m.setPage(p); return nil } }
	nav := func(d geom.Dir) func() tea.Cmd { return func() tea.Cmd { m.navigate(d); return nil } }
	toggle := func(k proxy.Kind) func() tea.Cmd {
		return func() tea.Cmd {
			if m.px == nil {
				return nil
			}
			on := !m.px.Holding(k)
			var cmds []tea.Cmd
			if !on {
				// send them here with their edits, the proxy would send
				// them as held
				for _, it := range m.inter.Queue(k).Items() {
					cmds = append(cmds, m.decide(it, proxy.Decision{}))
				}
			}
			m.px.SetHolding(k, on)
			return tea.Batch(cmds...)
		}
	}
	return []commands.Command{
		{ID: "app.quit", Title: "Quit", Run: m.quit},
		{ID: "app.help", Title: "Show key bindings", Help: "help", Run: func() tea.Cmd {
			return overlay.Open(help.New(m.reg.Groups(m.scopes())))
		}},
		{ID: "app.palette", Title: "Command palette", Run: func() tea.Cmd {
			return overlay.Open(palette.New(m.reg.Active(m.scopes()), "app.palette"))
		}},
		{ID: "page.intercept", Title: "Go to the intercept", Run: toPage(pageIntercept)},
		{ID: "page.history", Title: "Go to the history", Run: toPage(pageHistory)},
		{ID: "page.sitemap", Title: "Go to the sitemap", Run: toPage(pageSitemap)},
		{ID: "page.diff", Title: "Go to the diff", Run: toPage(pageDiff)},
		{ID: "page.findings", Title: "Go to the findings", Run: toPage(pageFindings)},
		{ID: "page.next", Title: "Go to the next page", Run: func() tea.Cmd { m.setPage((m.page + 1) % pageCount); return nil }},
		{ID: "page.prev", Title: "Go to the previous page", Run: func() tea.Cmd { m.setPage((m.page + pageCount - 1) % pageCount); return nil }},
		{ID: "pane.left", Title: "Go to the pane on the left", Run: nav(geom.Left)},
		{ID: "pane.down", Title: "Go to the pane below", Run: nav(geom.Down)},
		{ID: "pane.up", Title: "Go to the pane above", Run: nav(geom.Up)},
		{ID: "pane.right", Title: "Go to the pane on the right", Run: nav(geom.Right)},
		{ID: "pane.zoom", Title: "Toggle pane zoom", Run: func() tea.Cmd { m.zoomed = !m.zoomed; m.layout(); return nil }},
		{ID: "intercept.requests", Title: "Toggle request interception", Run: toggle(proxy.KindRequest)},
		{ID: "intercept.responses", Title: "Toggle response interception", Run: toggle(proxy.KindResponse)},
	}
}

// item is what the keys act on: the focused queue's selection, otherwise the
// item in the editor (which may come from the other queue if this one is empty).
func (m *Model) item() *proxy.Item {
	var it *proxy.Item
	switch m.focus[pageIntercept] {
	case paneRequests:
		it = m.inter.Requests.Selected()
	case paneResponses:
		it = m.inter.Responses.Selected()
	}
	if it == nil {
		it = m.inter.Shown()
	}
	return it
}

func (m *Model) decide(it *proxy.Item, d proxy.Decision) tea.Cmd {
	if !d.Drop {
		if err := m.inter.Check(it); err != nil {
			return notify.Error(fmt.Sprintf("#%d: %v", it.Seq, err))
		}
		d.Raw = m.inter.Draft(it)
		d.HoldResponse = m.inter.Holds(it)
	}
	if m.px != nil {
		m.px.Decide(it, d)
	}
	m.inter.Remove(it)
	return nil
}

func (m *Model) focusedQueue() *intercept.Queue {
	if it := m.item(); it != nil {
		return m.inter.Queue(it.Kind)
	}
	return m.inter.Queue(proxy.KindRequest)
}

// decideAllMsg goes through a message because a confirmation's answer must
// change the model inside its own loop. Items already released are skipped.
type decideAllMsg struct {
	items []*proxy.Item
	drop  bool
}

func (m *Model) decideAll(msg decideAllMsg) tea.Cmd {
	var cmds []tea.Cmd
	for _, it := range msg.items {
		if m.inter.Has(it) {
			cmds = append(cmds, m.decide(it, proxy.Decision{Drop: msg.drop}))
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) interceptCommands() []commands.Command {
	has := func() bool { return m.item() != nil }
	isRequest := func() bool { it := m.item(); return it != nil && it.Kind == proxy.KindRequest }
	with := func(f func(*proxy.Item) tea.Cmd) func() tea.Cmd {
		return func() tea.Cmd {
			if it := m.item(); it != nil {
				return f(it)
			}
			return nil
		}
	}
	return []commands.Command{
		{ID: "intercept.forward", Title: "Forward the item, as edited", Help: "forward", When: has,
			Run: with(func(it *proxy.Item) tea.Cmd { return m.decide(it, proxy.Decision{}) })},
		{ID: "intercept.drop", Title: "Drop the item", Help: "drop", When: has,
			Run: with(func(it *proxy.Item) tea.Cmd { return m.decide(it, proxy.Decision{Drop: true}) })},
		{ID: "intercept.forward-all", Title: "Forward every item of the queue", When: has, Run: func() tea.Cmd {
			return m.decideAll(decideAllMsg{items: m.focusedQueue().Items()})
		}},
		{ID: "intercept.drop-all", Title: "Drop every item of the queue", When: has, Run: func() tea.Cmd {
			msg := decideAllMsg{items: m.focusedQueue().Items(), drop: true}
			c := confirm.New("Drop", fmt.Sprintf("Drop the %s of the queue?", plural(len(msg.items), "item", "items")), func() tea.Msg { return msg })
			c.Danger = true
			return overlay.Open(c)
		}},
		{ID: "intercept.response", Title: "Also hold the response of this request", Help: "hold response", When: isRequest,
			Run: with(func(it *proxy.Item) tea.Cmd { m.inter.ToggleHold(it); return nil })},
		{ID: "intercept.steep", Title: "Send the request to steep", Help: "steep", When: has,
			Run: with(func(it *proxy.Item) tea.Cmd {
				raw := it.Request
				if it.Kind == proxy.KindRequest {
					raw = it.Raw
					if d := m.inter.Draft(it); d != nil {
						raw = d
					}
				}
				return m.toSteep(raw, it.URL)
			})},
		{ID: "intercept.copy", Title: "Copy the item (raw, as curl, or its URL)", Help: "copy", When: has,
			Run: with(m.copyItem)},
	}
}

func (m *Model) queueCommands(q *intercept.Queue) []commands.Command {
	return append(q.List().Commands(), commands.Command{ID: "intercept.open", Title: "Edit the item in the pane below", Help: "edit",
		When: func() bool { return q.Selected() != nil },
		Run:  func() tea.Cmd { m.setFocus(paneDetail); return nil }})
}

// steepCommand makes s available from the panes around the list: request,
// response, sitemap tree.
func (m *Model) steepCommand() commands.Command {
	selected := func() bool { _, ok := m.list().Selected(); return ok }
	return commands.Command{ID: "entries.steep", Title: "Send the request to steep", Help: "steep", When: selected,
		Run: func() tea.Cmd {
			e, ok := m.list().Selected()
			if !ok {
				return nil
			}
			return m.steepEntry(e)
		}}
}

func (m *Model) assetsCommand() commands.Command {
	title := "Show the assets (images, scripts, styles...)"
	if m.assets {
		title = "Hide the assets (images, scripts, styles...)"
	}
	return commands.Command{ID: "entries.assets", Title: title, Run: m.toggleAssets}
}

// scopes go from the global one to the focused pane.
func (m *Model) scopes() []commands.Scope {
	s := []commands.Scope{{Name: "spilltea", Commands: m.global()}}
	switch p := m.focus[m.page]; p {
	case paneRequests, paneResponses:
		q := m.inter.Queue(proxy.KindRequest)
		if p == paneResponses {
			q = m.inter.Queue(proxy.KindResponse)
		}
		s = append(s, commands.Scope{Name: "Queue", Commands: m.queueCommands(q)},
			commands.Scope{Name: "Intercept", Commands: m.interceptCommands()})
	case paneDetail:
		s = append(s, commands.Scope{Name: "Intercept", Commands: m.interceptCommands()})
	case paneHistory:
		s = append(s, commands.Scope{Name: "History", Commands: append(m.hist.Commands(), m.assetsCommand())})
	case paneSite:
		s = append(s, commands.Scope{Name: "Entries", Commands: append(m.site.Commands(), m.assetsCommand())})
	case paneTree:
		s = append(s, commands.Scope{Name: "Sitemap", Commands: append(m.tree.Commands(), m.assetsCommand(), m.steepCommand())})
	case paneHistRequest, paneHistResponse, paneSiteRequest, paneSiteResponse:
		s = append(s, commands.Scope{Name: "Message", Commands: append(m.reader(p).Commands(), m.steepCommand())})
	case paneDiffA, paneDiffB:
		s = append(s, commands.Scope{Name: "Diff", Commands: m.diffCommands()})
	case paneFindings:
		s = append(s, commands.Scope{Name: "Findings", Commands: append(m.finds.Commands(), m.rescanCommand(), m.copyFindingCommand())})
	case paneFinding:
		s = append(s, commands.Scope{Name: "Finding", Commands: append(m.findView.Commands(),
			append(m.finds.TriageCommands(), m.copyFindingCommand())...)})
	case paneSeen:
		s = append(s, commands.Scope{Name: "Seen in", Commands: m.seen.Commands()})
	}
	return s
}

func (m *Model) rescanCommand() commands.Command {
	return commands.Command{ID: "findings.rescan", Title: "Run the detectors on the whole history", When: func() bool { return !m.rescanning },
		Run: m.rescan}
}

func (m *Model) copyFindingCommand() commands.Command {
	return commands.Command{ID: "findings.copy", Title: "Copy the finding, in Markdown, with its first request", Help: "copy",
		When: func() bool { _, ok := m.finds.Selected(); return ok },
		Run: func() tea.Cmd {
			f, _ := m.finds.Selected()
			text := pfindings.Markdown(m.proj, f)
			return func() tea.Msg { return copiedMsg{text, "the finding"} }
		}}
}

func (m *Model) diffCommands() []commands.Command {
	return append(m.diff.Commands(),
		commands.Command{ID: "diff.swap", Title: "Swap the two sides of the diff", Help: "swap",
			When: func() bool { return m.diffA.ok && m.diffB.ok },
			Run: func() tea.Cmd {
				m.diffA, m.diffB = m.diffB, m.diffA
				return nil
			}},
		commands.Command{ID: "diff.mode", Title: "Switch the diff between requests and responses", Help: "requests/responses",
			Run: func() tea.Cmd { m.diffResp = !m.diffResp; return nil }},
	)
}

// allScopes is used to check every key binding at startup.
func (m *Model) allScopes() []commands.Scope {
	p := prompt.New("", "", nil)
	return []commands.Scope{
		{Name: "spilltea", Commands: m.global()},
		{Name: "Queue", Commands: m.queueCommands(&m.inter.Requests)},
		{Name: "Intercept", Commands: m.interceptCommands()},
		{Name: "History", Commands: append(m.hist.Commands(), m.assetsCommand())},
		{Name: "Sitemap", Commands: append(m.tree.Commands(), m.assetsCommand(), m.steepCommand())},
		{Name: "Message", Commands: append(m.histReq.Commands(), m.steepCommand())},
		{Name: "Diff", Commands: m.diffCommands()},
		{Name: "Findings", Commands: append(m.finds.Commands(), m.rescanCommand(), m.copyFindingCommand())},
		{Name: "Finding", Commands: append(m.findView.Commands(), append(m.finds.TriageCommands(), m.copyFindingCommand())...)},
		help.New(nil).Scope(),
		palette.New(nil, "").Scope(),
		confirm.New("", "", nil).Scope(),
		p.Scope(),
		picker.New("", "copy", nil, nil).Scope(),
		picker.New("", "filters", nil, nil).Scope(),
		picker.New("", "pipes", nil, nil).Scope(),
		picker.New("", "steep", nil, nil).Scope(),
	}
}

func (m *Model) navigate(d geom.Dir) {
	var cands []geom.Rect
	var panes []pane
	cur := m.focus[m.page]
	for _, p := range pagePanes[m.page] {
		if p != cur && !m.rects[p].Empty() {
			cands = append(cands, m.rects[p])
			panes = append(panes, p)
		}
	}
	if i := geom.Neighbor(m.rects[cur], cands, d); i >= 0 {
		m.setFocus(panes[i])
	}
}
