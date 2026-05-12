package plugins

import (
	"log"
	"strings"
	"time"

	"github.com/anotherhadi/spilltea/internal/db"
	goproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
	lua "github.com/yuin/gopher-lua"
)

func newLuaState(mgr *Manager, p *Plugin) *lua.LState {
	L := lua.NewState()
	registerUtilities(L, mgr, p)
	return L
}

func registerUtilities(L *lua.LState, mgr *Manager, p *Plugin) {
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		log.Printf("[plugin:%s] %s", p.Name, msg)
		return 0
	}))

	L.SetGlobal("notif", L.NewFunction(func(L *lua.LState) int {
		title := L.CheckString(1)
		body := L.CheckString(2)
		select {
		case mgr.Notifs <- PluginNotifMsg{Title: title, Body: body}:
		default:
		}
		return 0
	}))

	L.SetGlobal("create_finding", L.NewFunction(func(L *lua.LState) int {
		t := L.CheckTable(1)
		title := luaTableString(t, "title")
		desc := luaTableString(t, "description")
		key := luaTableString(t, "key")
		severity := luaTableString(t, "severity")
		if severity == "" {
			severity = "info"
		}
		if key == "" {
			key = title
		}
		if mgr.db == nil {
			return 0
		}
		inserted, err := mgr.db.UpsertFinding(db.Finding{
			PluginName:  p.Name,
			DedupKey:    key,
			Title:       title,
			Description: desc,
			Severity:    severity,
			CreatedAt:   time.Now(),
		})
		if err != nil {
			log.Printf("[plugin:%s] create_finding error: %v", p.Name, err)
			return 0
		}
		_ = inserted
		return 0
	}))

L.SetGlobal("quit", L.NewFunction(func(L *lua.LState) int {
		reason := L.OptString(1, "plugin requested quit")
		select {
		case mgr.Quit <- reason:
		default:
		}
		return 0
	}))
}

func luaTableString(t *lua.LTable, key string) string {
	v := t.RawGetString(key)
	if s, ok := v.(lua.LString); ok {
		return string(s)
	}
	return ""
}

func pushRequest(L *lua.LState, f *goproxy.Flow) *lua.LTable {
	t := L.NewTable()
	r := f.Request
	L.SetField(t, "method", lua.LString(r.Method))
	L.SetField(t, "url", lua.LString(r.URL.String()))
	L.SetField(t, "host", lua.LString(r.URL.Host))
	L.SetField(t, "path", lua.LString(r.URL.Path))

	headers := L.NewTable()
	for k, vals := range r.Header {
		L.SetField(headers, k, lua.LString(strings.Join(vals, ", ")))
	}
	L.SetField(t, "headers", headers)

	L.SetField(t, "get_body", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(string(r.Body)))
		return 1
	}))

	L.SetField(t, "set_header", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(2)
		value := L.CheckString(3)
		r.Header.Set(name, value)
		return 0
	}))

	L.SetField(t, "set_body", L.NewFunction(func(L *lua.LState) int {
		body := L.CheckString(2)
		r.Body = []byte(body)
		return 0
	}))

	return t
}

func pushResponse(L *lua.LState, f *goproxy.Flow) *lua.LTable {
	t := L.NewTable()
	if f.Response == nil {
		return t
	}
	resp := f.Response
	L.SetField(t, "status_code", lua.LNumber(resp.StatusCode))

	headers := L.NewTable()
	for k, vals := range resp.Header {
		L.SetField(headers, k, lua.LString(strings.Join(vals, ", ")))
	}
	L.SetField(t, "headers", headers)

	L.SetField(t, "get_body", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(string(resp.Body)))
		return 1
	}))

	L.SetField(t, "set_header", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(2)
		value := L.CheckString(3)
		resp.Header.Set(name, value)
		return 0
	}))

	L.SetField(t, "set_body", L.NewFunction(func(L *lua.LState) int {
		body := L.CheckString(2)
		resp.Body = []byte(body)
		return 0
	}))

	return t
}

func pushEntry(L *lua.LState, e db.Entry) *lua.LTable {
	t := L.NewTable()
	L.SetField(t, "id", lua.LNumber(e.ID))
	L.SetField(t, "method", lua.LString(e.Method))
	L.SetField(t, "host", lua.LString(e.Host))
	L.SetField(t, "path", lua.LString(e.Path))
	L.SetField(t, "status_code", lua.LNumber(e.StatusCode))
	L.SetField(t, "timestamp", lua.LString(e.Timestamp.Format("2006-01-02 15:04:05")))
	L.SetField(t, "request_raw", lua.LString(e.RequestRaw))
	L.SetField(t, "response_raw", lua.LString(e.ResponseRaw))
	return t
}

func callHook(p *Plugin, hookName string, args ...lua.LValue) (string, error) {
	fn := p.L.GetGlobal(hookName)
	if fn == lua.LNil {
		return "", nil
	}
	if err := p.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, args...); err != nil {
		return "", err
	}
	ret := p.L.Get(-1)
	p.L.Pop(1)
	if s, ok := ret.(lua.LString); ok {
		return string(s), nil
	}
	return "", nil
}
