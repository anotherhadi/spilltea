package db

import (
	"log"
	"time"
)

type ReplayEntry struct {
	ID          int64
	Timestamp   time.Time
	Scheme      string
	Host        string
	Path        string
	Method      string
	OriginalRaw string
	RequestRaw  string
	ResponseRaw string
	StatusCode  int
	ErrorMsg    string
	Flagged     bool
}

func (d *DB) InsertReplayEntry(e ReplayEntry) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO replay_entries (timestamp, scheme, host, path, method, original_raw, request_raw, response_raw, status_code, error_msg, flagged)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp.UTC().Format(time.RFC3339),
		e.Scheme, e.Host, e.Path, e.Method,
		e.OriginalRaw, e.RequestRaw, e.ResponseRaw,
		e.StatusCode, e.ErrorMsg, e.Flagged,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateReplayEntry(e ReplayEntry) error {
	_, err := d.conn.Exec(
		`UPDATE replay_entries SET request_raw=?, response_raw=?, status_code=?, error_msg=?, flagged=? WHERE id=?`,
		e.RequestRaw, e.ResponseRaw, e.StatusCode, e.ErrorMsg, e.Flagged, e.ID,
	)
	return err
}

func (d *DB) ListReplayEntries() ([]ReplayEntry, error) {
	rows, err := d.conn.Query(
		`SELECT id, timestamp, scheme, host, path, method, original_raw, request_raw, response_raw, status_code, error_msg, flagged
		 FROM replay_entries ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ReplayEntry
	for rows.Next() {
		var e ReplayEntry
		var ts string
		var flagged int
		if err := rows.Scan(&e.ID, &ts, &e.Scheme, &e.Host, &e.Path, &e.Method,
			&e.OriginalRaw, &e.RequestRaw, &e.ResponseRaw, &e.StatusCode, &e.ErrorMsg, &flagged); err != nil {
			return nil, err
		}
		var tsErr error
		e.Timestamp, tsErr = time.Parse(time.RFC3339, ts)
		if tsErr != nil {
			log.Printf("db: parse replay timestamp %q: %v", ts, tsErr)
		}
		e.Flagged = flagged != 0
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) ToggleReplayFlag(id int64) error {
	_, err := d.conn.Exec(`UPDATE replay_entries SET flagged = NOT flagged WHERE id = ?`, id)
	return err
}

func (d *DB) DeleteReplayEntry(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM replay_entries WHERE id = ?`, id)
	return err
}

func (d *DB) DeleteAllReplayEntries() error {
	_, err := d.conn.Exec(`DELETE FROM replay_entries`)
	return err
}

// DeleteAllReplayEntriesExceptFlagged deletes all unflagged replay entries. If
// none are unflagged (only flagged ones remain), it deletes everything.
func (d *DB) DeleteAllReplayEntriesExceptFlagged() error {
	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM replay_entries WHERE flagged = 0`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		_, err := d.conn.Exec(`DELETE FROM replay_entries WHERE flagged = 0`)
		return err
	}
	_, err := d.conn.Exec(`DELETE FROM replay_entries`)
	return err
}
