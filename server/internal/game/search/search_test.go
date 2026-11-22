// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package search

import (
	"fmt"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func group(t *testing.T, out map[string]any, section string) map[string]any {
	t.Helper()
	for _, raw := range snapshot.AsList(out["groups"]) {
		if g := snapshot.AsMap(raw); g["section"] == section {
			return g
		}
	}
	t.Fatalf("missing group %s in %s", section, fixtures.JSON(t, out))
	return nil
}

func firstPath(t *testing.T, g map[string]any) string {
	t.Helper()
	path, _ := snapshot.AsMap(snapshot.AsList(g["hits"])[0])["path"].([]string)
	return strings.Join(path, " > ")
}

func TestSearchAcrossSections(t *testing.T) {
	s := fixtures.Forever(t)
	out, err := Run(s, Query{Query: "dEaTh CaP"})
	if err != nil {
		t.Fatal(err)
	}
	// An objective of the Cave Mushrooms quest (case does not matter).
	quests := group(t, out, "quests")
	fixtures.WantAll(t, "quests", fixtures.JSON(t, quests), `"label":"Death Cap: 0/1"`, `"matchedIn":"text"`)

	if got := firstPath(t, quests); got != "quests > Cave Mushrooms > Death Cap: 0/1" {
		t.Errorf("hit path: %q", got)
	}

	out, _ = Run(s, Query{Query: "cloth"})
	// Bags (Linen Cloth and the Cloth subtype), bank, mail (attachment), not auctions.
	for _, section := range []string{"bags", "bank", "mail"} {
		group(t, out, section)
	}
	mail := group(t, out, "mail")
	fixtures.WantAll(t, "mail", fixtures.JSON(t, mail), `"label":"Linen Cloth"`, `"summary":"count 20"`)
	if got := firstPath(t, mail); got != "mail > mats > Linen Cloth" {
		t.Errorf("attachment path: %q", got)
	}
	bags := fixtures.JSON(t, group(t, out, "bags"))
	fixtures.WantAll(t, "bags", bags, `"label":"Linen Cloth"`, `"summary":"count 14, Trade Goods, Cloth"`, `"matchedIn":"name"`)

	out, _ = Run(s, Query{Query: "9 - 18 damage"})
	// Item description (tooltip) in equipment.
	fixtures.WantAll(t, "tooltip", fixtures.JSON(t, group(t, out, "equipment")), `"matchedIn":"tooltip"`, `"matchedText":"9 - 18 Damage | Speed 1.60"`)

	out, _ = Run(s, Query{Query: "Disengage"})
	fixtures.WantAll(t, "trainer", fixtures.JSON(t, group(t, out, "trainer")), `"summary":"Rank 1, available, costCopper 21s 0c, levelRequired 20"`)
	out, _ = Run(s, Query{Query: "Copper"})
	fixtures.WantAll(t, "auctions and mail", fixtures.JSON(t, out), `"section":"auctions"`, `"summary":"buyout 6s 0c, count 10, timeLeftSeconds 7200"`, `"section":"mail"`)
	out, _ = Run(s, Query{Query: "Darnassus"})
	fixtures.WantAll(t, "reputation", fixtures.JSON(t, group(t, out, "reputation")), `"summary":"Friendly, value 4200"`)
	out, _ = Run(s, Query{Query: "Coarse Thread"})
	fixtures.WantAll(t, "recipes", fixtures.JSON(t, group(t, out, "recipes")), "Embossed Leather Gloves", `"summary":"have 0, need 2"`)
	// The Craft window (Beast Training) exists only in older clients: the section directly.
	s["recipes"].(map[string]any)["Beast Training"] = map[string]any{"name": "Beast Training", "recipes": []any{
		map[string]any{"name": "Growl", "rank": "Rank 3", "difficulty": "used", "levelRequired": int64(10)},
	}}
	out, _ = Run(s, Query{Query: "Growl"})
	fixtures.WantAll(t, "pet abilities in recipes", fixtures.JSON(t, group(t, out, "recipes")), `"summary":"used, levelRequired 10"`)
}

func TestSearchNoMatchAndErrors(t *testing.T) {
	s := fixtures.Forever(t)
	out, err := Run(s, Query{Query: "zzzz"})
	if err != nil || out["totalHits"] != 0 || len(snapshot.AsList(out["groups"])) != 0 || out["truncated"] != false {
		t.Errorf("no hits: %v %v", out, err)
	}
	if !strings.Contains(out["note"].(string), "Nothing found") || len(out["searched"].([]string)) == 0 {
		t.Errorf("note and the list of searched sections: %v", out)
	}
	for _, empty := range []string{"", "   "} {
		if _, err := Run(s, Query{Query: empty}); err == nil {
			t.Errorf("empty text %q should give an error", empty)
		}
	}
	delete(s, "bank")
	delete(s, "vendor")
	out, _ = Run(s, Query{Query: "arrow"})
	missing := fixtures.JSON(t, out["notInSnapshot"])
	fixtures.WantAll(t, "missing sections", missing, `"bank"`, `"vendor"`)
}

func TestSearchCapsHitsPerGroup(t *testing.T) {
	s := fixtures.Forever(t)
	var items []any
	for i := 0; i < 25; i++ {
		items = append(items, map[string]any{"name": fmt.Sprintf("Rune %d", i), "count": int64(1)})
	}
	s["bags"] = []any{map[string]any{"name": "Backpack", "items": items}}
	out, _ := Run(s, Query{Query: "rune"})
	g := group(t, out, "bags")
	if g["count"] != 25 || g["shown"] != 20 || g["truncated"] != true || len(snapshot.AsList(g["hits"])) != 20 || out["truncated"] != true {
		t.Errorf("hit limit: count=%v shown=%v truncated=%v/%v", g["count"], g["shown"], g["truncated"], out["truncated"])
	}
	if out["totalHits"] != 25 {
		t.Errorf("totalHits counts all hits: %v", out["totalHits"])
	}
}

func TestWalkIsDeterministic(t *testing.T) {
	s := fixtures.Forever(t)
	a, _ := Run(s, Query{Query: "a"})
	b, _ := Run(s, Query{Query: "a"})
	if fixtures.JSON(t, a) != fixtures.JSON(t, b) {
		t.Error("the hit order must be stable")
	}
}

func TestSearchNewSections(t *testing.T) {
	s := fixtures.Forever(t)
	for query, section := range map[string]string{
		"curvewood":    "questTurnIn",
		"owl":          "stable",
		"track undead": "trainers",
	} {
		out, err := Run(s, Query{Query: query})
		if err != nil {
			t.Fatal(err)
		}
		group(t, out, section)
	}
	// The vendor Laird is only in vendors (the last one is Einris); the path carries the NPC name.
	out, _ := Run(s, Query{Query: "sharp arrow"})
	if g := group(t, out, "vendors"); !strings.Contains(firstPath(t, g), "Laird") {
		t.Errorf("hit path in vendors: %s", firstPath(t, g))
	}
	// A copy of the last visit from vendors does not duplicate the hit from vendor.
	vendor := group(t, out, "vendor")
	vendors := group(t, out, "vendors")
	if vendor["count"] != 1 || vendors["count"] != 1 {
		t.Errorf("hits vendor=%v vendors=%v", vendor["count"], vendors["count"])
	}
}
