// Package steep sends requests to steep: a .http file in its collection, then
// an order to open it to every steep running on that collection (see steep's
// docs/07-Advanced.md, "Driven by other tools").
package steep

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/anotherhadi/spilltea/internal/message"
)

var nameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Name names the request the way steep would:
// "<folder>/api.test/login" for https://api.test/login?x=1.
func Name(folder string, u *url.URL) string {
	// trim dots too: a ".." host (from an imported URL) would escape the folder
	host := strings.Trim(nameUnsafe.ReplaceAllString(u.Hostname(), "-"), "-.")
	if host == "" {
		host = "request"
	}
	p := strings.Trim(u.Path, "/")
	segs := []string{}
	for _, s := range strings.Split(p, "/") {
		if s = strings.Trim(nameUnsafe.ReplaceAllString(s, "-"), "-."); s != "" {
			segs = append(segs, s)
		}
	}
	leaf := strings.Join(segs, "-")
	if leaf == "" {
		leaf = "index"
	}
	return path.Join(folder, host, leaf)
}

// File is the protocol directive, then the request with "\n" line endings.
func File(raw []byte, scheme string) string {
	return "# steep proto: " + scheme + "\n" + message.Text(raw)
}

// Send takes the first free name (login, login-2...) and returns how many
// steep instances were notified.
func Send(collection, folder string, raw []byte, u *url.URL) (string, int, error) {
	base := Name(folder, u)
	dir := filepath.Join(collection, "requests")
	text := File(raw, u.Scheme)
	for n := 1; n < 10000; n++ {
		name := base
		if n > 1 {
			name += "-" + strconv.Itoa(n)
		}
		p := filepath.Join(dir, filepath.FromSlash(name)+".http")
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return "", 0, err
		}
		// the request carries cookies and tokens, same as in the project
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", 0, err
		}
		_, err = f.WriteString(text)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return "", 0, err
		}
		return name, focus(collection, name), nil
	}
	return "", 0, fmt.Errorf("%s: no free name", base)
}

// socketDir and key must match steep's.
func socketDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "steep")
	}
	return filepath.Join(os.TempDir(), "steep-"+strconv.Itoa(os.Getuid()))
}

func key(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return hex.EncodeToString(sum[:8])
}

// privateDir reports whether dir is a real directory closed to others.
// Without XDG_RUNTIME_DIR it lives in /tmp, where another user could have
// created it to receive the request names.
func privateDir(dir string) bool {
	fi, err := os.Lstat(dir)
	return err == nil && fi.IsDir() && fi.Mode().Perm()&0o077 == 0
}

func focus(collection, name string) int {
	dir := socketDir()
	if !privateDir(dir) {
		return 0
	}
	b, _ := json.Marshal(map[string]string{"focus": name})
	paths, _ := filepath.Glob(filepath.Join(dir, key(collection)+".*.sock"))
	n := 0
	for _, p := range paths {
		c, err := net.DialTimeout("unix", p, time.Second)
		if err != nil {
			continue // steep is gone; it cleans up its own sockets
		}
		c.SetWriteDeadline(time.Now().Add(time.Second))
		if _, err := c.Write(append(b, '\n')); err == nil {
			n++
		}
		c.Close()
	}
	return n
}

// Targets returns where a request can go: conf when no steep is open or one
// is open on it, otherwise the open collections (one, or several to pick
// from).
func Targets(conf string) []string {
	open := Open()
	if len(open) == 0 || slices.ContainsFunc(open, func(c string) bool { return key(c) == key(conf) }) {
		return []string{conf}
	}
	return open
}

// Open lists the collections of the running steeps, read from the .root file
// each one writes next to its socket. Older steeps write none and are left
// out.
func Open() []string {
	dir := socketDir()
	if !privateDir(dir) {
		return nil
	}
	socks, _ := filepath.Glob(filepath.Join(dir, "*.sock"))
	var out []string
	for _, sock := range socks {
		b, err := os.ReadFile(strings.TrimSuffix(sock, ".sock") + ".root")
		root := string(b)
		if err != nil || root == "" || slices.Contains(out, root) {
			continue
		}
		c, err := net.DialTimeout("unix", sock, time.Second)
		if err != nil {
			continue // left by a steep that crashed
		}
		c.Close()
		out = append(out, root)
	}
	slices.Sort(out)
	return out
}
