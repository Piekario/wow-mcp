// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestCheckFoodOnFixture(t *testing.T) {
	c := CheckFood(fixtures.Forever(t))
	if !c.Known || c.CheckedWith != "itemID" || c.Feedings != 17 {
		t.Fatalf("check: %+v", c)
	}
	if len(c.Edible) != 2 || c.Edible[0].Name != "Stringy Wolf Meat" || c.Edible[0].Count != 12 {
		t.Errorf("edible: %+v", c.Edible)
	}
	if len(c.Refused) != 1 || c.Refused[0].Name != "Moonberry Juice" {
		t.Errorf("refused: %+v", c.Refused)
	}
}

func TestCheckFoodUnknownOrEmpty(t *testing.T) {
	s := fixtures.Forever(t)
	delete(snapshot.AsMap(s["pet"]), "bagFood")
	if c := CheckFood(s); c.Known || c.Feedings != 0 {
		t.Errorf("without bagFood: %+v", c)
	}
	// A client that answered but found no candidates in the bags: known, no feedings.
	snapshot.AsMap(s["pet"])["bagFood"] = map[string]any{"items": []any{}}
	c := CheckFood(s)
	if !c.Known || c.Feedings != 0 || c.Render()["note"] == nil {
		t.Errorf("empty bags: %+v %v", c, c.Render())
	}
	// An entry without a boolean verdict is not counted.
	snapshot.AsMap(s["pet"])["bagFood"] = map[string]any{"items": []any{map[string]any{"id": int64(1), "count": int64(5)}}}
	if c := CheckFood(s); len(c.Edible)+len(c.Refused) != 0 {
		t.Errorf("no verdict: %+v", c)
	}
}

func TestReviewBagFoodFallback(t *testing.T) {
	s := fixtures.Forever(t)
	fresh := snapshot.Freshness{Now: func() time.Time { return time.Unix(1794760000, 0) }}
	known := snapshot.AsMap(Review(s, fresh)["bagFood"])
	if known["feedings"] != int64(17) || known["note"] != nil {
		t.Errorf("verdict: %v", known)
	}
	delete(snapshot.AsMap(s["pet"]), "bagFood")
	fallback := snapshot.AsMap(Review(s, fresh)["bagFood"])
	if fallback["checkedByClient"] != false || fallback["note"] != FoodNote || len(snapshot.AsList(fallback["foodInBags"])) != 2 {
		t.Errorf("fallback: %v", fallback)
	}
}
