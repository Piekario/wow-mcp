// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ledger

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"wowmcp/internal/game/auctions"
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

func withEvents(at int64, entries ...any) snapshot.Snapshot {
	s := dump(at, 5, 100)
	s["eventLog"] = map[string]any{"entries": entries}
	return s
}

func entry(at int64, kind string, fields map[string]any) map[string]any {
	e := map[string]any{"at": at, "kind": kind}
	for k, v := range fields {
		e[k] = v
	}
	return e
}

func TestEventsAreStoredOnceAcrossRepeatedDumps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	l := open(t, path)
	turnIn := entry(1000, "questTurnedIn", map[string]any{"questID": int64(1), "xp": int64(50)})
	other := entry(1000, "questTurnedIn", map[string]any{"questID": int64(2), "xp": int64(70)})
	lvl := entry(1005, "level", map[string]any{"level": int64(6)})
	l.Record(database("Ana", withEvents(2000, turnIn, other)), nil)
	l.Record(database("Ana", withEvents(2100, turnIn, other, lvl)), nil)
	l.Record(database("Ana", withEvents(2100, turnIn, other, lvl)), nil)
	l.Close()

	got, err := open(t, path).Events("Ana", 0, 5000)
	if err != nil || len(got) != 3 {
		t.Fatalf("events = %+v, err = %v", got, err)
	}
	if got[0].QuestID != 1 || got[1].QuestID != 2 || got[2].Kind != "level" || got[2].Level != 6 {
		t.Fatalf("events = %+v", got)
	}
	if ranged, _ := open(t, path).Events("Ana", 1001, 5000); len(ranged) != 1 {
		t.Fatalf("range = %+v", ranged)
	}
	if other, _ := open(t, path).Events("Bob", 0, 5000); len(other) != 0 {
		t.Fatalf("another character's events = %+v", other)
	}
}

func TestLatestMoneyValueWinsForTheSameTime(t *testing.T) {
	l := open(t, filepath.Join(t.TempDir(), "history.db"))
	l.Record(database("Ana", withEvents(2000, entry(1000, "money", map[string]any{"copper": int64(100)}))), nil)
	l.Record(database("Ana", withEvents(2100, entry(1000, "money", map[string]any{"copper": int64(250)}))), nil)
	got, err := l.Events("Ana", 0, 5000)
	if err != nil || len(got) != 1 || got[0].Copper != 250 {
		t.Fatalf("events = %+v, err = %v", got, err)
	}
}

var _ auctions.PriceLog = (*Ledger)(nil)

func withPrices(at int64, prices map[string]any) snapshot.Snapshot {
	s := dump(at, 5, 0)
	s["auctions"] = map[string]any{"prices": prices}
	return s
}

func price(seenAt, buyout int64, seen ...[2]int64) map[string]any {
	p := map[string]any{"minBuyout": buyout, "minBid": buyout / 2, "seenAt": seenAt}
	var visits []any
	for _, v := range seen {
		visits = append(visits, map[string]any{"at": v[0], "minBuyout": v[1]})
	}
	if visits != nil {
		p["seen"] = visits
	}
	return p
}

func TestPricesAreKeptOncePerRealmAcrossRepeatedDumps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	l := open(t, path)
	first := withPrices(100, map[string]any{
		"Linen Cloth":  price(50, 20, [2]int64{10, 30}, [2]int64{50, 20}),
		"No Buyout":    map[string]any{"minBid": int64(5), "seenAt": int64(50)},
		"Never Seen":   map[string]any{"minBuyout": int64(5)},
		"Copper Ore":   price(60, 15),
		"Wool Cloth":   price(70, 40),
		"Zero Buyout":  price(70, 0),
		"Silk Cloth":   price(80, 90, [2]int64{0, 70}),
		"Mageweave Cl": price(0, 90),
	})
	l.Record(database("Ana-Realm", first), nil)
	l.Record(database("Ana-Realm", first), nil)
	l.Record(database("Bob-Realm", withPrices(200, map[string]any{"Linen Cloth": price(150, 25, [2]int64{50, 20}, [2]int64{150, 25})})), nil)
	l.Record(database("Ana-Other", withPrices(200, map[string]any{"Linen Cloth": price(160, 99)})), nil)
	l.Close()

	l = open(t, path)
	got, err := l.Prices("Realm", []string{"Linen Cloth", "No Buyout", "Never Seen", "Zero Buyout", "Silk Cloth", "Mageweave Cl"})
	if err != nil {
		t.Fatal(err)
	}
	want := []auctions.Observation{
		{Item: "Linen Cloth", SeenAt: 10, MinBuyout: 30},
		{Item: "Linen Cloth", SeenAt: 50, MinBuyout: 20, MinBid: 10},
		{Item: "Silk Cloth", SeenAt: 80, MinBuyout: 90, MinBid: 45},
		{Item: "Linen Cloth", SeenAt: 150, MinBuyout: 25, MinBid: 12},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("prices = %+v, want %+v", got, want)
	}
	if other, _ := l.Prices("Other", []string{"Linen Cloth"}); len(other) != 1 || other[0].MinBuyout != 99 {
		t.Fatalf("the other realm = %+v", other)
	}
	if none, err := l.Prices("Realm", nil); err != nil || none != nil {
		t.Fatalf("no items: %v %v", none, err)
	}
}
