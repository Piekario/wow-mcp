// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ledger

import (
	"path/filepath"
	"testing"

	"wowmcp/internal/snapshot"
)

func withPvP(s snapshot.Snapshot, honor, kills int64) snapshot.Snapshot {
	s["pvp"] = map[string]any{"honor": honor, "honorableKills": kills}
	return s
}

func TestRecordKeepsHonorAndLeavesOtherDumpsUnknown(t *testing.T) {
	l := open(t, filepath.Join(t.TempDir(), "history.db"))
	l.Record(database("Ana", dump(100, 5, 1)), nil)
	l.Record(database("Ana", withPvP(dump(200, 5, 1), 340, 1234)), nil)
	points, err := l.Points("Ana")
	if err != nil || len(points) != 2 {
		t.Fatalf("points = %+v, err = %v", points, err)
	}
	if points[0].Honor != -1 || points[0].HonorableKills != -1 || points[1].Honor != 340 || points[1].HonorableKills != 1234 {
		t.Errorf("honor: %+v", points)
	}
	if err := l.Adopt("Ana", "Ana-Realm"); err != nil {
		t.Fatal(err)
	}
	if adopted, _ := l.Points("Ana-Realm"); len(adopted) != 2 || adopted[1].Honor != 340 || adopted[0].Honor != -1 {
		t.Errorf("adopted points: %+v", adopted)
	}
}

// A history.db written before the pvp columns existed (schema 3) opens, keeps its rows as unknown and takes new ones.
func TestOpenMigratesAnOlderDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	l := open(t, path)
	for _, stmt := range []string{
		"ALTER TABLE points DROP COLUMN honor", "ALTER TABLE points DROP COLUMN honorable_kills", "PRAGMA user_version = 3",
		"INSERT INTO points VALUES ('Ana', 50, 4, 1, 10, 700, 0, 100, 0, 2, 'Durotar')",
	} {
		if _, err := l.db.Exec(stmt); err != nil {
			t.Fatal(stmt, err)
		}
	}
	l.Close()
	l = open(t, path)
	l.Record(database("Ana", withPvP(dump(60, 5, 1), 10, 2)), nil)
	points, err := l.Points("Ana")
	if err != nil || len(points) != 2 || points[0].Honor != -1 || points[0].Money != 700 || points[1].Honor != 10 {
		t.Fatalf("points after the migration = %+v, err = %v", points, err)
	}
	var version int
	if err := l.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != len(migrations) {
		t.Errorf("schema version = %d (%v)", version, err)
	}
}
