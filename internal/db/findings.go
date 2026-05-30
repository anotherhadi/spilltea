package db

import "time"

var findingTimeFormats = []string{time.RFC3339, "2006-01-02 15:04:05"}

type Finding struct {
	ID          int64
	PluginName  string
	DedupKey    string
	Title       string
	Description string
	Severity    string
	Flagged     bool
	CreatedAt   time.Time
}

// UpsertFinding inserts the finding if the (plugin_name, dedup_key) pair does
// not already exist. Returns true when the row was actually inserted.
func (d *DB) UpsertFinding(f Finding) (bool, error) {
	d.dedupMu.Lock()
	defer d.dedupMu.Unlock()
	res, err := d.conn.Exec(
		`INSERT OR IGNORE INTO findings (plugin_name, dedup_key, title, description, severity, dismissed, created_at)
		 VALUES (?, ?, ?, ?, ?, 0, ?)`,
		f.PluginName, f.DedupKey, f.Title, f.Description, f.Severity,
		f.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (d *DB) LoadFindings() ([]Finding, error) {
	rows, err := d.conn.Query(
		`SELECT id, plugin_name, dedup_key, title, description, severity, flagged, created_at
		 FROM findings WHERE dismissed = 0 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var f Finding
		var ts string
		var flagged int
		if err := rows.Scan(&f.ID, &f.PluginName, &f.DedupKey, &f.Title, &f.Description, &f.Severity, &flagged, &ts); err != nil {
			return nil, err
		}
		f.Flagged = flagged != 0
		for _, layout := range findingTimeFormats {
			if t, err := time.Parse(layout, ts); err == nil {
				f.CreatedAt = t.Local()
				break
			}
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (d *DB) DismissFinding(id int64) error {
	_, err := d.conn.Exec(`UPDATE findings SET dismissed = 1 WHERE id = ?`, id)
	return err
}

func (d *DB) ToggleFindingFlag(id int64) error {
	_, err := d.conn.Exec(`UPDATE findings SET flagged = NOT flagged WHERE id = ?`, id)
	return err
}

// DismissAllFindings dismisses unflagged findings first. If none are unflagged
// (only flagged ones remain), it dismisses everything.
func (d *DB) DismissAllFindings() error {
	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM findings WHERE dismissed = 0 AND flagged = 0`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		_, err := d.conn.Exec(`UPDATE findings SET dismissed = 1 WHERE flagged = 0`)
		return err
	}
	_, err := d.conn.Exec(`UPDATE findings SET dismissed = 1`)
	return err
}
