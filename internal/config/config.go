// Package config loads spilltea's configuration. It never writes it.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/anotherhadi/foam"
	"github.com/anotherhadi/foam/commands"
	fconfig "github.com/anotherhadi/foam/config"
	"github.com/anotherhadi/foam/theme"
)

//go:embed default.yaml
var Defaults []byte

type Config struct {
	Theme     theme.Theme       `koanf:",squash"`
	Keys      commands.Bindings `koanf:"keys"`
	App       App               `koanf:",squash"`
	Proxy     Proxy             `koanf:"proxy"`
	Scope     Scope             `koanf:"scope"`
	Intercept Intercept         `koanf:"intercept"`
	Assets    Assets            `koanf:"assets"`
	History   History           `koanf:"history"`
	Findings  Findings          `koanf:"findings"`
	Rules     []Rule            `koanf:"rules" validate:"dive"`
	HTTP      HTTP              `koanf:"http"`
	Steep     Steep             `koanf:"steep"`
	View      View              `koanf:"view"`
}

type App struct {
	Project       string   `koanf:"project"`
	CreateProject bool     `koanf:"createProject"`
	Page          string   `koanf:"page" validate:"oneof=intercept history sitemap diff findings"`
	Filters       []Filter `koanf:"filters"`
	Pipes         []Pipe   `koanf:"pipes"`
}

// Filter is a named HTTPQL query, picked with ctrl+r or used as preset:"name".
type Filter struct {
	Name  string `koanf:"name"`
	Query string `koanf:"query"`
}

type Pipe struct {
	Name    string `koanf:"name"`
	Command string `koanf:"command"`
}

type Proxy struct {
	Listen   string `koanf:"listen" validate:"required"`
	Upstream string `koanf:"upstream"`
	Insecure bool   `koanf:"insecure"`
	CA       string `koanf:"ca"`
}

type Scope struct {
	Include []string `koanf:"include"`
	Exclude []string `koanf:"exclude"`
}

type Intercept struct {
	Requests  bool   `koanf:"requests"`
	Responses bool   `koanf:"responses"`
	Filter    string `koanf:"filter"`
}

type Assets struct {
	Extensions []string `koanf:"extensions"`
	Types      []string `koanf:"types"`
}

// Ext takes the extension without its dot.
func (a Assets) Ext(ext string) bool {
	return ext != "" && slices.ContainsFunc(a.Extensions, func(e string) bool {
		return strings.EqualFold(strings.TrimPrefix(e, "."), ext)
	})
}

// Type takes a media type already normalized by message.Mime.
func (a Assets) Type(mime string) bool {
	if mime == "" {
		return false
	}
	for _, t := range a.Types {
		if t = strings.ToLower(t); strings.HasSuffix(t, "/") && strings.HasPrefix(mime, t) || strings.Contains(mime, t) {
			return true
		}
	}
	return false
}

type History struct {
	Responses   string `koanf:"responses" validate:"oneof=full headers status none"`
	MaxBodySize int64  `koanf:"maxBodySize" validate:"gte=0"`
	JSEndpoints bool   `koanf:"jsEndpoints"`
}

type Findings struct {
	Secrets    bool `koanf:"secrets"`
	Cookies    bool `koanf:"cookies"`
	CORS       bool `koanf:"cors"`
	Disclosure bool `koanf:"disclosure"`
	Reflection bool `koanf:"reflection"`
}

type Rule struct {
	On      string `koanf:"on" validate:"oneof=request response"`
	Part    string `koanf:"part" validate:"oneof=head body"`
	Match   string `koanf:"match"`
	Replace string `koanf:"replace"`
}

type HTTP struct {
	Headers         []string `koanf:"headers"`
	UserAgentSuffix string   `koanf:"userAgentSuffix"`
	StripPwnFox     bool     `koanf:"stripPwnFox"`
}

type Steep struct {
	Collection string `koanf:"collection"`
	Folder     string `koanf:"folder"`
}

type View struct {
	PrettyPrint bool `koanf:"prettyPrint"`
	Wrap        bool `koanf:"wrap"`
}

func (c *Config) Validate() error {
	for i, p := range c.App.Pipes {
		if strings.TrimSpace(p.Command) == "" {
			return fmt.Errorf("pipes[%d]: no command", i)
		}
	}
	for i, f := range c.App.Filters {
		if strings.TrimSpace(f.Name) == "" || strings.TrimSpace(f.Query) == "" {
			return fmt.Errorf("filters[%d]: needs a name and a query", i)
		}
	}
	for i, h := range c.HTTP.Headers {
		if name, _, ok := strings.Cut(h, ":"); !ok || strings.TrimSpace(name) == "" {
			return fmt.Errorf("http.headers[%d]: %q is not \"Name: value\"", i, h)
		}
	}
	for i, r := range c.Rules {
		if _, err := regexp.Compile(r.Match); err != nil {
			return fmt.Errorf("rules[%d].match: %w", i, err)
		}
		if r.Match == "" && r.Part == "body" {
			return fmt.Errorf("rules[%d]: an empty match only adds a header (part: head)", i)
		}
	}
	if strings.Contains(c.Steep.Folder, "..") || strings.HasPrefix(c.Steep.Folder, "/") {
		return errors.New("steep.folder: a folder inside the collection")
	}
	return nil
}

