package intercept

import tea "charm.land/bubbletea/v2"

type RequestArrivedMsg struct{ Req *PendingRequest }
type ResponseArrivedMsg struct{ Resp *PendingResponse }

func WaitForRequest(b *Broker) tea.Cmd {
	return func() tea.Msg {
		return RequestArrivedMsg{Req: <-b.Incoming}
	}
}

func WaitForResponse(b *Broker) tea.Cmd {
	return func() tea.Msg {
		return ResponseArrivedMsg{Resp: <-b.IncomingResponse}
	}
}
