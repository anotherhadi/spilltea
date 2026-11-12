// Package ui is the root TUI model: the five pages (intercept, history,
// sitemap, diff, findings), their panes, key bindings and overlays.
package ui

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/foam/commands"
	"github.com/anotherhadi/foam/confirm"
	"github.com/anotherhadi/foam/draw"
	"github.com/anotherhadi/foam/geom"
	"github.com/anotherhadi/foam/notify"
	"github.com/anotherhadi/foam/overlay"
	"github.com/anotherhadi/foam/prompt"
	"github.com/anotherhadi/foam/theme"
	ftree "github.com/anotherhadi/foam/tree"
	"github.com/anotherhadi/foam/viewer"

	"github.com/anotherhadi/spilltea/internal/config"
	pfindings "github.com/anotherhadi/spilltea/internal/findings"
	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
	"github.com/anotherhadi/spilltea/internal/proxy"
	"github.com/anotherhadi/spilltea/internal/steep"
	"github.com/anotherhadi/spilltea/internal/ui/diff"
	"github.com/anotherhadi/spilltea/internal/ui/entries"
	"github.com/anotherhadi/spilltea/internal/ui/findings"
	"github.com/anotherhadi/spilltea/internal/ui/intercept"
	"github.com/anotherhadi/spilltea/internal/ui/picker"
	"github.com/anotherhadi/spilltea/internal/ui/reader"
	"github.com/anotherhadi/spilltea/internal/ui/sitemap"
)

type page int

const (
	pageIntercept page = iota
	pageHistory
	pageSitemap
	pageDiff
	pageFindings
	pageCount
)

var pageNames = [pageCount]string{"Intercept", "History", "Sitemap", "Diff", "Findings"}

type pane int

const (
	paneRequests pane = iota
	paneResponses
	paneDetail
	paneHistory
	paneHistRequest
	paneHistResponse
	paneTree
	paneSite
	paneSiteRequest
	paneSiteResponse
	paneDiffA
	paneDiffB
	paneFindings
	paneFinding
	paneSeen
	paneCount
)

var pagePanes = [pageCount][]pane{
	pageIntercept: {paneRequests, paneResponses, paneDetail},
	pageHistory:   {paneHistory, paneHistRequest, paneHistResponse},
	pageSitemap:   {paneTree, paneSite, paneSiteRequest, paneSiteResponse},
	pageDiff:      {paneDiffA, paneDiffB},
	pageFindings:  {paneFindings, paneFinding, paneSeen},
}

type side struct {
	entry project.Entry
	ok    bool
}

type Model struct {
	cfg  *config.Config
	reg  *commands.Registry
	proj *project.Project
	px   *proxy.Proxy
	// tests read the proxy events themselves
	events <-chan any

	overlays overlay.Stack
	notes    notify.Center

	inter     *intercept.Model
	lastQueue pane // the queue whose selection the detail pane shows

	hist             *entries.Model
	histReq, histRes *reader.Model
	histShown        int // ID of the entry shown in the readers

	tree             *ftree.Model[sitemap.Node]
	site             *entries.Model
	siteReq, siteRes *reader.Model
	siteShown        int
	siteNode         string
	siteDirty        bool

	diff       *diff.Model
	diffA      side
	diffB      side
	diffResp   bool // compare responses instead of requests
	diffLoaded [2]int
	diffing    bool

	finds      *findings.Model
	findView   viewer.Model
	seen       *entries.Model
	findKey    string
	findSig    string
	seenIn     map[int]bool
	rescanning bool

	assets bool // shown in the history and the sitemap

	steepLast string // collection picked last when several steeps are open

	page   page
	focus  [pageCount]pane
	zoomed bool

	w, h       int
	rects      [paneCount]geom.Rect
	toastRects []toastRect
}

type toastRect struct {
	geom.Rect
	id int
}

