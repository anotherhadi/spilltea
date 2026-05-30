package icons

import "github.com/anotherhadi/spilltea/internal/config"

type Icons struct {
	Forward   string
	Drop      string
	Edit      string
	Intercept string
	History   string
	Replay    string
	Diff      string
	Request   string
	Response  string
	Plugin    string
	Findings  string
	Scope     string
	Detail    string
	Docs      string
	New       string
	Temp      string
	Project   string
	Flag      string
}

var I *Icons

func Init(cfg *config.Config) {
	if cfg.TUI.UseNerdfontIcons {
		I = &Icons{
			Forward:   "󰁔 ",
			Drop:      "󰆴 ",
			Edit:      "󰏫 ",
			Intercept: " ",
			History:   "󰋚 ",
			Replay:    "󰑙 ",
			Diff:      "󰕛 ",
			Request:   "󰜷 ",
			Response:  "󰜮 ",
			Plugin:    " ",
			Findings:  "󱎸 ",
			Scope:     "󰓾 ",
			Detail:    "󰱼 ",
			Docs:      " ",
			New:       "󰐕 ",
			Temp:      "󰙨 ",
			Project:   "󰉋 ",
			Flag:      "󰈻 ",
		}
	} else {
		I = &Icons{
			Flag: "*",
		}
	}
}
