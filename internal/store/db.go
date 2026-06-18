package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// CloseDB is an alias for db.Close() matching the spec API.
func CloseDB(db *sql.DB) error {
	return db.Close()
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS entries (
    name         TEXT PRIMARY KEY,
    need_manual_dl INTEGER DEFAULT 0,
    download_url TEXT DEFAULT '',
    note         TEXT DEFAULT '',
    package_path TEXT DEFAULT '',
    is_archive   INTEGER DEFAULT 0,
    config_path  TEXT DEFAULT '',
    platform     TEXT DEFAULT '',
    script_path  TEXT DEFAULT ''
)`

func InitDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return db, nil
}

func SaveEntries(db *sql.DB, entries []AppEntry) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO entries
		(name, need_manual_dl, download_url, note, package_path, is_archive, config_path, platform, script_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("invalid entry %q: %w", e.Name, err)
		}
		needManual := 0
		if e.NeedManualDL {
			needManual = 1
		}
		isArchive := 0
		if e.IsArchive {
			isArchive = 1
		}
		if _, err := stmt.Exec(e.Name, needManual, e.DownloadURL, e.Note,
			e.PackagePath, isArchive, e.ConfigPath, e.Platform, e.ScriptPath); err != nil {
			return fmt.Errorf("exec %q: %w", e.Name, err)
		}
	}

	return tx.Commit()
}

func LoadEntries(db *sql.DB, platform string) ([]AppEntry, error) {
	var rows *sql.Rows
	var err error

	if platform == "" {
		rows, err = db.Query(`SELECT name, need_manual_dl, download_url, note,
			package_path, is_archive, config_path, platform, script_path FROM entries`)
	} else {
		rows, err = db.Query(`SELECT name, need_manual_dl, download_url, note,
			package_path, is_archive, config_path, platform, script_path FROM entries
			WHERE platform IN ('', ?)`, platform)
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var entries []AppEntry
	for rows.Next() {
		var e AppEntry
		var needManual, isArchive int
		if err := rows.Scan(&e.Name, &needManual, &e.DownloadURL, &e.Note,
			&e.PackagePath, &isArchive, &e.ConfigPath, &e.Platform, &e.ScriptPath); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		e.NeedManualDL = needManual != 0
		e.IsArchive = isArchive != 0
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	if entries == nil {
		entries = []AppEntry{}
	}
	return entries, nil
}

func DeleteEntry(db *sql.DB, name string) error {
	_, err := db.Exec("DELETE FROM entries WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete %q: %w", name, err)
	}
	return nil
}
