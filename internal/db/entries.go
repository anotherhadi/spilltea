package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Entry struct {
	ID          int64
	Timestamp   time.Time
	Method      string
	Host        string
	Path        string
	StatusCode  int
	RequestRaw  string
	ResponseRaw string
}

func bodyHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", sum)
}

// HasDuplicate returns true if an entry with the same method, host, path and
// request body hash already exists.
func (d *DB) HasDuplicate(method, host, path, body string) (bool, error) {
	hash := bodyHash(body)
	var exists int
	err := d.conn.QueryRow(
		`SELECT 1 FROM entries WHERE method = ? AND host = ? AND path = ? AND body_hash = ? LIMIT 1`,
		method, host, path, hash,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// InsertIfNotDuplicate atomically checks for a duplicate and inserts if none
// exists. Returns (entry, isDuplicate, error).
func (d *DB) InsertIfNotDuplicate(e Entry, body string) (Entry, bool, error) {
	d.dedupMu.Lock()
	defer d.dedupMu.Unlock()
	dup, err := d.HasDuplicate(e.Method, e.Host, e.Path, body)
	if err != nil || dup {
		return e, dup, err
	}
	e, err = d.InsertEntry(e, body)
	return e, false, err
}

func (d *DB) InsertEntry(e Entry, body string) (Entry, error) {
	res, err := d.conn.Exec(
		`INSERT INTO entries (timestamp, method, host, path, status_code, request_raw, response_raw, body_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp.UTC().Format(time.RFC3339),
		e.Method, e.Host, e.Path, e.StatusCode, e.RequestRaw, e.ResponseRaw, bodyHash(body),
	)
	if err != nil {
		return e, err
	}
	e.ID, _ = res.LastInsertId()
	return e, nil
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry
	for rows.Next() {
		var e Entry
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Method, &e.Host, &e.Path, &e.StatusCode, &e.RequestRaw, &e.ResponseRaw); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) ListEntries() ([]Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, timestamp, method, host, path, status_code, request_raw, response_raw
		 FROM entries ORDER BY id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (d *DB) SearchEntries(term string) ([]Entry, error) {
	like := "%" + term + "%"
	rows, err := d.conn.Query(
		`SELECT id, timestamp, method, host, path, status_code, request_raw, response_raw
		 FROM entries
		 WHERE method LIKE ? OR host LIKE ? OR path LIKE ? OR request_raw LIKE ? OR response_raw LIKE ?
		 ORDER BY id DESC`,
		like, like, like, like, like,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// QueryEntries runs a WHERE expression supplied by the user against the entries
// table (e.g. "status_code = 404" or "host LIKE '%example.com%'").
func (d *DB) QueryEntries(where string) ([]Entry, error) {
	q := "SELECT id, timestamp, method, host, path, status_code, request_raw, response_raw FROM entries WHERE " + strings.TrimSpace(where)
	rows, err := d.conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

func (d *DB) DeleteEntry(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM entries WHERE id = ?`, id)
	return err
}

func (d *DB) DeleteAllEntries() error {
	_, err := d.conn.Exec(`DELETE FROM entries`)
	return err
}