// New accepts a nil px (tests); no events come in then.
func New(cfg *config.Config, proj *project.Project, px *proxy.Proxy) (*Model, error) {
	reg, err := commands.New(cfg.Keys)
	if err != nil {
		return nil, fmt.Errorf("invalid config:\n%w", err)
	}
	es, meta, err := proj.Load()
	if err != nil {
		return nil, fmt.Errorf("history: %w", err)
	}
	history := loadHistory()
	m := &Model{
		cfg: cfg, reg: reg, proj: proj, px: px,
		inter:     intercept.New(),
		lastQueue: paneRequests,
		hist:      entries.New(proj, meta, cfg.App.Filters, history),
		histReq:   reader.New(cfg.View.Wrap, cfg.View.PrettyPrint, cfg.App.Pipes),
		histRes:   reader.New(cfg.View.Wrap, cfg.View.PrettyPrint, cfg.App.Pipes),
		tree:      sitemap.New(),
		site:      entries.New(proj, meta, cfg.App.Filters, history),
		siteReq:   reader.New(cfg.View.Wrap, cfg.View.PrettyPrint, cfg.App.Pipes),
		siteRes:   reader.New(cfg.View.Wrap, cfg.View.PrettyPrint, cfg.App.Pipes),
		siteDirty: true,
		diff:      diff.New(),
		finds:     findings.New(proj),
		seen:      entries.New(proj, meta, cfg.App.Filters, history),
	}
	if err := m.finds.Reload(); err != nil {
		return nil, fmt.Errorf("findings: %w", err)
	}
	m.findView.Wrap = true
	if px != nil {
		m.events = px.Events()
	}
	m.hist.Empty = "No requests yet · proxy on " + shortAddr(cfg.Proxy.Listen)
	m.site.Empty = "Select a host or a path"
	m.seen.Empty = "Select a finding"
	m.notes.Max = 100
	m.focus = [pageCount]pane{paneRequests, paneHistory, paneTree, paneDiffA, paneFindings}
	for i, n := range []string{"intercept", "history", "sitemap", "diff", "findings"} {
		if cfg.App.Page == n {
			m.page = page(i)
		}
	}
	m.hist.SetEntries(es)
	m.hist.SetWithin(nil)
	m.site.SetEntries(append([]project.Entry(nil), es...))
	m.site.SetWithin(func(project.Entry) bool { return false })
	m.seen.SetEntries(append([]project.Entry(nil), es...))
	m.seen.SetAssets(true)
	m.seen.SetWithin(func(project.Entry) bool { return false })
	if err := reg.Check(m.allScopes()); err != nil {
		return nil, fmt.Errorf("invalid config:\n%w", err)
	}
	return m, nil
}

type eventMsg struct{ ev any }

func (m *Model) waitEvent() tea.Cmd {
	ch := m.events
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg{ev}
	}
}

func (m *Model) Init() tea.Cmd {
	if m.px != nil && !loopback(m.cfg.Proxy.Listen) {
		// no authentication: anyone on the network can use it
		return tea.Batch(m.waitEvent(), notify.Warning("The proxy listens on "+m.cfg.Proxy.Listen+": anyone on the network can use it"))
	}
	return m.waitEvent()
}

func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	return m, tea.Batch(cmd, m.afterUpdate())
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	if m.overlays.Handle(msg) {
		return nil
	}
	if cmd, ok := m.notes.Update(msg); ok {
		return cmd
	}
	if m.hist.Update(msg) || m.site.Update(msg) || m.seen.Update(msg) {
		return nil
	}
	for _, r := range m.readers() {
		if r.Update(msg) {
			return nil
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
	case tea.KeyPressMsg:
		return m.key(msg)
	case tea.PasteMsg:
		return m.paste(msg.Content)
	case tea.MouseMsg:
		return m.mouse(msg)
	case eventMsg:
		return tea.Batch(m.event(msg.ev), m.waitEvent())
	case entries.OpenMsg:
		return m.open()
	case entries.NoteMsg:
		return m.askNote(msg.Entry)
	case entries.MetaMsg:
		return tea.Batch(m.hist.Refresh(), m.site.Refresh(), m.seen.Refresh())
	case entries.DiffMsg:
		return m.setDiff(msg.Entry, msg.B)
	case entries.SteepMsg:
		return m.steepEntry(msg.Entry)
	case entries.CopyMsg:
		return m.copyEntry(msg.Entry)
	case entries.ForgetMsg:
		return m.askForget(msg)
	case entries.HistoryMsg:
		m.hist.SetHistory(msg.History)
		m.site.SetHistory(msg.History)
		return saveHistory(msg.History)
	case findings.OpenMsg:
		m.setFocus(paneSeen)
	case findings.NoteMsg:
		return m.askFindingNote(msg.Finding)
	case findings.TriagedMsg:
		return m.hist.RefreshCheap()
	case rescannedMsg:
		return m.rescanned(msg)
	case diffedMsg:
		if msg.want == m.diffLoaded {
			m.diff, m.diffing = msg.d, false
		}
	case ftree.OpenMsg:
		m.setFocus(paneSite)
	case copiedMsg:
		return m.copied(msg)
	case forgetMsg:
		return m.forget(msg.ids)
	case decideAllMsg:
		return m.decideAll(msg)
	}
	return nil
}

