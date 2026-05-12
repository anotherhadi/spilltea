package db

type PluginState struct {
	Name       string
	Enabled    bool
	ConfigText string
}

func (d *DB) SavePluginState(name string, enabled bool, configText string) error {
	_, err := d.conn.Exec(
		`INSERT INTO plugins (name, enabled, config_text) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET enabled = excluded.enabled, config_text = excluded.config_text`,
		name, enabled, configText,
	)
	return err
}

func (d *DB) LoadPluginStates() ([]PluginState, error) {
	rows, err := d.conn.Query(`SELECT name, enabled, config_text FROM plugins`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PluginState
	for rows.Next() {
		var s PluginState
		var enabled int
		if err := rows.Scan(&s.Name, &enabled, &s.ConfigText); err != nil {
			return nil, err
		}
		s.Enabled = enabled != 0
		out = append(out, s)
	}
	return out, rows.Err()
}
