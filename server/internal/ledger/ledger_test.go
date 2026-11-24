// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ledger

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

var _ progress.Series = (*Ledger)(nil)

func dump(at, level, money int64) snapshot.Snapshot {
	return snapshot.Snapshot{
		"dumpedAt": at,
		"character": map[string]any{"level": level, "xp": int64(10), "xpMax": int64(100), "moneyCopper": money,
			"zone": "Elwynn Forest", "health": int64(50)},
		"spells":    []any{map[string]any{"name": "Attack"}},
		"equipment": []any{map[string]any{"slot": "Chest"}},
	}
}

func database(name string, s snapshot.Snapshot) *snapshot.Database {
	return &snapshot.Database{Characters: map[string]snapshot.Snapshot{name: s}}
}

func open(t *testing.T, path string) *Ledger {
	t.Helper()
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestRecordKeepsOnePointPerDumpAndSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dir with space & żółć", "history.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	l := open(t, path)
	l.Record(database("Ana-Realm", dump(100, 5, 1200)), nil)
	l.Record(database("Ana-Realm", dump(100, 5, 1200)), nil)
	l.Record(database("Ana-Realm", dump(200, 6, 900)), nil)
	l.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the database is not at the given path: %v", err)
	}

	points, err := open(t, path).Points("Ana-Realm")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 || points[0].At != 100 || points[1].Level != 6 || points[1].Money != 900 ||
		points[1].Played != -1 || points[1].Zone != "Elwynn Forest" {
		t.Fatalf("points = %+v", points)
	}
}

func TestRecordSkipsUnloadedDumps(t *testing.T) {
	l := open(t, filepath.Join(t.TempDir(), "history.db"))
	empty := snapshot.Snapshot{"dumpedAt": int64(300), "character": map[string]any{"level": int64(5), "health": int64(0)},
		"spells": []any{}, "equipment": []any{}, "bags": []any{}}
	if !empty.IsUnloadedDump() {
		t.Fatal("the test dump should look unloaded")
	}
	l.Record(database("Ana", empty), nil)
	if points, _ := l.Points("Ana"); len(points) != 0 {
		t.Fatalf("an empty-client dump was recorded: %+v", points)
	}
}

func TestFirstRecordImportsTheFileHistory(t *testing.T) {
	dir := t.TempDir()
	files := storage.NewHistory(filepath.Join(dir, "history"), 50)
	for _, at := range []int64{100, 200} {
		if err := files.Save("Ana", dump(at, 5, at)); err != nil {
			t.Fatal(err)
		}
	}
	l := open(t, filepath.Join(dir, "history.db"))
	l.Record(database("Ana", dump(300, 6, 300)), files)
	if points, _ := l.Points("Ana"); len(points) != 3 || points[0].At != 100 {
		t.Fatalf("points = %+v", points)
	}
}

func TestSeveralConnectionsWriteTheSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	open(t, path)
	var wg sync.WaitGroup
	for range 4 {
		l := open(t, path)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for at := int64(1); at <= 50; at++ {
				l.Record(database("Ana", dump(at, 5, at)), nil)
			}
		}()
	}
	wg.Wait()
	if points, err := open(t, path).Points("Ana"); err != nil || len(points) != 50 {
		t.Fatalf("points = %d, err = %v", len(points), err)
	}
}

func TestOpenRefusesANewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	l := open(t, path)
	if _, err := l.db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	l.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("a database from a newer server was opened")
	}
}
