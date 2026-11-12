package ui

import (
	"fmt"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/foam/confirm"
	"github.com/anotherhadi/foam/notify"
	"github.com/anotherhadi/foam/overlay"

	"github.com/anotherhadi/spilltea/internal/curl"
	"github.com/anotherhadi/spilltea/internal/message"
	"github.com/anotherhadi/spilltea/internal/project"
	"github.com/anotherhadi/spilltea/internal/proxy"
	"github.com/anotherhadi/spilltea/internal/ui/picker"
)

// Above this size, ask first: terminals and tmux often truncate large
// clipboard writes (OSC 52).
const copyAskAbove = 1 << 20

type copiedMsg struct {
	text string
	what string
}

// choice.text is only computed once picked.
type choice struct {
	name string
	text func() (string, error)
}

func (m *Model) pickCopy(title string, choices []choice) tea.Cmd {
	items := make([]picker.Item, len(choices))
	for i, c := range choices {
		items[i] = picker.Item{Name: c.name}
	}
	return overlay.Open(picker.New(title, "copy", items, func(it picker.Item) tea.Cmd {
		for _, c := range choices {
			if c.name == it.Name {
				text, err := c.text()
				if err != nil {
					return notify.Err(err)
				}
				return func() tea.Msg { return copiedMsg{text, strings.TrimPrefix(title, "Copy ") + " " + c.name} }
			}
		}
		return nil
	}))
}

func (m *Model) copied(c copiedMsg) tea.Cmd {
	done := tea.Batch(tea.SetClipboard(c.text), notify.Success("Copied "+c.what))
	if len(c.text) > copyAskAbove {
		q := fmt.Sprintf("Copy %s (%.1f MB)? Terminals and tmux may truncate copies this large.", c.what, float64(len(c.text))/(1<<20))
		return overlay.Open(confirm.New("Copy", q, done))
	}
	return done
}

func curlOf(raw []byte, rawURL string) (string, error) {
	scheme := "https"
	if u, err := url.Parse(rawURL); err == nil && u.Scheme != "" {
		scheme = u.Scheme
	}
	req, err := message.ParseRequest(raw, scheme)
	if err != nil {
		return "", err
	}
	return curl.Format(req), nil
}

func bodyOf(raw []byte) string {
	_, body := message.Split(raw)
	return string(body)
}

func (m *Model) copyEntry(e project.Entry) tea.Cmd {
	req := func() ([]byte, error) { return m.proj.ReadRequest(e.ID) }
	resp := func() ([]byte, error) {
		b, err := m.proj.ReadResponse(e.ID)
		if err == nil && b == nil {
			err = fmt.Errorf("#%d: no response kept", e.ID)
		}
		return b, err
	}
	str := func(f func() ([]byte, error), conv func([]byte) string) func() (string, error) {
		return func() (string, error) {
			b, err := f()
			if err != nil {
				return "", err
			}
			return conv(b), nil
		}
	}
	return m.pickCopy(fmt.Sprintf("Copy #%d", e.ID), []choice{
		{"request", str(req, func(b []byte) string { return string(b) })},
		{"request as curl", func() (string, error) {
			b, err := req()
			if err != nil {
				return "", err
			}
			return curlOf(b, e.URL)
		}},
		{"response", str(resp, func(b []byte) string { return string(b) })},
		{"response body", str(resp, bodyOf)},
		{"URL", func() (string, error) { return e.URL, nil }},
	})
}

func (m *Model) copyItem(it *proxy.Item) tea.Cmd {
	raw := func() []byte {
		if d := m.inter.Draft(it); d != nil {
			return d
		}
		return it.Raw
	}
	request := func() []byte {
		if it.Kind == proxy.KindRequest {
			return raw()
		}
		return it.Request
	}
	choices := []choice{
		{"as held", func() (string, error) { return string(raw()), nil }},
		{"request as curl", func() (string, error) { return curlOf(request(), it.URL) }},
		{"URL", func() (string, error) { return it.URL, nil }},
	}
	if it.Kind == proxy.KindResponse {
		choices = append(choices, choice{"body", func() (string, error) { return bodyOf(raw()), nil }})
	}
	return m.pickCopy(fmt.Sprintf("Copy #%d", it.Seq), choices)
}
