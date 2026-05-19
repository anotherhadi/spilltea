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
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		{lua.CoroutineLibName, lua.OpenCoroutine},
	} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}
	// Remove filesystem-access functions to prevent plugins from reading/executing arbitrary files.
	for _, name := range []string{"dofile", "loadfile", "load"} {
		L.SetGlobal(name, lua.LNil)
	}
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
		kind := L.OptString(3, "info")
		select {
		case mgr.Notifs <- PluginNotifMsg{Title: title, Body: body, Kind: kind}:
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

	L.SetGlobal("db_query", L.NewFunction(func(L *lua.LState) int {
		if mgr.db == nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("db not available"))
			return 2
		}
		query := L.CheckString(1)
		var args []any
		for i := 2; i <= L.GetTop(); i++ {
			switch v := L.Get(i).(type) {
			case lua.LString:
				args = append(args, string(v))
			case lua.LNumber:
				args = append(args, float64(v))
			case lua.LBool:
				args = append(args, bool(v))
			default:
				args = append(args, nil)
			}
		}
		rows, err := mgr.db.Query(query, args...)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		result := L.NewTable()
		rowIdx := 1
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			row := L.NewTable()
			for i, col := range cols {
				switch v := vals[i].(type) {
				case int64:
					L.SetField(row, col, lua.LNumber(v))
				case float64:
					L.SetField(row, col, lua.LNumber(v))
				case string:
					L.SetField(row, col, lua.LString(v))
				case []byte:
					L.SetField(row, col, lua.LString(string(v)))
				case bool:
					if v {
						L.SetField(row, col, lua.LTrue)
					} else {
						L.SetField(row, col, lua.LFalse)
					}
				case nil:
					L.SetField(row, col, lua.LNil)
				}
			}
			L.RawSetInt(result, rowIdx, row)
			rowIdx++
		}
		if err := rows.Err(); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(result)
		L.Push(lua.LNil)
		return 2
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
