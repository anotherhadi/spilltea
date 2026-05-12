package db

func (d *DB) SaveScope(whitelist, blacklist []string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM scope`); err != nil {
		tx.Rollback()
		return err
	}
	for _, p := range whitelist {
		if _, err := tx.Exec(`INSERT INTO scope (kind, pattern) VALUES ('whitelist', ?)`, p); err != nil {
			tx.Rollback()
			return err
		}
	}
	for _, p := range blacklist {
		if _, err := tx.Exec(`INSERT INTO scope (kind, pattern) VALUES ('blacklist', ?)`, p); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) LoadScope() (whitelist, blacklist []string, err error) {
	rows, err := d.conn.Query(`SELECT kind, pattern FROM scope`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, pattern string
		if err := rows.Scan(&kind, &pattern); err != nil {
			return nil, nil, err
		}
		if kind == "whitelist" {
			whitelist = append(whitelist, pattern)
		} else {
			blacklist = append(blacklist, pattern)
		}
	}
	return whitelist, blacklist, rows.Err()
}