func (m *Model) readers() []*reader.Model {
	return []*reader.Model{m.histReq, m.histRes, m.siteReq, m.siteRes}
}

func (m *Model) event(ev any) tea.Cmd {
	switch ev := ev.(type) {
	case proxy.HeldMsg:
		m.inter.Add(ev.Item)
		if m.page != pageIntercept && m.inter.Len() == 1 {
			what := "Request"
			if ev.Item.Kind == proxy.KindResponse {
				what = "Response"
			}
			return notify.Info(what + " held · " + m.keyOf("page.intercept"))
		}
	case proxy.ReleasedMsg:
		m.inter.Remove(ev.Item)
	case proxy.EntryMsg:
		m.hist.Add(ev.Entry)
		m.site.Add(ev.Entry)
		m.seen.Add(ev.Entry)
		m.siteDirty = true
	case proxy.FindingMsg:
		before := m.finds.Untriaged()
		if err := m.finds.Reload(); err != nil {
			return notify.Err(err)
		}
		cmd := m.hist.RefreshCheap()
		if before == 0 && m.finds.Untriaged() > 0 && m.page != pageFindings {
			return tea.Batch(cmd, notify.Info("New finding · "+m.keyOf("page.findings")))
		}
		return cmd
	case proxy.ErrorMsg:
		return notify.Err(ev.Err)
	}
	return nil
}

func (m *Model) keyOf(id string) string {
	ks := m.reg.KeysOf(commands.Command{ID: id})
	if len(ks) == 0 {
		return "the palette (" + m.reg.KeysOf(commands.Command{ID: "app.palette"}).String() + ")"
	}
	return ks[0].Canonical()
}

// afterUpdate syncs whatever depends on the selections: the item in the
// editor, the entries in the readers, the sitemap, the layout.
func (m *Model) afterUpdate() tea.Cmd {
	var cmds []tea.Cmd
	if s, ok := m.inter.Yanked(); ok {
		cmds = append(cmds, tea.SetClipboard(s))
	}
	switch m.page {
	case pageIntercept:
		m.inter.Show(m.currentItem())
	case pageHistory:
		cmds = append(cmds, m.syncReaders(m.hist, &m.histShown, m.histReq, m.histRes))
	case pageSitemap:
		if m.siteDirty {
			m.tree.SetEntries(sitemap.Build(m.treeEntries()))
			m.siteDirty = false
		}
		node := ""
		if n, ok := m.tree.Selected(); ok {
			node = n.Prefix
		}
		if node != m.siteNode {
			m.siteNode = node
			cmds = append(cmds, m.site.SetWithin(func(e project.Entry) bool {
				return node != "" && sitemap.Under(sitemap.Path(e.URL), node)
			}))
		}
		cmds = append(cmds, m.syncReaders(m.site, &m.siteShown, m.siteReq, m.siteRes))
	case pageDiff:
		cmds = append(cmds, m.syncDiff())
	case pageFindings:
		cmds = append(cmds, m.syncFinding())
	}
	m.layout()
	return tea.Batch(cmds...)
}

// currentItem is the selection of the last focused queue, or of the other
// one if it is empty.
func (m *Model) currentItem() *proxy.Item {
	switch m.focus[pageIntercept] {
	case paneRequests, paneResponses:
		m.lastQueue = m.focus[pageIntercept]
	case paneDetail:
		if it := m.inter.Shown(); it != nil {
			return it
		}
	}
	first, second := &m.inter.Requests, &m.inter.Responses
	if m.lastQueue == paneResponses {
		first, second = second, first
	}
	if it := first.Selected(); it != nil {
		return it
	}
	return second.Selected()
}

