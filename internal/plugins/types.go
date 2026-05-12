package plugins

import (
	"sync"

	lua "github.com/yuin/gopher-lua"
)

type HookConfig struct {
	Sync bool
}

type Plugin struct {
	Name       string
	FilePath   string
	Enabled    bool
	ConfigText string

	L     *lua.LState
	mu    sync.Mutex
	hooks map[string]HookConfig
}

func (p *Plugin) HookNames() []string {
	out := make([]string, 0, len(p.hooks))
	for name := range p.hooks {
		out = append(out, name)
	}
	return out
}

func (p *Plugin) HookConfig(name string) (HookConfig, bool) {
	hc, ok := p.hooks[name]
	return hc, ok
}

type Info struct {
	Name       string
	FilePath   string
	Enabled    bool
	ConfigText string
	Hooks      map[string]HookConfig
}

func (p *Plugin) Info() Info {
	hooks := make(map[string]HookConfig, len(p.hooks))
	for k, v := range p.hooks {
		hooks[k] = v
	}
	return Info{
		Name:       p.Name,
		FilePath:   p.FilePath,
		Enabled:    p.Enabled,
		ConfigText: p.ConfigText,
		Hooks:      hooks,
	}
}

type PluginNotifMsg struct {
	Title string
	Body  string
}

type PluginQuitMsg struct {
	Reason string
}
