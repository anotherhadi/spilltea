package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anotherhadi/foam"
	fconfig "github.com/anotherhadi/foam/config"
)

func TestDefaults(t *testing.T) {
	c, err := fconfig.LoadBytes[Config](foam.Defaults, Defaults)
	if err != nil {
		t.Fatal(err)
	}
	if c.Proxy.Listen != "127.0.0.1:8080" || c.History.Responses != "full" || !c.HTTP.StripPwnFox || c.Steep.Folder != "spilltea" {
		t.Fatalf("%+v", c)
	}
}

func TestLayers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	proj := filepath.Join(home, "program", "reqs")
	os.MkdirAll(proj, 0o700)
	os.WriteFile(filepath.Join(proj, "config.yaml"), []byte("scope:\n  include: ['*.target.test']\n"), 0o600)
	t.Setenv("SPILLTEA_PROJECT", proj)
	t.Setenv("SPILLTEA_STEEP__COLLECTION", "~/program/collection")
	c, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.App.Project != proj || len(c.Scope.Include) != 1 || c.Scope.Include[0] != "*.target.test" {
		t.Fatalf("the project's config: %+v %+v", c.App, c.Scope)
	}
	if c.Steep.Collection != filepath.Join(home, "program", "collection") || c.Proxy.CA != filepath.Join(home, "data", "spilltea", "ca") {
		t.Fatalf("paths: %q %q", c.Steep.Collection, c.Proxy.CA)
	}
	if _, err := Load([]string{"-c", "rules=x"}); err == nil {
		t.Fatal("a broken rules list is accepted")
	}
	if _, err := Load([]string{"-c", "history.responses=some"}); err == nil {
		t.Fatal("history.responses: some is accepted")
	}
	if _, err := Load([]string{"-c", "keys.intercept.forwrd=f"}); err == nil || !strings.Contains(err.Error(), "keys.intercept.forwrd: no such command") {
		t.Fatalf("a key of no command: %v", err)
	}
	if _, err := Load([]string{"-c", "keys.intercept.forward=g"}); err != nil {
		t.Fatal(err)
	}
}

func TestSplitCommand(t *testing.T) {
	own, sub, rest := SplitCommand([]string{"--project", "ca", "ca", "-c", "a=b"})
	if sub != "ca" || len(own) != 4 || own[1] != "ca" || rest != nil {
		t.Fatalf("%q %q %q", own, sub, rest)
	}
	own, sub, rest = SplitCommand([]string{"import", "--all", "urls.txt", "--project", "p", "--source", "gau", "-", "--", "-odd"})
	if sub != "import" || strings.Join(own, " ") != "--project p" || strings.Join(rest, " ") != "--all urls.txt --source gau - -odd" {
		t.Fatalf("%q %q %q", own, sub, rest)
	}
}