func (m *Model) syncReaders(l *entries.Model, shown *int, req, res *reader.Model) tea.Cmd {
	e, ok := l.Selected()
	if !ok {
		if *shown != 0 {
			*shown = 0
			req.Clear()
			res.Clear()
		}
		return nil
	}
	if e.ID == *shown {
		return nil
	}
	*shown = e.ID
	var cmds []tea.Cmd
	raw, err := m.proj.ReadRequest(e.ID)
	if err != nil {
		req.Clear()
		cmds = append(cmds, notify.Err(err))
	} else {
		cmds = append(cmds, req.Show(raw, m.noteLines(e)))
	}
	raw, err = m.proj.ReadResponse(e.ID)
	switch {
	case err != nil:
		res.Clear()
		cmds = append(cmds, notify.Err(err))
	case raw == nil:
		res.Clear()
	default:
		cmds = append(cmds, res.Show(raw, m.responseNotes(e)))
	}
	return tea.Batch(cmds...)
}

// responseNotes explain why the body is missing.
func (m *Model) responseNotes(e project.Entry) []string {
	switch {
	case e.Kept != project.KeptHeaders:
		return nil
	case m.cfg.History.Responses == string(project.KeptHeaders):
		return []string{"# body not stored (history.responses: headers)"}
	case e.Len > 0:
		return []string{"# body not stored: " + entries.Size(e.Len) + ", over history.maxBodySize"}
	}
	return []string{"# body not stored: streamed"}
}

// noteLines are shown as comments above the request.
func (m *Model) noteLines(e project.Entry) []string {
	var out []string
	if note := m.hist.Meta(e.ID).Note; note != "" {
		for _, l := range strings.Split(note, "\n") {
			out = append(out, "# note: "+l)
		}
	}
	if e.Edited {
		out = append(out, "# edited")
	}
	if e.Source != "" {
		out = append(out, "# "+imported(e))
	}
	for _, f := range m.proj.FindingsOf(e.ID) {
		out = append(out, "# finding: "+f.Title+" ("+f.Detector+")")
	}
	return out
}

// imported returns "imported from katana", or "imported" for a plain URL list.
func imported(e project.Entry) string {
	if e.Source == "import" {
		return "imported"
	}
	return "imported from " + e.Source
}

func (m *Model) noResponse(l *entries.Model) string {
	e, ok := l.Selected()
	switch {
	case !ok:
		return ""
	case e.Error == "dropped":
		return "Dropped"
	case e.Error != "":
		return "No response · " + e.Error
	case e.Source != "" && e.Status == 0:
		return strings.ToUpper(imported(e)[:1]) + imported(e)[1:] + " · no response"
	case e.Kept == project.KeptStatus:
		return fmt.Sprintf("%d · body not stored", e.Status)
	case e.Kept == project.KeptNone:
		return "Responses not stored"
	}
	return "No response"
}

func (m *Model) open() tea.Cmd {
	switch m.page {
	case pageHistory:
		m.setFocus(paneHistRequest)
	case pageSitemap:
		m.setFocus(paneSiteRequest)
	case pageFindings:
		if e, ok := m.seen.Selected(); ok {
			return m.showInHistory(e)
		}
	}
	return nil
}

func (m *Model) showInHistory(e project.Entry) tea.Cmd {
	var cmds []tea.Cmd
	if !m.hist.SelectID(e.ID) && e.Asset && !m.assets {
		cmds = append(cmds, m.toggleAssets())
	}
	if !m.hist.SelectID(e.ID) && m.hist.Filter() != "" {
		cmds = append(cmds, m.hist.SetFilter(""), notify.Info(fmt.Sprintf("Filter cleared to show #%d", e.ID)))
	}
	m.hist.SelectID(e.ID)
	m.setFocus(paneHistory)
	return tea.Batch(cmds...)
}

func (m *Model) list() *entries.Model {
	switch m.page {
	case pageSitemap:
		return m.site
	case pageFindings:
		return m.seen
	}
	return m.hist
}

