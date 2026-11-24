// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package dungeonloot

import (
	"fmt"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

func verdicts(t *testing.T, r Report, dungeon string) map[string]Verdict {
	t.Helper()
	out := map[string]Verdict{}
	for _, d := range r.Dungeons {
		if d.Dungeon.Name != dungeon {
			continue
		}
		for _, b := range d.Bosses {
			for _, it := range b.Items {
				out[it.Loot.Name] = it.Verdict
			}
		}
		return out
	}
	t.Fatalf("no dungeon %q in %+v", dungeon, r.Saved)
	return nil
}

func TestVerdictsForAHunter(t *testing.T) {
	all := false
	r, err := Analyse(fixtures.Forever(t), Query{UpgradesOnly: &all}, items.Scoring{})
	if err != nil || r.State != Ready {
		t.Fatalf("state %v, err %v", r.State, err)
	}
	got := verdicts(t, r, "The Deadmines")
	want := map[string]Verdict{
		"Smite's Reaver": Upgrade, "Cape of the Brotherhood": Upgrade, "Smelting Pants": Upgrade,
		"Gold-flecked Gloves": LaterLevel, "Cookie's Stirring Rod": NotForClass, "Cruel Barb": Loading,
	}
	for name, v := range want {
		if got[name] != v {
			t.Errorf("%s: verdict %v, want %v", name, got[name], v)
		}
	}
}

func TestUpgradesOnlyCountsTheRest(t *testing.T) {
	r, err := Analyse(fixtures.Forever(t), Query{Dungeon: "dead"}, items.Scoring{})
	if err != nil || len(r.Dungeons) != 1 {
		t.Fatalf("dungeons %d, err %v", len(r.Dungeons), err)
	}
	d := r.Dungeons[0]
	if d.Skipped[NotForClass] != 1 || d.Skipped[LaterLevel] != 1 || d.Skipped[Loading] != 1 {
		t.Errorf("skipped: %v", d.Skipped)
	}
	if len(d.Loading) != 1 || d.Loading[0] != "Cruel Barb" {
		t.Errorf("loading: %v", d.Loading)
	}
	if d.Bosses[0].Name != "Sneed's Shredder" || d.Bosses[len(d.Bosses)-1].Name != "" {
		t.Errorf("boss order: %+v", d.Bosses)
	}
}

func TestSlotFilter(t *testing.T) {
	r, err := Analyse(fixtures.Forever(t), Query{Slot: "cloak"}, items.Scoring{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range r.Dungeons {
		for _, b := range d.Bosses {
			for _, it := range b.Items {
				if it.Loot.Name != "Cape of the Brotherhood" {
					t.Errorf("slot back: got %s", it.Loot.Name)
				}
			}
		}
	}
	if _, err := Analyse(fixtures.Forever(t), Query{Slot: "moon"}, items.Scoring{}); err == nil {
		t.Error("an unknown slot should be an error")
	}
}

func TestAbsentAndUnavailable(t *testing.T) {
	s := fixtures.Forever(t)
	delete(s, "dungeonLoot")
	if r, _ := Analyse(s, Query{}, items.Scoring{}); r.State != Absent {
		t.Errorf("no section: %v", r.State)
	}
	if r, _ := Analyse(fixtures.Modern(t), Query{}, items.Scoring{}); r.State != Unavailable || r.Reason == "" {
		t.Errorf("modern mock without the journal API: %v %q", r.State, r.Reason)
	}
}

func TestUnknownClassKeepsItems(t *testing.T) {
	s := fixtures.Forever(t)
	delete(s.Character(), "classToken")
	all := false
	r, _ := Analyse(s, Query{UpgradesOnly: &all}, items.Scoring{})
	if v := verdicts(t, r, "The Deadmines")["Cookie's Stirring Rod"]; v == NotForClass {
		t.Error("without a class nothing is left out for its type")
	}
}

func TestNothingReadShowsLastAttempt(t *testing.T) {
	s := fixtures.Forever(t)
	s["dungeonLoot"] = map[string]any{
		"instances": []any{},
		"lastAttempt": map[string]any{"at": int64(1794760000), "query": "deadmins", "outcome": "noMatch", "instancesInJournal": int64(31),
			"tiers": int64(3), "inInstance": false, "instanceNames": []any{"The Deadmines", "Wailing Caverns"}},
	}
	r, err := Analyse(s, Query{}, items.Scoring{})
	if err != nil || r.State != NothingRead {
		t.Fatalf("state %v, err %v", r.State, err)
	}
	out := r.Render()
	text := fmt.Sprint(out)
	for _, want := range []string{"journalOffers", "The Deadmines", "noMatch", "deadmins", "journalOffersNote"} {
		if !strings.Contains(text, want) {
			t.Errorf("the answer lacks %q: %v", want, out)
		}
	}
	s["dungeonLoot"].(map[string]any)["lastAttempt"].(map[string]any)["instancesInJournal"] = int64(0)
	s["dungeonLoot"].(map[string]any)["lastAttempt"].(map[string]any)["hint"] = "The journal has no data yet."
	if note := fmt.Sprint(mustRender(t, s)["note"]); !strings.Contains(note, "no data yet") {
		t.Errorf("hint: %v", note)
	}
}

func mustRender(t *testing.T, s snapshot.Snapshot) map[string]any {
	t.Helper()
	r, err := Analyse(s, Query{}, items.Scoring{})
	if err != nil {
		t.Fatal(err)
	}
	return r.Render()
}
