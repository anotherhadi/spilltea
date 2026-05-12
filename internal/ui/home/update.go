package home

import (
	crypto "crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/anotherhadi/spilltea/internal/keys"
	"github.com/anotherhadi/spilltea/internal/ui/components/teapot"
)

type teapotTickMsg struct{}

func teapotTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return teapotTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(ws.Width, ws.Height)
		return m, nil
	}

	if _, ok := msg.(teapotTickMsg); ok {
		frames := teapot.TeapotFrames()
		m.teapotFrame = (m.teapotFrame + 1) % len(frames)
		return m, teapotTick()
	}

	if m.mode == modeNaming {
		if kp, ok := msg.(tea.KeyPressMsg); ok {
			return m.updateNaming(kp)
		}
		return m, nil
	}

	if kp, ok := msg.(tea.KeyPressMsg); ok {
		if !m.list.SettingFilter() {
			if key.Matches(kp, keys.Keys.Global.Quit) {
				return m, tea.Quit
			}
			if key.Matches(kp, keys.Keys.Home.Open) {
				return m.handleSelection()
			}
			if key.Matches(kp, keys.Keys.Home.Delete) {
				return m.deleteSelected()
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleSelection() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(listItem)
	if !ok {
		return m, nil
	}
	switch item.kind {
	case kindNew:
		m.mode = modeNaming
		m.nameInput.SetValue("")
		return m, m.nameInput.Focus()
	case kindTemp:
		dir := tempDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return m, nil
		}
		initProjectFiles(dir)
		m.selected = &Project{Name: "temporary", Path: filepath.Join(dir, "data.db")}
		return m, tea.Quit
	default:
		m.selected = &Project{Name: item.name, Path: item.path}
		return m, tea.Quit
	}
}

func (m Model) deleteSelected() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(listItem)
	if !ok || item.kind != kindExisting {
		return m, nil
	}
	dir := filepath.Dir(item.path) // parent dir of data.db
	os.RemoveAll(dir)
	idx := m.list.GlobalIndex()
	m.list.RemoveItem(idx)
	if idx > 0 && idx >= len(m.list.Items()) {
		m.list.Select(idx - 1)
	}
	return m, nil
}

func (m Model) updateNaming(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Keys.Global.Escape):
		m.mode = modeSelect
		m.nameInput.Blur()
		return m, nil
	case msg.String() == "enter":
		name := m.nameInput.Value()
		if name == "" {
			return m, nil
		}
		m.mode = modeSelect
		m.nameInput.Blur()
		dir := filepath.Join(m.projectDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return m, nil
		}
		initProjectFiles(dir)
		m.selected = &Project{Name: name, Path: filepath.Join(dir, "data.db")}
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		m.nameInput.SetValue(sanitizeName(m.nameInput.Value()))
		return m, cmd
	}
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func IsValidProjectName(s string) bool {
	if s == "tmp" {
		return true
	}
	return s != "" && s == sanitizeName(s)
}

func OpenProject(projectDir, name string) (*Project, error) {
	if name == "tmp" {
		dir := tempDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		initProjectFiles(dir)
		return &Project{Name: "temporary", Path: filepath.Join(dir, "data.db")}, nil
	}
	dir := filepath.Join(projectDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	initProjectFiles(dir)
	return &Project{Name: name, Path: filepath.Join(dir, "data.db")}, nil
}

func tempDir() string {
	b := make([]byte, 4)
	_, _ = crypto.Read(b)
	return filepath.Join(os.TempDir(), "spilltea", fmt.Sprintf("%08x", b))
}

func initProjectFiles(dir string) {
	for _, name := range []string{"data.db", "logs.log"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			f, err := os.Create(p)
			if err == nil {
				f.Close()
			}
		}
	}
}
