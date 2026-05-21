package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

//go:embed default_config.yaml
var defaultConfig []byte

type Config struct {
	Version string `mapstructure:"-"`

	App struct {
		Host           string `mapstructure:"host"`
		Port           int    `mapstructure:"port"`
		CertDir        string `mapstructure:"cert_dir"`
		ProjectDir     string `mapstructure:"project_dir"`
		PluginsDir     string `mapstructure:"plugins_dir"`
		UpstreamProxy  string `mapstructure:"upstream_proxy"`
		ProxyAuth      string `mapstructure:"proxy_auth"`
		MaxBodySizeMB  int    `mapstructure:"max_body_size_mb"`
		ExternalEditor string `mapstructure:"external_editor"`
	} `mapstructure:"app"`

	TUI struct {
		Colors              Colors `mapstructure:"colors"`
		UseNerdfontIcons    bool   `mapstructure:"use_nerdfont_icons"`
		DefaultSidebarState string `mapstructure:"default_sidebar_state"`
		PrettyPrintBody     bool   `mapstructure:"pretty_print_body"`
	} `mapstructure:"tui"`

	Intercept struct {
		DefaultInterceptEnabled bool     `mapstructure:"default_intercept_enabled"`
		DefaultCaptureResponse  bool     `mapstructure:"default_capture_response"`
		AutoForwardRegex        []string `mapstructure:"auto_forward_regex"`
		QueueSize               int      `mapstructure:"queue_size"`
	} `mapstructure:"intercept"`

	Replay struct {
		SwitchToPageOnSend bool `mapstructure:"switch_to_page_on_send"`
	} `mapstructure:"replay"`

	History struct {
		SkipDuplicates bool `mapstructure:"skip_duplicates"`
		KeepResponses  bool `mapstructure:"keep_responses"`
	} `mapstructure:"history"`

	Keybindings Keybindings `mapstructure:"keybindings"`

	Plugins map[string]GlobalPlugin `mapstructure:"plugins"`
}

type GlobalPlugin struct {
	Enable *bool       `mapstructure:"enable"`
	Config interface{} `mapstructure:"config"`
}

var Global *Config

func Load(path string) error {
	var defaults map[string]any
	if err := yaml.Unmarshal(defaultConfig, &defaults); err != nil {
		return fmt.Errorf("default config: %w", err)
	}
	for k, v := range flatten("", defaults) {
		viper.SetDefault(k, v)
	}

	viper.SetConfigType("yaml")
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	Global = &Config{}
	return viper.Unmarshal(Global)
}

func WriteDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, defaultConfig, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

func flatten(prefix string, m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]any); ok {
			for nk, nv := range flatten(key, nested) {
				out[nk] = nv
			}
		} else {
			out[key] = v
		}
	}
	return out
}
