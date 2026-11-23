// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questrewards

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

func item(id int64, name, equipLoc string, ilvl, sell int64) map[string]any {
	m := map[string]any{"id": id, "name": name, "itemLevel": ilvl, "sellPrice": sell, "count": int64(1), "requiredLevel": int64(1)}
	if equipLoc != "" {
		m["equipLoc"] = equipLoc
	}
	return m
}

func rewardsOf(choices ...map[string]any) map[string]any {
	list := make([]any, len(choices))
	for i, c := range choices {
		list[i] = c
	}
	return map[string]any{"choices": list}
}

func quest(id int64, title string, rewards map[string]any) map[string]any {
	return map[string]any{"id": id, "title": title, "rewards": rewards, "complete": true}
}

// bare is the Forever dump without NPC windows and with the given quests in the log.
func bare(t *testing.T, quests ...any) snapshot.Snapshot {
	s := fixtures.Forever(t)
	delete(s, "questOffer")
	delete(s, "questTurnIn")
	s["quests"] = quests
	return s
}

func run(t *testing.T, s snapshot.Snapshot, q Query) map[string]any {
	t.Helper()
	out, err := View(s, q, items.Scoring{}, fixtures.At(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func bestName(entry any) string {
	b := snapshot.AsMap(snapshot.AsMap(entry)["best"])
	n, _ := b["name"].(string)
	return n
}

func TestBestIsTheUpgradeNotTheSellValue(t *testing.T) {
	// Is ranged empty in the fixture? We use the "tabard" slot (usually empty) so the upgrade is certain.
	s := bare(t, quest(1, "Q", rewardsOf(
		item(10, "Fancy Tabard", "INVTYPE_TABARD", 50, 5),
		item(11, "Vendor Trash", "", 1, 900))))
	out := run(t, s, Query{})
	qs := snapshot.AsList(out["quests"])
	if len(qs) != 1 || bestName(qs[0]) != "Fancy Tabard" {
		t.Fatalf("best reward: %v", out)
	}
	reason := snapshot.AsMap(snapshot.AsMap(qs[0])["best"])["reason"].(string)
	if !strings.Contains(reason, "tabard") || !strings.Contains(reason, "empty") {
		t.Errorf("justification: %s", reason)
	}
}

func TestNoUpgradeFallsBackToVendorValue(t *testing.T) {
	s := bare(t, quest(1, "Q", rewardsOf(
		item(10, "Cheap Junk", "", 1, 5),
		item(11, "Pricey Junk", "", 1, 900))))
	out := run(t, s, Query{})
	qs := snapshot.AsList(out["quests"])
	if bestName(qs[0]) != "Pricey Junk" {
		t.Fatalf("choice by price: %v", qs[0])
	}
	reason := snapshot.AsMap(snapshot.AsMap(qs[0])["best"])["reason"].(string)
	if !strings.Contains(reason, "vendor value") {
		t.Errorf("justification: %s", reason)
	}
}

func TestUnusableChoiceNeverWinsOverUpgrade(t *testing.T) {
	unusable := item(10, "Plate Helm", "INVTYPE_HEAD", 90, 1000)
	unusable["usable"] = false
	s := bare(t, quest(1, "Q", rewardsOf(unusable, item(11, "Cloth Hood", "INVTYPE_HEAD", 20, 10))))
	out := run(t, s, Query{})
	if got := bestName(snapshot.AsList(out["quests"])[0]); got != "Cloth Hood" {
		t.Errorf("unusable reward won: %s", got)
	}
}

func TestSuspectWhenSameChoicesRepeat(t *testing.T) {
	r := func() map[string]any {
		return rewardsOf(item(10, "A", "INVTYPE_HEAD", 20, 10), item(11, "B", "INVTYPE_CHEST", 20, 10))
	}
	s := bare(t, quest(1, "First", r()), quest(2, "Second", r()),
		quest(3, "Third", rewardsOf(item(12, "C", "INVTYPE_HEAD", 20, 10))))
	out := run(t, s, Query{})
	if out["suspectNote"] == nil {
		t.Error("missing suspectNote")
	}
	suspects := 0
	for _, raw := range snapshot.AsList(out["quests"]) {
		e := snapshot.AsMap(raw)
		if e["suspect"] == true {
			suspects++
			if e["best"] != nil {
				t.Errorf("a suspect quest has no pick: %v", e)
			}
			if !strings.Contains(e["note"].(string), "verify in game") {
				t.Errorf("note: %v", e["note"])
			}
		} else if e["title"] == "Third" && e["best"] == nil {
			t.Errorf("a quest with a unique list should have a pick: %v", e)
		}
	}
	if suspects != 2 {
		t.Errorf("suspects: %d", suspects)
	}
}

func TestDifferentOrderIsNotSuspect(t *testing.T) {
	a, b := item(10, "A", "INVTYPE_HEAD", 20, 10), item(11, "B", "INVTYPE_CHEST", 20, 10)
	s := bare(t, quest(1, "X", rewardsOf(a, b)), quest(2, "Y", rewardsOf(b, a)))
	if out := run(t, s, Query{}); out["suspectNote"] != nil {
		t.Error("a different order is a different list")
	}
}

func TestWindowsFirstAndFilter(t *testing.T) {
	s := fixtures.Forever(t)
	out := run(t, s, Query{})
	qs := snapshot.AsList(out["quests"])
	if len(qs) < 2 || snapshot.AsMap(qs[0])["source"] != fromTurnIn || snapshot.AsMap(qs[1])["source"] != fromOffer {
		t.Fatalf("windows first: %v", out)
	}
	if bestName(qs[0]) == "" {
		t.Errorf("the turn-in quest has a pick: %v", qs[0])
	}
	only := run(t, s, Query{Quest: "65"})
	if len(snapshot.AsList(only["quests"])) != 2 { // turnIn and offer have id 65
		t.Errorf("filter by ID: %v", only["quests"])
	}
	none := run(t, s, Query{Quest: "zzz"})
	if none["note"] == nil {
		t.Errorf("no matches: %v", none)
	}
}

func TestStaleWindowGetsNoPick(t *testing.T) {
	s := fixtures.Forever(t)
	out, err := View(s, Query{Quest: "Defias"}, items.Scoring{}, fixtures.At(5*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range snapshot.AsList(out["quests"]) {
		e := snapshot.AsMap(raw)
		if e["source"] == fromLog {
			continue
		}
		if e["best"] != nil || e["capturedNote"] == nil {
			t.Errorf("a stale window should not have a pick: %v", e)
		}
	}
}

func TestWeightsChangeThePick(t *testing.T) {
	agi := item(10, "Agile Hood", "INVTYPE_HEAD", 10, 1)
	agi["stats"] = map[string]any{"ITEM_MOD_AGILITY_SHORT": int64(10)}
	sta := item(11, "Sturdy Hood", "INVTYPE_HEAD", 30, 1)
	sta["stats"] = map[string]any{"ITEM_MOD_STAMINA_SHORT": int64(10)}
	s := bare(t, quest(1, "Q", rewardsOf(agi, sta)))
	// The head slot in the fixture is occupied; the weights decide the score.
	byAgility := run(t, s, Query{Weights: map[string]float64{"agility": 5}})
	if got := bestName(snapshot.AsList(byAgility["quests"])[0]); got != "Agile Hood" {
		t.Errorf("agility weights: %s (%v)", got, byAgility)
	}
	if snapshot.AsMap(byAgility["scoring"])["method"] != "stat weights" {
		t.Errorf("scoring description: %v", byAgility["scoring"])
	}
}

func TestNoRewardsData(t *testing.T) {
	s := bare(t)
	out := run(t, s, Query{})
	if out["note"] == nil || !strings.Contains(out["note"].(string), "0.6.0") {
		t.Errorf("no data: %v", out)
	}
}

func TestFixedOnlyQuestsListedSeparately(t *testing.T) {
	s := bare(t, map[string]any{"id": int64(5), "title": "Only XP", "complete": false, "rewards": map[string]any{"xp": int64(900), "money": int64(120)}})
	out := run(t, s, Query{})
	if len(snapshot.AsList(out["quests"])) != 0 || len(snapshot.AsList(out["withoutChoices"])) != 1 {
		t.Errorf("fixed rewards: %v", out)
	}
}

func TestAnalyseVerdicts(t *testing.T) {
	s := bare(t, quest(1, "Q", rewardsOf(item(10, "Fancy Tabard", "INVTYPE_TABARD", 50, 5), item(11, "Trash", "", 1, 900))),
		map[string]any{"id": int64(5), "title": "Only XP", "rewards": map[string]any{"xp": int64(900)}})
	rep, err := analyse(s, Query{}, items.Scoring{}, fixtures.At(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Judged) != 1 || len(rep.WithoutChoices) != 1 || rep.WithoutChoices[0].Rewards.XP != 900 {
		t.Fatalf("report: %+v", rep)
	}
	j := rep.Judged[0]
	if j.Verdict != Rated || j.Best == nil || !j.Best.Upgrade || j.Best.label() != "Fancy Tabard" {
		t.Errorf("judgement: %+v", j)
	}
}
