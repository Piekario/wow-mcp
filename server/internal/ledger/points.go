// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ledger

import (
	"log"

	"wowmcp/internal/game/auctions"
	"wowmcp/internal/game/eventlog"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Record adds the dump of every character in the database. A character the ledger sees for the first time also gets
// the dumps already kept in files, so the history from before the ledger is not lost. Errors go to the log only:
// a tool answers from the dump even when the ledger cannot be written.
func (l *Ledger) Record(db *snapshot.Database, files progress.History) {
	for _, name := range db.CharacterNames() {
		if err := l.record(name, db.Characters[name], files); err != nil {
			log.Printf("history database: %s: %v", name, err)
		}
	}
}

func (l *Ledger) record(character string, s snapshot.Snapshot, files progress.History) error {
	var points []progress.Point
	if p, ok := progress.PointOf(s); ok {
		points = append(points, p)
	}
	if files != nil {
		known, err := l.has(character)
		if err != nil {
			return err
		}
		if !known {
			points = append(points, pointsFromFiles(character, files)...)
		}
	}
	if err := l.add(character, points); err != nil {
		return err
	}
	if err := l.addEvents(character, eventlog.Read(s)); err != nil {
		return err
	}
	return l.addPrices(snapshot.RealmOf(character), auctions.Observations(s))
}

func pointsFromFiles(character string, files progress.History) []progress.Point {
	times, err := files.List(character)
	if err != nil {
		return nil
	}
	var points []progress.Point
	for _, at := range times {
		if s, err := files.Load(character, at); err == nil {
			if p, ok := progress.PointOf(s); ok {
				points = append(points, p)
			}
		}
	}
	return points
}

func (l *Ledger) has(character string) (bool, error) {
	var found bool
	err := l.db.QueryRow("SELECT EXISTS (SELECT 1 FROM points WHERE character = ?)", character).Scan(&found)
	return found, err
}

func (l *Ledger) add(character string, points []progress.Point) error {
	if len(points) == 0 {
		return nil
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range points {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO points
			(character, dumped_at, level, xp, xp_max, money, rested, played, deaths, completed_quests, zone)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			character, p.At, p.Level, p.XP, p.XPMax, p.Money, p.Rested, p.Played, p.Deaths, p.CompletedQuests, p.Zone); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Points implements progress.Series.
func (l *Ledger) Points(character string) ([]progress.Point, error) {
	rows, err := l.db.Query(`SELECT dumped_at, level, xp, xp_max, money, rested, played, deaths, completed_quests, zone
		FROM points WHERE character = ? ORDER BY dumped_at`, character)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []progress.Point
	for rows.Next() {
		var p progress.Point
		if err := rows.Scan(&p.At, &p.Level, &p.XP, &p.XPMax, &p.Money, &p.Rested, &p.Played, &p.Deaths, &p.CompletedQuests, &p.Zone); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}
