package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCA(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ca")
	ca, err := LoadCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	root := ca.GetRootCA()
	if root.Subject.CommonName != CAName || !root.IsCA {
		t.Fatalf("CN %q, IsCA %v", root.Subject.CommonName, root.IsCA)
	}
	for name, want := range map[string]os.FileMode{".": 0o700, caKeyFile: 0o600, caCertFile: 0o644} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil || fi.Mode().Perm() != want {
			t.Errorf("%s: %v, %v, want %o", name, fi.Mode().Perm(), err, want)
		}
	}
	// loaded again, not recreated
	again, err := LoadCA(dir)
	if err != nil || again.GetRootCA().SerialNumber.Cmp(root.SerialNumber) != 0 {
		t.Fatalf("reload: %v", err)
	}
}

// An existing world-readable key (older spilltea, go-mitmproxy) is kept and
// made private.
func TestLoadCAFixesMode(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadCA(dir); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, caKeyFile)
	os.Chmod(key, 0o644)
	if _, err := LoadCA(dir); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(key); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
}
