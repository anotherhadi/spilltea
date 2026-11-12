// Command spilltea is an intercepting HTTP proxy for the terminal.
package main

import (
	"crypto/sha256"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	flag "github.com/spf13/pflag"

	fconfig "github.com/anotherhadi/foam/config"

	"github.com/anotherhadi/spilltea/internal/config"
	"github.com/anotherhadi/spilltea/internal/importer"
	"github.com/anotherhadi/spilltea/internal/project"
	"github.com/anotherhadi/spilltea/internal/proxy"
	"github.com/anotherhadi/spilltea/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spilltea:", err)
		os.Exit(1)
	}
}

func run() error {
	args, sub, rest := config.SplitCommand(os.Args[1:])
	cfg, err := config.Load(args)
	if errors.Is(err, fconfig.ErrExit) {
		return nil
	}
	if err != nil {
		return err
	}
	switch sub {
	case "ca":
		return printCA(cfg)
	case "import":
		return runImport(cfg, rest)
	}
	return ui.Run(cfg)
}

func runImport(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("spilltea import", flag.ContinueOnError)
	var opt importer.Options
	fs.BoolVar(&opt.All, "all", false, "also import out-of-scope URLs")
	fs.StringVar(&opt.Source, "source", "", "tool name stored in row.source (default: guessed from the input)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()
	if len(files) == 0 {
		if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
			return errors.New("import: no input, pass files or pipe URLs on stdin")
		}
		files = []string{"-"}
	}
	proxy.Quiet() // go-mitmproxy logs bodies it fails to decode
	open := project.Open
	if !cfg.App.CreateProject {
		open = project.OpenExisting
	}
	proj, err := open(cfg.App.Project)
	if err != nil {
		return err
	}
	defer proj.Close()
	im, err := importer.New(cfg, proj, opt)
	if err != nil {
		return err
	}
	for _, name := range files {
		if err := importFile(im, name); err != nil {
			return fmt.Errorf("import %s: %w", name, err)
		}
	}
	st := im.Stats
	fmt.Fprintf(os.Stderr, "%d imported in %s", st.Added, cfg.App.Project)
	for _, n := range []struct {
		n    int
		what string
	}{{st.Duplicates, "already there"}, {st.OutOfScope, "out of scope (--all)"}, {st.Invalid, "unreadable"}} {
		if n.n > 0 {
			fmt.Fprintf(os.Stderr, ", %d %s", n.n, n.what)
		}
	}
	fmt.Fprintln(os.Stderr)
	return nil
}

func importFile(im *importer.Importer, name string) error {
	if name == "-" {
		return im.Read(os.Stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return im.Read(f)
}

// printCA writes the certificate to stdout, so it can be redirected to a
// file, and its location to stderr.
func printCA(cfg *config.Config) error {
	proxy.Quiet()
	ca, err := proxy.LoadCA(cfg.Proxy.CA)
	if err != nil {
		return fmt.Errorf("CA in %s: %w", cfg.Proxy.CA, err)
	}
	c := ca.GetRootCA()
	sum := sha256.Sum256(c.Raw)
	hexs := make([]string, len(sum))
	for i, b := range sum {
		hexs[i] = fmt.Sprintf("%02X", b)
	}
	fmt.Fprintf(os.Stderr, "%s\nin %s\nSHA-256 %s\n\n", c.Subject.CommonName, cfg.Proxy.CA, strings.Join(hexs, ":"))
	return pem.Encode(os.Stdout, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
}
