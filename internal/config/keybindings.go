package config

type GlobalKeys struct {
	Quit          string `mapstructure:"quit"`
	OpenLogs      string `mapstructure:"open_logs"`
	ToggleSidebar string `mapstructure:"toggle_sidebar"`
	Help          string `mapstructure:"help"`
	Up            string `mapstructure:"up"`
	Down          string `mapstructure:"down"`
	Left          string `mapstructure:"left"`
	Right         string `mapstructure:"right"`
	CycleFocus    string `mapstructure:"cycle_focus"`
	CopyRequest   string `mapstructure:"copy_request"`
	SendToReplay  string `mapstructure:"send_to_replay"`
	ScrollUp      string `mapstructure:"scroll_up"`
	ScrollDown    string `mapstructure:"scroll_down"`
	SendToDiff    string `mapstructure:"send_to_diff"`
}

type InterceptKeys struct {
	Forward         string `mapstructure:"forward"`
	ForwardAll      string `mapstructure:"forward_all"`
	Drop            string `mapstructure:"drop"`
	DropAll         string `mapstructure:"drop_all"`
	AutoForward     string `mapstructure:"auto_forward"`
	CaptureResponse string `mapstructure:"capture_response"`
	UndoEdits       string `mapstructure:"undo_edits"`
	Edit            string `mapstructure:"edit"`
	EditExternal    string `mapstructure:"edit_external"`
}

type HistoryKeys struct {
	DeleteEntry string `mapstructure:"delete_entry"`
	DeleteAll   string `mapstructure:"delete_all"`
	Filter      string `mapstructure:"filter"`
	SqlQuery    string `mapstructure:"sql_query"`
}

type HomeKeys struct {
	Open   string `mapstructure:"open"`
	Delete string `mapstructure:"delete"`
	Filter string `mapstructure:"filter"`
}

type ReplayKeys struct {
	Send      string `mapstructure:"send"`
	Edit      string `mapstructure:"edit"`
	EditExt   string `mapstructure:"edit_external"`
	UndoEdits string `mapstructure:"undo_edits"`
	Delete    string `mapstructure:"delete_entry"`
	DeleteAll string `mapstructure:"delete_all"`
}

type DiffKeys struct {
	Clear string `mapstructure:"clear"`
}

type FindingsKeys struct {
	Dismiss string `mapstructure:"dismiss"`
}

type PluginsKeys struct {
	Toggle     string `mapstructure:"toggle"`
	EditConfig string `mapstructure:"edit_config"`
	Filter     string `mapstructure:"filter"`
}

type Keybindings struct {
	Global    GlobalKeys    `mapstructure:"global"`
	Intercept InterceptKeys `mapstructure:"intercept"`
	Home      HomeKeys      `mapstructure:"home"`
	History   HistoryKeys   `mapstructure:"history"`
	Replay    ReplayKeys    `mapstructure:"replay"`
	Diff      DiffKeys      `mapstructure:"diff"`
	Findings  FindingsKeys  `mapstructure:"findings"`
	Plugins   PluginsKeys   `mapstructure:"plugins"`
}
