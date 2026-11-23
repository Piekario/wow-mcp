// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package leftovers

import (
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func addQuestItem(s snapshot.Snapshot, bag int, item map[string]any) {
	b := snapshot.AsMap(snapshot.AsList(s["bags"])[bag])
	item["type"] = "Quest"
	b["items"] = append(snapshot.AsList(b["items"]), item)
}

func TestNoQuestItems(t *testing.T) {
	out := View(fixtures.Forever(t))
	if out["questItems"] != 0 || out["note"] == nil || out["leftovers"] != nil {
		t.Errorf("the fixture has no quest items: %v", out)
	}
}

func TestLeftoversAndKeep(t *testing.T) {
	s := fixtures.Forever(t)
	// Death Cap is an objective of the quest Cave Mushrooms; Crate of Wine matches nothing; Strange Map begins a quest.
	addQuestItem(s, 0, map[string]any{"name": "Death Cap", "count": int64(2)})
	addQuestItem(s, 0, map[string]any{"name": "Crate of Wine", "count": int64(1)})
	addQuestItem(s, 1, map[string]any{"name": "Strange Map", "count": int64(1), "tooltip": []any{"Strange Map", "This Item Begins a Quest"}})
	snapshot.AsMap(s["bank"])["containers"] = []any{map[string]any{"name": "Bank", "items": []any{
		map[string]any{"name": "Crate of Wine", "count": int64(3), "type": "Quest"},
	}}}

	out := View(s)
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "leftovers", text, `"questItems":3`, `"confidence":"medium"`,
		`"forQuests":["Cave Mushrooms"]`, `"name":"Crate of Wine","reason":`, `"where":["bags","bank"]`, `"count":4`,
		`"completedQuestsKnown":3`)
	leftovers := snapshot.AsList(out["leftovers"])
	if len(leftovers) != 1 || snapshot.AsMap(leftovers[0])["confidence"] != ConfidenceMedium {
		t.Fatalf("leftovers: %v", leftovers)
	}
	keep := snapshot.AsList(out["keep"])
	if len(keep) != 2 {
		t.Fatalf("keep: %v", keep)
	}
	if !strings.Contains(fixtures.JSON(t, keep), "begins a quest") {
		t.Errorf("item that begins a quest: %v", keep)
	}
	if !strings.Contains(out["advice"].(string), "nothing should be removed automatically") {
		t.Errorf("no warning: %v", out["advice"])
	}
}

func TestNoQuestLogMeansNoConfidence(t *testing.T) {
	s := fixtures.Forever(t)
	addQuestItem(s, 0, map[string]any{"name": "Crate of Wine", "count": int64(1)})
	delete(s, "quests")
	out := View(s)
	if out["confidence"] != ConfidenceNone || snapshot.AsMap(snapshot.AsList(out["leftovers"])[0])["confidence"] != ConfidenceNone {
		t.Errorf("without a log: %v", out)
	}
	// An empty log (list) is still a log: the candidate has medium confidence.
	s["quests"] = []any{}
	if View(s)["confidence"] != ConfidenceMedium {
		t.Error("an empty quest log is still a log")
	}
}

func TestAllQuestItemsNeeded(t *testing.T) {
	s := fixtures.Forever(t)
	addQuestItem(s, 0, map[string]any{"name": "scaber stalk", "count": int64(5)}) // letter case does not matter
	out := View(s)
	if out["confidence"] != "n/a" || len(snapshot.AsList(out["leftovers"])) != 0 {
		t.Errorf("everything needed: %v", out)
	}
}

func TestInspectSeparatesNeededItemsFromLeftovers(t *testing.T) {
	s := fixtures.Forever(t)
	addQuestItem(s, 0, map[string]any{"name": "Death Cap", "count": int64(2)})
	addQuestItem(s, 0, map[string]any{"name": "Crate of Wine", "count": int64(1)})
	r := Inspect(s)
	left := r.Leftovers()
	if len(left) != 1 || left[0].Name != "Crate of Wine" || left[0].Confidence != ConfidenceMedium || r.Confidence() != ConfidenceMedium {
		t.Errorf("leftovers = %+v, confidence %q", left, r.Confidence())
	}
	if !r.HasLog || len(r.Verdicts) != 2 || !r.Verdicts[1].Keep || r.Verdicts[1].ForQuests[0] != "Cave Mushrooms" {
		t.Errorf("verdicts = %+v", r.Verdicts)
	}
}
