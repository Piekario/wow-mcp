// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package rested

import (
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func TestViewOnFixture(t *testing.T) {
	out := View(fixtures.Forever(t), fixtures.At(30*time.Minute))
	text := fixtures.JSON(t, out)
	// xpMax 21300, rested 4000: 18.8% of the level, 3.8 bubbles of 20, 20100 to level-up, 2P=8000 does not cover it.
	fixtures.WantAll(t, "rested", text,
		`"source":"classic rule, approximate"`, `"restedXp":4000`, `"percentOfLevel":18.8`, `"levelsWorth":0.19`,
		`"bubbles":3.8`, `"bubblesPerLevel":20`, `"xpToNextLevel":20100`, `"coversRestOfLevel":false`, `"xpLeftAfterPool":12100`,
		`"resting":true`, `"bindLocation":"Auberdine"`, `"cap":31950`, `"missing":27950`, `"full":false`)
	inn := snapshot.AsMap(snapshot.AsMap(out["pool"])["inn"])
	// 5% of 21300 = 1065 XP per 8 h; 27950/1065*8 = 209.95 h.
	if inn["hoursToFull"] != 210.0 || inn["xpPer8h"] != int64(1065) {
		t.Errorf("inn: %v", inn)
	}
	outside := snapshot.AsMap(snapshot.AsMap(out["pool"])["outsideInn"])
	if outside["hoursToFull"] != 839.8 || outside["xpPer8h"] != int64(266) {
		t.Errorf("outside inn: %v", outside)
	}
	if inn["fullAt"] == nil || inn["assumption"] == nil {
		t.Errorf("missing forecast: %v", inn)
	}
}

func TestCoversRestOfLevel(t *testing.T) {
	s := fixtures.Forever(t)
	c := snapshot.AsMap(s["character"])
	c["xp"], c["restedXp"] = int64(20000), int64(700) // 1300 left, 2*700 = 1400 >= 1300
	out := View(s, fixtures.RealTime())
	if out["coversRestOfLevel"] != true || out["xpLeftAfterPool"] != int64(0) {
		t.Errorf("the pool should cover the rest of the level: %v", out)
	}
	c["restedXp"] = int64(650) // 1300 >= 1300: exactly enough
	if View(s, fixtures.RealTime())["coversRestOfLevel"] != true {
		t.Error("just barely enough")
	}
	c["restedXp"] = int64(649)
	if View(s, fixtures.RealTime())["coversRestOfLevel"] != false {
		t.Error("1 short")
	}
}

func TestFullPoolAndNoPool(t *testing.T) {
	s := fixtures.Forever(t)
	c := snapshot.AsMap(s["character"])
	c["restedXp"] = int64(40000) // above the limit 31950
	pool := snapshot.AsMap(View(s, fixtures.RealTime())["pool"])
	if pool["full"] != true || pool["inn"] != nil {
		t.Errorf("a full pool has no forecast: %v", pool)
	}
	c["resting"] = false
	out := View(s, fixtures.RealTime())
	if out["resting"] != false || out["restingNote"] == nil {
		t.Errorf("resting=false: %v", out)
	}

	delete(c, "restedXp")
	delete(c, "resting")
	out = View(s, fixtures.RealTime())
	if out["note"] == nil || out["restedXp"] != int64(0) || out["resting"] != nil {
		t.Errorf("missing restedXp: %v", out)
	}

	c["xpMax"] = int64(0)
	out = View(s, fixtures.RealTime())
	if out["note"] == nil || out["restedXp"] != nil || out["source"] != Source {
		t.Errorf("max level: %v", out)
	}
}
