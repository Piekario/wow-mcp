// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ledger

import (
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Adopt copies what the ledger keeps under the character key from to the key to; the rows of from stay. Prices move
// with it only when from had no realm in its key, because only then they were stored under no realm at all.
func (l *Ledger) Adopt(from, to string) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO points
		SELECT ?, dumped_at, level, xp, xp_max, money, rested, played, deaths, completed_quests, zone
		FROM points WHERE character = ?`, to, from); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO events SELECT ?, at, kind, key, data FROM events WHERE character = ?`,
		to, from); err != nil {
		return err
	}
	if snapshot.RealmOf(from) == "" && snapshot.RealmOf(to) != "" {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO prices SELECT ?, item, seen_at, min_buyout, min_bid FROM prices WHERE realm = ''`,
			snapshot.RealmOf(to)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Import adds the points of the dumps the file history keeps for the character.
func (l *Ledger) Import(character string, files progress.History) error {
	return l.add(character, pointsFromFiles(character, files))
}
