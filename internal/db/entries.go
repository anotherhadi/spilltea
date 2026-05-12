package db

import (
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

// HasDuplicate returns true if an entry with the same method, host, path and
// request body already exists. Used to implement skip_duplicates filtering.
func (d *DB) HasDuplicate(method, host, path, body string) (bool, error) {
	rows, err := d.conn.Query(
		`SELECT request_raw FROM entries WHERE method = ? AND host = ? AND path = ?`,
		method, host, path,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return false, err
		}
		parts := strings.SplitN(raw, "\n\n", 2)
		entryBody := ""
		if len(parts) == 2 {
			entryBody = parts[1]
		}
		if entryBody == body {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (d *DB) InsertEntry(e Entry) (Entry, error) {
	res, err := d.conn.Exec(
		`INSERT INTO entries (timestamp, method, host, path, status_code, request_raw, response_raw)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp.UTC().Format(time.RFC3339),
		e.Method, e.Host, e.Path, e.StatusCode, e.RequestRaw, e.ResponseRaw,
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

// QueryEntries executes a user-supplied query against the entries table.
// If the query does not start with SELECT, it is treated as a WHERE expression
// and wrapped automatically (e.g. "status_code = 404" becomes a full SELECT).
func (d *DB) QueryEntries(rawSQL string) ([]Entry, error) {
	q := strings.TrimSpace(rawSQL)
	if !strings.HasPrefix(strings.ToUpper(q), "SELECT") {
		q = "SELECT id, timestamp, method, host, path, status_code, request_raw, response_raw FROM entries WHERE " + q
	} else if strings.ContainsAny(strings.ToUpper(q), "INSERTDELETEUPDATEDROP") {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}
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
