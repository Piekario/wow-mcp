// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package ledger is the long history: a SQLite file in the server's own directory with one compact row per dump.
// Full dumps stay in storage.History (a bounded number of files); the ledger keeps their numbers without a limit.
package ledger

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite" // cgo-free driver: the server ships as one static binary per platform
)

// migrations are applied in order; PRAGMA user_version is the number already applied. Never edit an entry, append.
var migrations = []string{
	`CREATE TABLE points (
		character TEXT NOT NULL,
		dumped_at INTEGER NOT NULL,
		level INTEGER NOT NULL, xp INTEGER NOT NULL, xp_max INTEGER NOT NULL,
		money INTEGER NOT NULL, rested INTEGER NOT NULL,
		played INTEGER NOT NULL, deaths INTEGER NOT NULL,
		completed_quests INTEGER NOT NULL,
		zone TEXT NOT NULL,
		PRIMARY KEY (character, dumped_at)
	) WITHOUT ROWID`,
	`CREATE TABLE events (
		character TEXT NOT NULL,
		at INTEGER NOT NULL,
		kind TEXT NOT NULL,
		key TEXT NOT NULL,
		data TEXT NOT NULL,
		PRIMARY KEY (character, at, kind, key)
	) WITHOUT ROWID`,
}

// Ledger is safe for concurrent use, also by several server processes on the same file (the desktop app starts more
// than one): WAL mode, a busy timeout, and rows keyed by character and dump time so a second writer changes nothing.
type Ledger struct {
	db *sql.DB
}

// Open opens or creates the ledger file and brings its schema up to date.
func Open(path string) (*Ledger, error) {
	pragmas := url.Values{"_pragma": {"busy_timeout(5000)", "journal_mode(WAL)", "synchronous(NORMAL)"}, "_txlock": {"immediate"}}
	dsn := url.URL{Scheme: "file", Opaque: filepath.ToSlash(path), RawQuery: pragmas.Encode()}
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("history database %s: %w", path, err)
	}
	return &Ledger{db: db}, nil
}

func (l *Ledger) Close() error { return l.db.Close() }

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return fmt.Errorf("written by a newer server (schema %d, this server knows %d)", version, len(migrations))
	}
	for i := version; i < len(migrations); i++ {
		if _, err := tx.Exec(migrations[i]); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations))); err != nil {
		return err
	}
	return tx.Commit()
}