func (m *Model) syncFinding() tea.Cmd {
	f, ok := m.finds.Selected()
	sig := fmt.Sprint(f.Entries, f.Triage, f.Last)
	if f.Key == m.findKey && sig == m.findSig {
		return nil
	}
	same := f.Key == m.findKey
	m.findKey, m.findSig = f.Key, sig
	m.seenIn = map[int]bool{}
	for _, id := range f.Entries {
		m.seenIn[id] = true
	}
	if !ok {
		m.findView.SetContent(nil, nil)
	} else {
		plain := findings.Detail(f)
		for i, l := range plain {
			plain[i] = viewer.Sanitize(l)
		}
		styled := slices.Clone(plain)
		styled[0] = theme.Primary().Style().Bold(true).Render(plain[0])
		for i := 1; i < len(styled) && plain[i] != ""; i++ {
			styled[i] = theme.Subtle().Render(plain[i])
		}
		m.findView.SetContent(plain, styled)
	}
	in := m.seenIn
	within := func(e project.Entry) bool { return in[e.ID] }
	if same {
		return m.seen.Rewithin(within)
	}
	m.findView.Top()
	cmd := m.seen.SetWithin(within)
	if ok {
		m.seen.SelectID(f.Entries[0])
	}
	return cmd
}

type rescannedMsg struct {
	changed int
	err     error
}

func (m *Model) rescan() tea.Cmd {
	ds, err := pfindings.Detectors(m.cfg.Findings)
	if err != nil {
		return notify.Err(err)
	}
	if len(ds) == 0 {
		return notify.Warning("No detector is on, see findings in the config")
	}
	m.rescanning = true
	es, proj := slices.Clone(m.hist.All()), m.proj
	return tea.Batch(notify.Info("Scanning "+plural(len(es), "entry", "entries")), func() tea.Msg {
		n, err := pfindings.Rescan(proj, ds, es)
		return rescannedMsg{n, err}
	})
}

func (m *Model) rescanned(msg rescannedMsg) tea.Cmd {
	m.rescanning = false
	if err := m.finds.Reload(); err != nil {
		return notify.Err(err)
	}
	if msg.err != nil {
		return notify.Err(msg.err)
	}
	if msg.changed == 0 {
		return notify.Info("Scan done, nothing new")
	}
	return notify.Success("Scan done, " + plural(msg.changed, "request", "requests") + " with new findings")
}

func (m *Model) askFindingNote(f project.Finding) tea.Cmd {
	p := prompt.New("Note on "+draw.Clean(f.Title), f.Note, func(note string) tea.Cmd {
		return m.finds.SetTriage(f.Key, project.Triage{Status: f.Status, Note: strings.TrimSpace(note)})
	})
	p.Hint = "Leave empty to remove it"
	return overlay.Open(p)
}

func (m *Model) askNote(e project.Entry) tea.Cmd {
	p := prompt.New(fmt.Sprintf("Note on #%d", e.ID), m.hist.Meta(e.ID).Note, func(note string) tea.Cmd {
		meta := m.hist.Meta(e.ID)
		meta.Note = strings.TrimSpace(note)
		if err := m.hist.SetMeta(e.ID, meta); err != nil {
			return notify.Err(err)
		}
		m.histShown, m.siteShown = 0, 0 // redraw: the note is shown above the request
		return tea.Batch(m.hist.Refresh(), m.site.Refresh())
	})
	p.Hint = "Leave empty to remove it"
	return overlay.Open(p)
}

func (m *Model) setDiff(e project.Entry, b bool) tea.Cmd {
	if !b {
		m.diffA = side{e, true}
		return notify.Info(fmt.Sprintf("Diff: #%d on the left", e.ID))
	}
	m.diffB = side{e, true}
	if !m.diffA.ok {
		return notify.Info(fmt.Sprintf("Diff: #%d on the right", e.ID))
	}
	m.setPage(pageDiff)
	return nil
}

func (m *Model) diffText(e project.Entry) (string, error) {
	var raw []byte
	var err error
	if m.diffResp {
		raw, err = m.proj.ReadResponse(e.ID)
	} else {
		raw, err = m.proj.ReadRequest(e.ID)
	}
	return message.Text(raw), err
}

func (m *Model) syncDiff() tea.Cmd {
	if !m.diffA.ok || !m.diffB.ok {
		return nil
	}
	want := [2]int{m.diffA.entry.ID, m.diffB.entry.ID}
	if m.diffResp {
		want[0], want[1] = -want[0], -want[1]
	}
	if want == m.diffLoaded {
		return nil
	}
	m.diffLoaded = want
	a, err := m.diffText(m.diffA.entry)
	if err != nil {
		return notify.Err(err)
	}
	b, err := m.diffText(m.diffB.entry)
	if err != nil {
		return notify.Err(err)
	}
	m.diffing = true
	return func() tea.Msg {
		d := diff.New()
		d.Set(a, b)
		return diffedMsg{want, d}
	}
}

