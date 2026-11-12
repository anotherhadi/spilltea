package steep

import (
	"bufio"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	for raw, want := range map[string]string{
		"https://api.test/login?x=1":   "spilltea/api.test/login",
		"https://api.test/":            "spilltea/api.test/index",
		"http://h:8080/a/b c/../d.php": "spilltea/h/a-b-c-d.php",
		"https://[::1]:8443/x":         "spilltea/1/x",
		"http://../a":                  "spilltea/request/a",
	} {
		u, _ := url.Parse(raw)
		if got := Name("spilltea", u); got != want {
			t.Errorf("%s: %q, want %q", raw, got, want)
		}
	}
}

func TestSend(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	col := t.TempDir()
	os.MkdirAll(socketDir(), 0o700)
	ln, err := net.Listen("unix", filepath.Join(socketDir(), key(col)+".42.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	orders := make(chan string, 2)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(c).ReadString('\n')
			orders <- line
			c.Close()
		}
	}()
	u, _ := url.Parse("http://api.test/login")
	raw := []byte("POST /login HTTP/1.1\r\nHost: api.test\r\n\r\na=1")
	name, n, err := Send(col, "spilltea", raw, u)
	if err != nil || name != "spilltea/api.test/login" || n != 1 {
		t.Fatalf("Send = %q, %d, %v", name, n, err)
	}
	if o := <-orders; o != `{"focus":"spilltea/api.test/login"}`+"\n" {
		t.Fatalf("order %q", o)
	}
	b, _ := os.ReadFile(filepath.Join(col, "requests", "spilltea", "api.test", "login.http"))
	if string(b) != "# steep proto: http\nPOST /login HTTP/1.1\nHost: api.test\n\na=1" {
		t.Fatalf("file %q", b)
	}
	if name, _, _ := Send(col, "spilltea", raw, u); name != "spilltea/api.test/login-2" {
		t.Fatalf("second: %q", name)
	}
}

// A socket folder open to others (created by another user in /tmp) gets
// nothing.
func TestSendSharedSocketDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	os.MkdirAll(socketDir(), 0o777)
	os.Chmod(socketDir(), 0o777)
	col := t.TempDir()
	ln, err := net.Listen("unix", filepath.Join(socketDir(), key(col)+".1.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	u, _ := url.Parse("http://api.test/x")
	if _, n, err := Send(col, "spilltea", []byte("GET /x HTTP/1.1\r\nHost: api.test\r\n\r\n"), u); err != nil || n != 0 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	fi, err := os.Stat(filepath.Join(col, "requests", "spilltea", "api.test", "x.http"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file: %v, %v", fi, err)
	}
}

// fakeSteep listens like a steep open on root does.
func fakeSteep(t *testing.T, root string, pid int) {
	t.Helper()
	os.MkdirAll(socketDir(), 0o700)
	sock := filepath.Join(socketDir(), key(root)+"."+strconv.Itoa(pid)+".sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	os.WriteFile(strings.TrimSuffix(sock, ".sock")+".root", []byte(root), 0o600)
}

func TestTargets(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	conf, a, b := t.TempDir(), t.TempDir(), t.TempDir()
	if got := Targets(conf); !slices.Equal(got, []string{conf}) {
		t.Fatalf("none open: %v", got)
	}
	fakeSteep(t, a, 1)
	fakeSteep(t, a, 2) // two steeps on the same collection
	if got := Targets(conf); !slices.Equal(got, []string{a}) {
		t.Fatalf("one open: %v", got)
	}
	fakeSteep(t, b, 3)
	if got := Targets(conf); len(got) != 2 {
		t.Fatalf("two open: %v", got)
	}
	fakeSteep(t, conf, 4)
	if got := Targets(conf); !slices.Equal(got, []string{conf}) {
		t.Fatalf("conf open: %v", got)
	}
}

// A .root file whose steep is gone does not count.
func TestOpenStale(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	os.MkdirAll(socketDir(), 0o700)
	stale := filepath.Join(socketDir(), "abcd.9")
	os.WriteFile(stale+".sock", nil, 0o600)
	os.WriteFile(stale+".root", []byte(t.TempDir()), 0o600)
	if got := Open(); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}
