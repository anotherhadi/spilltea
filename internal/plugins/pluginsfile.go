package plugins

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type pluginFileEntry struct {
	Enable bool   `yaml:"enable"`
	Config string `yaml:"config,omitempty"`
}

type pluginsFileData struct {
	Plugins map[string]pluginFileEntry `yaml:"plugins"`
}

type PluginsFile struct {
	path string
	data pluginsFileData
}

func OpenPluginsFile(dbPath string) (*PluginsFile, error) {
	path := filepath.Join(filepath.Dir(dbPath), "plugins.yaml")
	pf := &PluginsFile{
		path: path,
		data: pluginsFileData{Plugins: make(map[string]pluginFileEntry)},
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return pf, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, &pf.data); err != nil {
		return nil, err
	}
	if pf.data.Plugins == nil {
		pf.data.Plugins = make(map[string]pluginFileEntry)
	}
	return pf, nil
}

func (pf *PluginsFile) save() error {
	raw, err := yaml.Marshal(&pf.data)
	if err != nil {
		return err
	}
	return os.WriteFile(pf.path, raw, 0o600)
}

func (pf *PluginsFile) get(name string) (enabled bool, config string, found bool) {
	e, ok := pf.data.Plugins[name]
	return e.Enable, e.Config, ok
}

func (pf *PluginsFile) register(name string, defaultEnabled bool) {
	if _, ok := pf.data.Plugins[name]; !ok {
		pf.data.Plugins[name] = pluginFileEntry{Enable: defaultEnabled}
		_ = pf.save()
	}
}

func (pf *PluginsFile) setEnabled(name string, enabled bool) error {
	e := pf.data.Plugins[name]
	e.Enable = enabled
	pf.data.Plugins[name] = e
	return pf.save()
}

func (pf *PluginsFile) setConfig(name string, configText string) error {
	e := pf.data.Plugins[name]
	e.Config = configText
	pf.data.Plugins[name] = e
	return pf.save()
}
