// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package lookup

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/snapshot"
)

func dump() snapshot.Snapshot {
	return snapshot.Snapshot{
		"lookups": []any{
			map[string]any{"kind": "spell", "id": int64(3044), "name": "Arcane Shot", "query": "arcane", "at": int64(100),
				"description": "A shot.", "castTimeMs": int64(0), "maxRange": int64(35),
				"costs": []any{map[string]any{"power": "MANA", "amount": int64(50)}}},
			map[string]any{"kind": "item", "id": int64(5586), "name": "Curvewood Dagger", "query": "5586", "at": int64(90),
				"quality": int64(2), "tooltip": []any{"Curvewood Dagger"}},
		},
		"bags": []any{map[string]any{"index": int64(0), "items": []any{
			map[string]any{"id": int64(2589), "name": "Linen Cloth", "tooltip": []any{"Arcane dust"}},
		}}},
		"quests": []any{map[string]any{"id": int64(5586), "title": "Quest With Same ID"}},
		"spells": []any{map[string]any{"id": int64(3044), "name": "Arcane Shot", "rank": "Rank 1"}},
	}
}

func find(t *testing.T, s snapshot.Snapshot, query, kind string) Result {
	t.Helper()
	r, err := Find(s, Query{Query: query, Kind: kind})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestFindSavedByIDNameAndLink(t *testing.T) {
	for _, query := range []string{"3044", "arcane shot", "|cff|Hspell:3044|h[Arcane Shot]|h|r"} {
		r := find(t, dump(), query, "spell")
		if len(r.Saved) != 1 || r.Saved[0].ID != 3044 || !r.HasDescription() {
			t.Errorf("%q: saved %+v", query, r.Saved)
		}
	}
}

func TestKindNarrowsSavedAndSections(t *testing.T) {
	r := find(t, dump(), "5586", "spell")
	if r.Found() {
		t.Errorf("an item ID must not match as a spell: %+v", r)
	}
	r = find(t, dump(), "5586", "item")
	if len(r.Saved) != 1 || r.ElsewhereN != 0 {
		t.Errorf("a quest's own ID is not an item ID: %+v", r)
	}
}

func TestNameHitsIgnoreDescriptionText(t *testing.T) {
	r := find(t, dump(), "arcane", "")
	for _, g := range r.Elsewhere {
		if g.Section == "bags" {
			t.Errorf("Linen Cloth only mentions arcane in its tooltip: %+v", g)
		}
	}
	if len(r.Elsewhere) != 1 || r.Elsewhere[0].Section != "spells" {
		t.Errorf("elsewhere: %+v", r.Elsewhere)
	}
}

func TestNothingFoundAsksThePlayer(t *testing.T) {
	r := find(t, dump(), "Zzzz", "item")
	if r.Found() {
		t.Fatal("found")
	}
	out := r.Render(snapshot.Freshness{Now: func() time.Time { return time.Unix(1000, 0) }})
	ask, _ := out["ask"].(string)
	if !strings.Contains(ask, "/mcp item Zzzz") || !strings.Contains(ask, "/mcp reload") || strings.Contains(ask, "/mcp spell") {
		t.Errorf("ask: %q", ask)
	}
}

func TestMentionWithoutDescriptionAsksForTheText(t *testing.T) {
	r := find(t, dump(), "linen", "")
	if !r.Found() || r.HasDescription() || r.hint() == "" {
		t.Errorf("result: %+v", r)
	}
	if !strings.Contains(r.hint(), "/mcp spell linen") {
		t.Errorf("both commands expected for an open kind: %q", r.hint())
	}
}

func TestRejectsBadInput(t *testing.T) {
	if _, err := Find(dump(), Query{Query: "x", Kind: "quest"}); err == nil {
		t.Error("an unknown kind must be an error")
	}
	if _, err := Find(dump(), Query{Query: "  "}); err == nil {
		t.Error("an empty query must be an error")
	}
}

func TestOldAddonDumpIsReported(t *testing.T) {
	r := find(t, snapshot.Snapshot{}, "arcane", "")
	if r.HasLookups {
		t.Error("no lookups section in the dump")
	}
}
