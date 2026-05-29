package util

import (
	"log"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/spilltea/internal/config"
)

type EditorFinishedMsg struct {
	Content string
	Err     error
}

func ResolveEditor() string {
	editor := config.Global.App.ExternalEditor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	return editor
}

func openWithEditor(content string, callback func(string, error) tea.Msg) tea.Cmd {
	f, err := os.CreateTemp("", "spilltea-*.http")
	if err != nil {
		log.Printf("editor: create temp file: %v", err)
		return nil
	}
	tmpPath := f.Name()
	if _, err := f.WriteString(content); err != nil {
		log.Printf("editor: writing temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		log.Printf("editor: closing temp file: %v", err)
	}
	return tea.ExecProcess(exec.Command(ResolveEditor(), tmpPath), func(err error) tea.Msg { // #nosec G204 -- editor from trusted config/$EDITOR, tmpPath from os.CreateTemp
		defer os.Remove(tmpPath)
		return callback(tmpPath, err)
	})
}

func OpenExternalEditor(content string) tea.Cmd {
	return openWithEditor(content, func(tmpPath string, err error) tea.Msg {
		if err != nil {
			return EditorFinishedMsg{Err: err}
		}
		data, readErr := os.ReadFile(tmpPath) // #nosec G304 -- tmpPath is from os.CreateTemp, controlled by this process
		if readErr != nil {
			return EditorFinishedMsg{Err: readErr}
		}
		return EditorFinishedMsg{Content: string(data)}
	})
}

func OpenExternalEditorView(content string) tea.Cmd {
	return openWithEditor(content, func(_ string, _ error) tea.Msg {
		return nil
	})
}
