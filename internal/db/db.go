package db

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn    *sql.DB
	dedupMu sync.Mutex
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS entries (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp    DATETIME NOT NULL,
			method       TEXT NOT NULL,
			host         TEXT NOT NULL,
			path         TEXT NOT NULL,
			status_code  INTEGER NOT NULL,
			request_raw  TEXT NOT NULL,
			response_raw TEXT NOT NULL,
			body_hash    TEXT NOT NULL DEFAULT ''
		);
CREATE TABLE IF NOT EXISTS replay_entries (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp    DATETIME NOT NULL,
			scheme       TEXT NOT NULL,
			host         TEXT NOT NULL,
			path         TEXT NOT NULL,
			method       TEXT NOT NULL,
			original_raw TEXT NOT NULL,
			request_raw  TEXT NOT NULL,
			response_raw TEXT NOT NULL,
			status_code  INTEGER NOT NULL,
			error_msg    TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS plugins (
			name        TEXT PRIMARY KEY,
			enabled     INTEGER NOT NULL DEFAULT 1,
			config_text TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS findings (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			plugin_name TEXT NOT NULL,
			dedup_key   TEXT NOT NULL,
			title       TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			severity    TEXT NOT NULL DEFAULT 'info',
			dismissed   INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL,
			UNIQUE(plugin_name, dedup_key)
		);
	`)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_entries_dedup ON entries(method, host, path, body_hash)`)
	return err
}

// Query executes a SQL query and returns the rows. The caller must close the
// returned rows. Args are passed as positional parameters.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.conn.Query(query, args...)
}

func (d *DB) Close() error {
	if d == nil {
		return nil
	}
	return d.conn.Close()
}

// CountEntriesAt opens the database at path read-only, counts entries, and
// closes it immediately. Safe to call on files not yet opened by the app.
func CountEntriesAt(path string) int {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return 0
	}
	defer conn.Close()
	var n int
	conn.QueryRow(`SELECT COUNT(*) FROM entries`).Scan(&n)
	return n
}
