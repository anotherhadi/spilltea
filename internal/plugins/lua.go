package plugins

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/db"
	goproxy "github.com/lqqyt2423/go-mitmproxy/proxy"
	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"
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

	L.SetGlobal("get_config", L.NewFunction(func(L *lua.LState) int {
		// p.mu is already held by the hook caller - do not lock again.
		configText := p.ConfigText
		if configText == "" {
			L.Push(L.NewTable())
			return 1
		}
		var data interface{}
		if err := yaml.Unmarshal([]byte(configText), &data); err != nil || data == nil {
			L.Push(L.NewTable())
			return 1
		}
		lv := goToLuaValue(L, data)
		if _, ok := lv.(*lua.LTable); !ok {
			L.Push(L.NewTable())
			return 1
		}
		L.Push(lv)
		return 1
	}))

	L.SetGlobal("send_request", L.NewFunction(func(L *lua.LState) int {
		method := L.CheckString(1)
		rawURL := L.CheckString(2)
		hdrs := L.OptTable(3, L.NewTable())
		body := L.OptString(4, "")
		opts := L.OptTable(5, L.NewTable())

		insecure := false
		if v, ok := L.GetField(opts, "insecure").(lua.LBool); ok {
			insecure = bool(v)
		}

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		}
		if up := config.Global.App.UpstreamProxy; up != "" {
			if proxyURL, err := url.Parse(up); err == nil {
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		}
		client := &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}

		var bodyReader io.Reader
		if body != "" {
			bodyReader = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, rawURL, bodyReader)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		hdrs.ForEach(func(k, v lua.LValue) {
			ks, kok := k.(lua.LString)
			vs, vok := v.(lua.LString)
			if kok && vok {
				req.Header.Set(string(ks), string(vs))
			}
		})

		resp, err := client.Do(req)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		result := L.NewTable()
		L.SetField(result, "status_code", lua.LNumber(resp.StatusCode))
		L.SetField(result, "body", lua.LString(string(respBody)))
		respHeaders := L.NewTable()
		for k, vals := range resp.Header {
			L.SetField(respHeaders, k, lua.LString(strings.Join(vals, ", ")))
		}
		L.SetField(result, "headers", respHeaders)

		L.Push(result)
		L.Push(lua.LNil)
		return 2
	}))

	L.SetGlobal("shell_pipe", L.NewFunction(func(L *lua.LState) int {
		cmd := L.CheckString(1)
		input := L.OptString(2, "")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		c := exec.CommandContext(ctx, "sh", "-c", cmd) // #nosec G204 -- intentional: shell_pipe is a Lua plugin API for user-written scripts
		c.Stdin = strings.NewReader(input)

		var stdout, stderr bytes.Buffer
		c.Stdout = &stdout
		c.Stderr = &stderr

		err := c.Run()
		if err != nil {
			L.Push(lua.LString(stdout.String()))
			L.Push(lua.LString(err.Error() + ": " + stderr.String()))
			return 2
		}
		L.Push(lua.LString(stdout.String()))
		L.Push(lua.LNil)
		return 2
	}))
}

func goToLuaValue(L *lua.LState, v interface{}) lua.LValue {
	switch val := v.(type) {
	case map[string]interface{}:
		t := L.NewTable()
		for k, v2 := range val {
			L.SetField(t, k, goToLuaValue(L, v2))
		}
		return t
	case []interface{}:
		t := L.NewTable()
		for i, v2 := range val {
			L.RawSetInt(t, i+1, goToLuaValue(L, v2))
		}
		return t
	case string:
		return lua.LString(val)
	case int:
		return lua.LNumber(val)
	case float64:
		return lua.LNumber(val)
	case bool:
		if val {
			return lua.LTrue
		}
		return lua.LFalse
	}
	return lua.LNil
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

	L.SetField(t, "set_path", L.NewFunction(func(L *lua.LState) int {
		r.URL.Path = L.CheckString(2)
		return 0
	}))

	L.SetField(t, "set_url", L.NewFunction(func(L *lua.LState) int {
		parsed, err := url.Parse(L.CheckString(2))
		if err != nil {
			log.Printf("[plugin] set_url: %v", err)
			return 0
		}
		r.URL = parsed
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

func callHook(p *Plugin, hookName string, args ...lua.LValue) (lua.LValue, error) {
	fn := p.L.GetGlobal(hookName)
	if fn == lua.LNil {
		return lua.LNil, nil
	}
	if err := p.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, args...); err != nil {
		return lua.LNil, err
	}
	ret := p.L.Get(-1)
	p.L.Pop(1)
	return ret, nil
}
