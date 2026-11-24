// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package mcpserver

import (
	"path/filepath"
	"testing"

	"wowmcp/internal/ledger"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

func dumpOf(at int64, class, race string) snapshot.Snapshot {
	return snapshot.Snapshot{
		"dumpedAt":  at,
		"character": map[string]any{"level": int64(30), "moneyCopper": at, "classToken": class, "raceToken": race, "health": int64(50)},
		"spells":    []any{map[string]any{"name": "Attack"}},
		"equipment": []any{map[string]any{"slot": "Chest"}},
	}
}

func TestACharacterWithANewKeyTakesItsOldHistory(t *testing.T) {
	dir := t.TempDir()
	files := storage.NewHistory(dir, 3)
	long, err := ledger.Open(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer long.Close()
	save := func(key string, at int64, class, race string) {
		t.Helper()
		if err := files.Save(key, dumpOf(at, class, race)); err != nil {
			t.Fatal(err)
		}
	}
	save("Ana-Old Realm Two", 100, "WARLOCK", "Scourge")
	save("Ana-Old Realm Two", 200, "WARLOCK", "Scourge")
	save("Ana", 300, "WARLOCK", "Scourge")
	save("Ana", 400, "WARLOCK", "Scourge")
	save("Ana-Old Realm Seven", 150, "HUNTER", "NightElf")
	save("Anabel", 160, "WARLOCK", "Scourge")
	save("Ana-77", 170, "WARLOCK", "Scourge")

	db := &snapshot.Database{Characters: map[string]snapshot.Snapshot{
		"Ana-55": dumpOf(500, "WARLOCK", "Scourge"),
		"Ana-77": dumpOf(510, "WARLOCK", "Scourge"),
	}}
	adoptFormerKeys(db, files, long)
	files.Record(db)
	long.Record(db, files)

	points, err := long.Points("Ana-55")
	if err != nil {
		t.Fatal(err)
	}
	var times []int64
	for _, p := range points {
		times = append(times, p.At)
	}
	if want := []int64{100, 200, 300, 400, 500}; len(times) != len(want) || times[0] != 100 || times[4] != 500 {
		t.Fatalf("ledger points of the new key = %v, want %v (the keep limit must not cut the import)", times, want)
	}
	if kept, _ := files.List("Ana-55"); len(kept) != 3 || kept[2] != 500 {
		t.Fatalf("file history of the new key = %v", kept)
	}
	if old, _ := files.List("Ana"); len(old) != 2 {
		t.Fatalf("the old key lost its dumps: %v", old)
	}
	if other, _ := long.Points("Ana-77"); len(other) != 2 {
		t.Fatalf("a character with its own history adopted someone else's: %+v", other)
	}

	adoptFormerKeys(db, files, long)
	if again, _ := long.Points("Ana-55"); len(again) != 5 {
		t.Fatalf("a second run changed the history: %d points", len(again))
	}
}
