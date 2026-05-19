package util

import (
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/anotherhadi/spilltea/internal/config"
)

type EditorFinishedMsg struct {
	Content string
	Err     error
}

func OpenExternalEditor(content string) tea.Cmd {
	editor := config.Global.App.ExternalEditor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	f, err := os.CreateTemp("", "spilltea-*.http")
	if err != nil {
		return nil
	}
	tmpPath := f.Name()
	_, _ = f.WriteString(content)
	f.Close()
	return tea.ExecProcess(exec.Command(editor, tmpPath), func(err error) tea.Msg {
		defer os.Remove(tmpPath)
		if err != nil {
			return EditorFinishedMsg{Err: err}
		}
		data, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return EditorFinishedMsg{Err: readErr}
		}
		return EditorFinishedMsg{Content: string(data)}
	})
}
