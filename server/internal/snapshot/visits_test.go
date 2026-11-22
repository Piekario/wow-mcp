// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import (
	"testing"
	"time"
)

func TestVisitsNewestPerNpc(t *testing.T) {
	s := Snapshot{
		"vendor": map[string]any{"npc": "Laird", "capturedAt": int64(300)},
		"vendors": map[string]any{
			"Laird": map[string]any{"npc": "Laird", "capturedAt": int64(100), "old": true},
			"Gryan": map[string]any{"npc": "Gryan", "capturedAt": int64(200)},
		},
	}
	got := s.Visits("vendor", "vendors")
	if len(got) != 2 || got[0].Npc != "Laird" || got[1].Npc != "Gryan" {
		t.Fatalf("visit order: %+v", got)
	}
	if got[0].Section["old"] != nil {
		t.Error("the newest offer must remain for the NPC (from the vendor section)")
	}
	if at, ok := got[0].CapturedAt(); !ok || at != 300 {
		t.Errorf("CapturedAt = %d %v", at, ok)
	}
}

func TestVisitsOlderSingleDoesNotReplaceNewerMapEntry(t *testing.T) {
	s := Snapshot{
		"trainer":  map[string]any{"npc": "Astaia", "capturedAt": int64(10)},
		"trainers": map[string]any{"Astaia": map[string]any{"npc": "Astaia", "capturedAt": int64(20), "new": true}},
	}
	got := s.Visits("trainer", "trainers")
	if len(got) != 1 || got[0].Section["new"] != true {
		t.Errorf("visits: %+v", got)
	}
	if len(Snapshot{}.Visits("trainer", "trainers")) != 0 {
		t.Error("an empty snapshot has no visits")
	}
}

func TestIsStale(t *testing.T) {
	f := Freshness{Now: func() time.Time { return time.Unix(10000, 0) }, StaleAfter: time.Hour}
	if f.IsStale(10000-60) || !f.IsStale(10000-3600) {
		t.Error("stale data threshold")
	}
	if (Freshness{Now: f.Now}).IsStale(0) {
		t.Error("StaleAfter 0 disables warnings")
	}
}

func TestFindVisit(t *testing.T) {
	visits := []Visit{{Npc: "Einris Brightspear"}, {Npc: "Astaia"}, {Npc: "Astaia Junior"}}
	if v, ok := FindVisit(visits, "astaia"); !ok || v.Npc != "Astaia" {
		t.Errorf("the exact name wins: %+v %v", v, ok)
	}
	if v, ok := FindVisit(visits, "bright"); !ok || v.Npc != "Einris Brightspear" {
		t.Errorf("fragment: %+v %v", v, ok)
	}
	if _, ok := FindVisit(visits, "zzz"); ok {
		t.Error("no match")
	}
	if _, ok := FindVisit(visits, "  "); ok {
		t.Error("an empty name selects nothing")
	}
	if names := VisitNames(visits); len(names) != 3 || names[1] != "Astaia" {
		t.Errorf("names: %v", names)
	}
}
