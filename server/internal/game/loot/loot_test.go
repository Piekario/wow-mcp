// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package loot

import (
	"testing"

	"wowmcp/internal/snapshot"
)

func snap(items ...map[string]any) snapshot.Snapshot {
	list := make([]any, len(items))
	for i, it := range items {
		list[i] = it
	}
	return snapshot.Snapshot{"loot": map[string]any{"since": int64(100), "items": list}}
}

func TestSectionAndSum(t *testing.T) {
	a := snap(map[string]any{"name": "Linen Cloth", "id": int64(2589), "count": int64(5), "quality": int64(1)},
		map[string]any{"name": "Mystery", "count": int64(2)})
	b := snap(map[string]any{"name": "Linen Cloth", "count": int64(3), "quality": int64(1)},
		map[string]any{"name": "Wolfskin Pants", "quality": int64(3)})
	items, since, ok := Section(a)
	if !ok || since != 100 || len(items) != 2 || items[1].Quality != UnknownQuality {
		t.Fatalf("Section: %+v %d %v", items, since, ok)
	}
	if _, _, ok := Section(snapshot.Snapshot{}); ok {
		t.Error("dump without a loot section")
	}
	sum, _ := Gained(nil, []snapshot.Snapshot{a, b})
	if len(sum) != 3 || sum[0].Name != "Wolfskin Pants" || sum[1].Name != "Linen Cloth" || sum[1].Count != 8 {
		t.Errorf("Gained: %+v", sum)
	}
	if Total(sum) != 11 {
		t.Errorf("Total = %d", Total(sum))
	}
}

func TestDescribe(t *testing.T) {
	items := []Item{{Name: "Wolfskin Pants", Quality: 3, Count: 1}, {Name: "Linen Cloth", Quality: 1, Count: 8}, {Name: "Mystery", Quality: UnknownQuality, Count: 2}}
	d := Describe(items, 10)
	notable, _ := d["notable"].([]string)
	if len(notable) != 1 || notable[0] != "Wolfskin Pants x1 (Rare)" || d["unknownQuality"] != 1 || d["pieces"].(int64) != 11 {
		t.Errorf("description: %v", d)
	}
	if Describe(nil, 10) != nil {
		t.Error("without loot there is no description")
	}
	many := []Item{{Name: "A", Quality: 3, Count: 1}, {Name: "B", Quality: 3, Count: 1}, {Name: "C", Quality: 3, Count: 1}}
	if d := Describe(many, 2); d["more"] != 1 {
		t.Errorf("list truncation: %v", d)
	}
	if QualityLabel(9) != "?" || QualityLabel(4) != "Epic" {
		t.Error("QualityLabel")
	}
}

func TestSummarise(t *testing.T) {
	items := []Item{{Name: "A", Quality: 3, Count: 1}, {Name: "B", Quality: 4, Count: 2}, {Name: "C", Quality: 1, Count: 5}, {Name: "D", Quality: UnknownQuality, Count: 1}}
	sum := Summarise(items, 1)
	if sum.Distinct != 4 || sum.Pieces != 9 || len(sum.Notable) != 1 || sum.More != 1 || sum.UnknownQuality != 1 {
		t.Errorf("summary: %+v", sum)
	}
	if Summarise(nil, 5) != nil {
		t.Error("no loot, no summary")
	}
}