type diffedMsg struct {
	want [2]int
	d    *diff.Model
}

func (m *Model) steepEntry(e project.Entry) tea.Cmd {
	raw, err := m.proj.ReadRequest(e.ID)
	if err != nil {
		return notify.Err(err)
	}
	return m.toSteep(raw, e.URL)
}

func (m *Model) toSteep(raw []byte, rawURL string) tea.Cmd {
	u, err := url.Parse(rawURL)
	if err != nil {
		return notify.Err(err)
	}
	send := func(col string) tea.Cmd {
		name, n, err := steep.Send(col, m.cfg.Steep.Folder, raw, u)
		if err != nil {
			return notify.Err(fmt.Errorf("steep: %w", err))
		}
		if n == 0 {
			return notify.Success("Sent to steep: " + name + " (steep not running)")
		}
		return notify.Success("Sent to steep: " + name)
	}
	cols := steep.Targets(m.cfg.Steep.Collection)
	if len(cols) == 1 {
		return send(cols[0])
	}
	// the last pick first: enter sends there again
	if i := slices.Index(cols, m.steepLast); i > 0 {
		last := cols[i]
		cols = append([]string{last}, slices.Delete(cols, i, i+1)...)
	}
	items := make([]picker.Item, len(cols))
	byName := map[string]string{}
	for i, c := range cols {
		items[i] = picker.Item{Name: shortPath(c)}
		byName[items[i].Name] = c
	}
	return overlay.Open(picker.New("Send to steep", "steep", items, func(it picker.Item) tea.Cmd {
		m.steepLast = byName[it.Name]
		return send(m.steepLast)
	}))
}

type forgetMsg struct{ ids []int }

func (m *Model) askForget(msg entries.ForgetMsg) tea.Cmd {
	do := func() tea.Msg { return forgetMsg{msg.IDs} }
	if !msg.All {
		return do
	}
	q := fmt.Sprintf("Delete the %s shown?", plural(len(msg.IDs), "entry", "entries"))
	if !msg.Filtered {
		q = fmt.Sprintf("Delete the whole history (%s)?", plural(len(msg.IDs), "entry", "entries"))
	}
	c := confirm.New("Delete", q, do)
	c.Danger = true
	return overlay.Open(c)
}

func (m *Model) forget(ids []int) tea.Cmd {
	err := m.proj.Forget(ids)
	m.hist.Forget(ids)
	m.site.Forget(ids)
	m.seen.Forget(ids)
	if ferr := m.finds.Reload(); err == nil {
		err = ferr
	}
	m.siteDirty = true
	m.histShown, m.siteShown = 0, 0
	for _, s := range []*side{&m.diffA, &m.diffB} {
		if s.ok && slices.Contains(ids, s.entry.ID) {
			*s = side{}
			m.diffLoaded = [2]int{}
		}
	}
	if err != nil {
		return notify.Err(err)
	}
	return notify.Success("Deleted " + plural(len(ids), "entry", "entries"))
}

func (m *Model) Close() {
	if m.px != nil {
		m.px.Close()
	}
	m.proj.Close()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func (m *Model) quit() tea.Cmd {
	if n := m.inter.Len(); n > 0 {
		c := confirm.New("Quit", plural(n, "held item", "held items")+" will be dropped. Quit?", tea.Quit)
		c.Danger = true
		return overlay.Open(c)
	}
	return tea.Quit
}

func (m *Model) treeEntries() []project.Entry {
	if m.assets {
		return m.hist.All()
	}
	var out []project.Entry
	for _, e := range m.hist.All() {
		if !e.Asset {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) toggleAssets() tea.Cmd {
	m.assets = !m.assets
	m.siteDirty = true
	m.histShown, m.siteShown = 0, 0
	return tea.Batch(m.hist.SetAssets(m.assets), m.site.SetAssets(m.assets))
}

func (m *Model) setPage(p page) {
	m.page = p
	m.zoomed = false
	m.layout()
}

func (m *Model) setFocus(p pane) {
	for pg, panes := range pagePanes {
		for _, q := range panes {
			if q == p {
				m.page = page(pg)
				m.focus[pg] = p
			}
		}
	}
	m.layout()
}
