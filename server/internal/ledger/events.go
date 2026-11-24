// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ledger

import (
	"encoding/json"

	"wowmcp/internal/game/eventlog"
)

// addEvents stores the entries of a dump. The addon carries entries across saves, so most are already here: an event
// is identified by character, time, kind and Entry.Key, and a repeated one rewrites its data (a money entry keeps its
// time while its copper changes, and the latest value is the right one).
func (l *Ledger) addEvents(character string, entries []eventlog.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO events (character, at, kind, key, data) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (character, at, kind, key) DO UPDATE SET data = excluded.data`,
			character, e.At, e.Kind, e.Key(), string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Events returns the character's events with from <= at <= to, oldest first.
func (l *Ledger) Events(character string, from, to int64) ([]eventlog.Entry, error) {
	rows, err := l.db.Query(`SELECT data FROM events WHERE character = ? AND at BETWEEN ? AND ?
		ORDER BY at, kind, key`, character, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []eventlog.Entry
	for rows.Next() {
		var data string
		var e eventlog.Entry
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &e); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
