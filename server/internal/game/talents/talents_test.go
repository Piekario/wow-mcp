// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package talents

import (
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestTalentsView(t *testing.T) {
	withPoint := fixtures.Forever(t)
	snapshot.AsMap(withPoint["talents"])["unspent"] = int64(1)
	out := View(withPoint, Query{})
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "forever talents", text, `"split":"7/4/0"`, `"mainTree":"Beast Mastery"`, "Lethal Shots 4/5",
		`"talent":"Efficiency 0/5"`, "Reduces the Mana cost", "Wyvern Sting", "Counterattack", `"expectedPointsClassic":11`)
	if strings.Contains(text, "Shield Wall") || strings.Contains(text, "Hidden") {
		t.Errorf("nodes of another class or hidden ones in the tree: %s", text)
	}
	if n := len(out["canBuyNow"].([]any)); n != 2 {
		t.Errorf("canBuyNow = %d, want 2 (Efficiency, Lethal Shots)", n)
	}
	// Without free points the canPurchase flag from the game means nothing: canBuyNow is empty and there is an explanation.
	spent := View(fixtures.Forever(t), Query{})
	if n := len(spent["canBuyNow"].([]any)); n != 0 || !strings.Contains(spent["note"].(string), "unspent = 0") {
		t.Errorf("unspent=0: canBuyNow=%v note=%v", spent["canBuyNow"], spent["note"])
	}
	if out := View(fixtures.Forever(t), Query{Tree: "marks"}); len(out["fullTrees"].([]any)) != 1 {
		t.Errorf("tree filter: %v", out["fullTrees"])
	}

	// Classic trees: row 3 requires 10 points, so nothing can be bought despite a free point.
	classic := View(fixtures.Modern(t), Query{})
	fixtures.WantAll(t, "classic talents", fixtures.JSON(t, classic), `"split":"7/0/0"`, `"unspent":1`, "Improved Revive Pet")
	if len(classic["canBuyNow"].([]any)) != 0 || classic["note"] == nil {
		t.Errorf("classic canBuyNow: %v / %v", classic["canBuyNow"], classic["note"])
	}
	if !classicRowUnlocked(3, 10) || classicRowUnlocked(3, 9) || !classicRowUnlocked(1, 0) {
		t.Error("classicRowUnlocked")
	}

	// An older dump without nodes.
	old := fixtures.Forever(t)
	delete(snapshot.AsMap(old["talents"]), "nodes")
	if note, _ := View(old, Query{})["note"].(string); !strings.Contains(note, "0.4.0") {
		t.Errorf("missing note about the old addon: %q", note)
	}

	before := fixtures.Forever(t)
	snapshot.AsMap(snapshot.AsList(snapshot.AsMap(before["talents"])["points"])[2])["rank"] = int64(3)
	if got := Diff(before["talents"], fixtures.Forever(t)["talents"]); len(got) != 1 || got[0] != "Lethal Shots 3 -> 4" {
		t.Errorf("Diff = %s", got)
	}
}

// Real trees from addon 0.5.0 (pixels with outliers): each has rows 1..7 and columns 1..4.
func TestRealTreesGetGridPositions(t *testing.T) {
	out := View(fixtures.Real(t), Query{})
	trees := out["fullTrees"].([]any)
	if len(trees) != 3 {
		t.Fatalf("trees: %d", len(trees))
	}
	for _, raw := range trees {
		tree := raw.(map[string]any)
		rows, cols := map[int64]bool{}, map[int64]bool{}
		for _, n := range tree["talents"].([]any) {
			node := n.(map[string]any)
			r, _ := snapshot.AsInt(node["row"])
			c, _ := snapshot.AsInt(node["column"])
			rows[r], cols[c] = true, true
			if _, ok := node["posY"]; !ok {
				t.Errorf("%v: missing posY", node["name"])
			}
			if _, ok := node["posX"]; !ok {
				t.Errorf("%v: missing posX", node["name"])
			}
		}
		if len(rows) != 7 || len(cols) != 4 {
			t.Errorf("%v: rows %v, columns %v (want 1..7 and 1..4)", tree["tree"], rows, cols)
		}
		for r := int64(1); r <= 7; r++ {
			if !rows[r] {
				t.Errorf("%v: missing row %d", tree["tree"], r)
			}
		}
		for c := int64(1); c <= 4; c++ {
			if !cols[c] {
				t.Errorf("%v: missing column %d", tree["tree"], c)
			}
		}
	}
}

// Outliers from real data: 5740 is row 7 (next to 5730), 39300 and 102800 are tenfold values of 3930 and 10280.
func TestGridIndexesHandleOutliers(t *testing.T) {
	rows := gridIndexes([]float64{2130, 2730, 3330, 3930, 4530, 5130, 5730, 5740, 39300})
	for v, want := range map[float64]int64{2130: 1, 3930: 4, 5730: 7, 5740: 7, 39300: 4} {
		if rows[v] != want {
			t.Errorf("row %v = %d, want %d", v, rows[v], want)
		}
	}
	cols := gridIndexes([]float64{9080, 9680, 10280, 10880, 102800})
	for v, want := range map[float64]int64{9080: 1, 10280: 3, 10880: 4, 102800: 3} {
		if cols[v] != want {
			t.Errorf("column %v = %d, want %d", v, cols[v], want)
		}
	}
	if gridIndexes([]float64{1, 2, 3, 7}) != nil || gridIndexes(nil) != nil {
		t.Error("row numbers (<= 50) should not be changed")
	}
}

// Descriptions: the 0.5.0 dump had rank-0 talents without a description; the server counts them and hints what to do.
// The real 0.7.0 dump has all descriptions, so the gaps are simulated on its copy.
func TestMissingDescriptionsNote(t *testing.T) {
	real := View(fixtures.Real(t), Query{})
	if real["missingDescriptions"] != nil {
		t.Errorf("the 0.7.0 dump has all descriptions: %v", real["missingDescriptions"])
	}
	s := fixtures.Real(t)
	stripped := 0
	for _, raw := range snapshot.AsList(s.Map("talents")["nodes"]) {
		if node := snapshot.AsMap(raw); stripped < 30 {
			delete(node, "description")
			stripped++
		}
	}
	out := View(s, Query{})
	if out["missingDescriptions"] != 30 {
		t.Errorf("missingDescriptions = %v, want 30", out["missingDescriptions"])
	}
	note, _ := out["note"].(string)
	fixtures.WantAll(t, "note", note, "talent window", "/mcp reload")
}

func TestAnalyseRules(t *testing.T) {
	s := fixtures.Forever(t)
	snapshot.AsMap(s["talents"])["unspent"] = int64(1)
	r := Analyse(s, Query{})
	if r.Advice != AdviceNone || len(r.CanBuy) != 2 || r.MainTree != "Beast Mastery" || r.Split[0] != 7 {
		t.Errorf("free point: %+v", r)
	}
	if len(r.NotMaxed) == 0 {
		t.Errorf("notMaxed empty: %+v", r)
	}
	if r := Analyse(fixtures.Forever(t), Query{}); r.Advice != AdviceNoFreePoints || len(r.CanBuy) != 0 {
		t.Errorf("no free points: %+v", r)
	}
	gains := NewRanks(Sheet{Points: []Spent{{Name: "A", Rank: 1}}}, Sheet{Points: []Spent{{Name: "A", Rank: 2}, {Name: "B", Rank: 1}, {Name: "C"}}})
	if len(gains) != 2 || gains[0] != (RankGain{"A", 1, 2}) || gains[1] != (RankGain{"B", 0, 1}) {
		t.Errorf("NewRanks = %v", gains)
	}
}
