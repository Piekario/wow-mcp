// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package compare

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func char(level int64, money int64, dumped int64, gear map[string]int64) snapshot.Snapshot {
	var eq []any
	for slot, lvl := range gear {
		eq = append(eq, map[string]any{"slot": slot, "name": "Item " + slot, "itemLevel": lvl, "quality": int64(2)})
	}
	return snapshot.Snapshot{
		"character":   map[string]any{"level": level, "moneyCopper": money, "class": "Hunter", "race": "Night Elf"},
		"equipment":   eq,
		"professions": []any{map[string]any{"name": "Mining", "rank": int64(10), "maxRank": int64(75)}},
		"dumpedAt":    dumped,
	}
}

func testDB() *snapshot.Database {
	return &snapshot.Database{Last: "Aaa-Realm", Characters: map[string]snapshot.Snapshot{
		"Aaa-Realm": char(20, 5000, fixtures.ForeverDumpedAt, map[string]int64{"head": 20, "chest": 10, "shirt": 1}),
		"Bbb-Realm": char(30, 90000, fixtures.ForeverDumpedAt-7200, map[string]int64{"head": 15, "legs": 30}),
		"Ccc-Realm": char(5, 1, fixtures.ForeverDumpedAt, nil),
	}}
}

func TestResolve(t *testing.T) {
	db := testDB()
	two := &snapshot.Database{Last: "Aaa-Realm", Characters: map[string]snapshot.Snapshot{"Aaa-Realm": db.Characters["Aaa-Realm"], "Bbb-Realm": db.Characters["Bbb-Realm"]}}
	one := &snapshot.Database{Characters: map[string]snapshot.Snapshot{"Aaa-Realm": db.Characters["Aaa-Realm"]}}
	tests := []struct {
		name    string
		db      *snapshot.Database
		q       Query
		wa, wb  string
		wantErr string
	}{
		{"full names", db, Query{"Aaa-Realm", "Bbb-Realm"}, "Aaa-Realm", "Bbb-Realm", ""},
		{"without realm and case", db, Query{"bbb", "CCC"}, "Bbb-Realm", "Ccc-Realm", ""},
		{"empty a is the last one", db, Query{"", "Bbb"}, "Aaa-Realm", "Bbb-Realm", ""},
		{"one other character", two, Query{}, "Aaa-Realm", "Bbb-Realm", ""},
		{"empty b with several", db, Query{A: "Aaa"}, "", "", "Available"},
		{"unknown", db, Query{"Aaa", "Zzz"}, "", "", `no character "Zzz"`},
		{"same", db, Query{"Aaa", "aaa-realm"}, "", "", "same character"},
		{"one character on the account", one, Query{}, "", "", "only one character"},
		{"empty database", &snapshot.Database{}, Query{}, "", "", "no characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b, err := Resolve(tt.db, tt.q)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %v, wanted %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || a != tt.wa || b != tt.wb {
				t.Errorf("got %q %q %v", a, b, err)
			}
		})
	}
}

func TestView(t *testing.T) {
	fresh := fixtures.At(0)
	out, err := View(testDB(), Query{"Aaa", "Bbb"}, fresh)
	if err != nil {
		t.Fatal(err)
	}
	cards := out["characters"].([]any)
	a, b := cards[0].(map[string]any), cards[1].(map[string]any)
	if a["avgItemLevel"] != 15.0 || b["avgItemLevel"] != 22.5 {
		t.Errorf("average ilvl: %v %v", a["avgItemLevel"], b["avgItemLevel"])
	}
	if b["ageMinutes"] != int64(120) || b["stale"] != true {
		t.Errorf("age of b: %v", b)
	}
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "comparison", text, `"slot":"head"`, `"better":"a"`, `"better":"b"`, "Higher level: Bbb-Realm", "The dumps come from different moments", "Mining 10/75")
	if strings.Contains(text, `"slot":"shirt","a":{"itemLevel":1,"name":"Item shirt","quality":"Uncommon"},"b":null,"better"`) {
		t.Error("shirt should not have better")
	}
}

func TestNoDataNoteWhenSameTime(t *testing.T) {
	out, err := View(testDB(), Query{"Aaa", "Ccc"}, fixtures.At(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["dataNote"]; ok {
		t.Error("dumps from the same moment should not have dataNote")
	}
	c := out["characters"].([]any)[1].(map[string]any)
	if _, ok := c["avgItemLevel"]; ok {
		t.Error("no equipment = no average")
	}
}

func TestViewError(t *testing.T) {
	if _, err := View(testDB(), Query{"Aaa", "Nope"}, fixtures.At(0)); err == nil {
		t.Error("expected an error")
	}
}

func TestCompareRulesOnTheTypedReport(t *testing.T) {
	report, err := Compare(testDB(), Query{"Aaa", "Bbb"}, fixtures.At(0))
	if err != nil {
		t.Fatal(err)
	}
	if report.A.AverageItemLevel != 15 || report.A.GearPieces != 2 || report.B.Stale != true || report.B.AgeMinutes != 120 {
		t.Errorf("cards: %+v %+v", report.A, report.B)
	}
	if len(report.Edges) != 3 || report.Edges[0] != (Edge{Kind: HigherLevel, Leader: "Bbb-Realm", Other: "Aaa-Realm", Gap: 10}) {
		t.Errorf("edges: %+v", report.Edges)
	}
	if !report.Skew.Unfair() || report.Skew.Older != "Bbb-Realm" || report.Skew.GapSeconds != 7200 {
		t.Errorf("skew: %+v", report.Skew)
	}
	for _, s := range report.Slots {
		if s.Slot == "head" && s.Better != BetterA {
			t.Errorf("head: %+v", s)
		}
		if s.Slot == "shirt" && s.Compared {
			t.Error("a shirt is not compared")
		}
	}
}
