package db

import (
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
}

func (d *DB) InsertReplayEntry(e ReplayEntry) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO replay_entries (timestamp, scheme, host, path, method, original_raw, request_raw, response_raw, status_code, error_msg)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp.UTC().Format(time.RFC3339),
		e.Scheme, e.Host, e.Path, e.Method,
		e.OriginalRaw, e.RequestRaw, e.ResponseRaw,
		e.StatusCode, e.ErrorMsg,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateReplayEntry(e ReplayEntry) error {
	_, err := d.conn.Exec(
		`UPDATE replay_entries SET request_raw=?, response_raw=?, status_code=?, error_msg=? WHERE id=?`,
		e.RequestRaw, e.ResponseRaw, e.StatusCode, e.ErrorMsg, e.ID,
	)
	return err
}

func (d *DB) ListReplayEntries() ([]ReplayEntry, error) {
	rows, err := d.conn.Query(
		`SELECT id, timestamp, scheme, host, path, method, original_raw, request_raw, response_raw, status_code, error_msg
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
		if err := rows.Scan(&e.ID, &ts, &e.Scheme, &e.Host, &e.Path, &e.Method,
			&e.OriginalRaw, &e.RequestRaw, &e.ResponseRaw, &e.StatusCode, &e.ErrorMsg); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) DeleteReplayEntry(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM replay_entries WHERE id = ?`, id)
	return err
}

func (d *DB) DeleteAllReplayEntries() error {
	_, err := d.conn.Exec(`DELETE FROM replay_entries`)
	return err
}