// Version is set at build time with -ldflags -X.
var Version string

func getVersion() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}

const usage = `spilltea: an intercepting HTTP proxy for the terminal

Usage: spilltea [options]
       spilltea [options] ca     print the CA certificate (PEM) to trust in your browser
       spilltea [options] import [--all] [--source name] [file...]
                                 add URLs from files (or stdin) to the history: one per line,
                                 or JSON lines from httpx, katana, urlfinder`

// Options that take a value, so the value is not mistaken for a subcommand
// (--project import).
var valued = map[string]bool{"--project": true, "--listen": true, "--upstream": true, "-c": true, "--set": true}

var importValued = map[string]bool{"--source": true}

// SplitCommand separates spilltea's own arguments, the subcommand ("" if
// none) and the subcommand's arguments, in any order.
func SplitCommand(args []string) (own []string, sub string, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case sub == "import" && a == "--":
			return own, sub, append(rest, args[i+1:]...)
		case a == "--":
			return append(own, args[i:]...), sub, rest
		case valued[a]:
			own = append(own, a)
			if i+1 < len(args) {
				own = append(own, args[i+1])
				i++
			}
		case sub == "" && (a == "ca" || a == "import"):
			sub = a
		case sub == "import" && importValued[a]:
			rest = append(rest, a)
			if i+1 < len(args) {
				rest = append(rest, args[i+1])
				i++
			}
		case sub == "import" && (a == "--all" || strings.HasPrefix(a, "--source=") || a == "-" || !strings.HasPrefix(a, "-")):
			rest = append(rest, a)
		default:
			own = append(own, a)
		}
	}
	return own, sub, rest
}

// Load returns fconfig.ErrExit after --help or --version.
func Load(args []string) (*Config, error) {
	var version bool
	cfg, _, err := fconfig.Load[Config](fconfig.Options{
		Name:  "spilltea",
		Usage: usage,
		Base:  [][]byte{foam.Defaults, Defaults},
		Files: []string{
			filepath.Join(fconfig.Dir("foam"), "config.yaml"),
			filepath.Join(fconfig.Dir("spilltea"), "config.yaml"),
		},
		Extra: func(get func(string) string) []string {
			return []string{filepath.Join(ProjectDir(get("project")), "config.yaml")}
		},
		Flags: func(fs *flag.FlagSet) {
			fs.String("project", "", "directory of the project to open")
			fs.String("listen", "", "address the proxy listens on")
			fs.String("upstream", "", "upstream proxy")
			fs.BoolVarP(&version, "version", "v", false, "show the version")
		},
		FlagKeys: map[string]string{
			"project":  "project",
			"listen":   "proxy.listen",
			"upstream": "proxy.upstream",
		},
	}, args)
	if version {
		fmt.Println("spilltea", getVersion())
		return nil, fconfig.ErrExit
	}
	if err != nil {
		return nil, err
	}
	if err := checkKeys(cfg.Keys); err != nil {
		return nil, fmt.Errorf("invalid config:\n%w", err)
	}
	cfg.App.Project = ProjectDir(cfg.App.Project)
	cfg.Proxy.CA = CADir(cfg.Proxy.CA)
	cfg.Steep.Collection = SteepCollection(cfg.Steep.Collection)
	for i, p := range cfg.App.Pipes {
		// a YAML block (command: |) ends with a newline
		cfg.App.Pipes[i].Command = strings.TrimSpace(p.Command)
		if p.Name == "" {
			cfg.App.Pipes[i].Name = strings.Join(strings.Fields(p.Command), " ")
		}
	}
	return cfg, nil
}

// checkKeys rejects bindings for unknown commands, otherwise a typo in keys
// would be silently ignored.
func checkKeys(b commands.Bindings) error {
	base, err := fconfig.LoadBytes[Config](foam.Defaults, Defaults)
	if err != nil {
		return err
	}
	known, err := base.Keys.Flatten()
	if err != nil {
		return err
	}
	ids, err := b.Flatten()
	if err != nil {
		return err
	}
	var errs []error
	for id := range ids {
		if _, ok := known[id]; !ok {
			errs = append(errs, fmt.Errorf("keys.%s: no such command", id))
		}
	}
	slices.SortFunc(errs, func(a, b error) int { return strings.Compare(a.Error(), b.Error()) })
	return errors.Join(errs...)
}

func dataDir(name string) string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return name
	}
	return filepath.Join(home, ".local", "share", name)
}

// ProjectDir defaults to $XDG_DATA_HOME/spilltea/project.
func ProjectDir(dir string) string {
	if dir != "" {
		return fconfig.ExpandHome(dir)
	}
	return filepath.Join(dataDir("spilltea"), "project")
}

// CADir defaults to $XDG_DATA_HOME/spilltea/ca.
func CADir(dir string) string {
	if dir != "" {
		return fconfig.ExpandHome(dir)
	}
	return filepath.Join(dataDir("spilltea"), "ca")
}

// SteepCollection resolves the path the same way steep does, defaulting to
// $XDG_DATA_HOME/steep.
func SteepCollection(dir string) string {
	if dir != "" {
		return fconfig.ExpandHome(dir)
	}
	return dataDir("steep")
}
