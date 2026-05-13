package spilltea

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed plugins/*.lua
var PluginsFS embed.FS

// InstallDefaultPlugins copies embedded default plugins into dir, skipping
// files that already exist. Returns the number of files written.
func InstallDefaultPlugins(dir string) (int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("create plugins dir: %w", err)
	}

	entries, err := fs.ReadDir(PluginsFS, "plugins")
	if err != nil {
		return 0, err
	}

	written := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		data, err := PluginsFS.ReadFile("plugins/" + e.Name())
		if err != nil {
			return written, fmt.Errorf("read embedded %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return written, fmt.Errorf("write %s: %w", dst, err)
		}
		written++
	}
	return written, nil
}
