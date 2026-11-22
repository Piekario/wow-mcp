// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

// realSessionStart: the history snapshot the server had before the real session of 2026-11-16 (level 22),
// some hours before the /mcp reload at fixtures.Real0110ReloadAt.
const realSessionStart = fixtures.Real0110ReloadAt - 4*3600

// realSequence rebuilds the real sequence of 2026-11-16: the server's previous history snapshot (level 22),
// then the logout dump of addon 0.11.0 (level 23, loot.items = [], since = the /mcp reload save the server never saw).
// withCarried adds what addon 0.12.0 carries in that dump: the closed session period.
func realSequence(t *testing.T, withCarried bool) (before, after snapshot.Snapshot) {
	before = fixtures.Real0110(t)
	before["dumpedAt"] = int64(realSessionStart)
	before.Character()["level"] = int64(22)
	before["loot"] = map[string]any{"since": int64(realSessionStart - 600), "items": []any{}}
	before["xpGains"] = map[string]any{"since": int64(realSessionStart - 600), "total": int64(0)}
	after = fixtures.Real0110(t)
	if withCarried {
		after.Map("loot")["periods"] = []any{map[string]any{
			"since": int64(realSessionStart), "until": int64(fixtures.Real0110ReloadAt),
			"items": []any{
				map[string]any{"id": int64(4306), "name": "Silk Cloth", "quality": int64(1), "count": int64(14)},
				map[string]any{"id": int64(2319), "name": "Medium Leather", "quality": int64(1), "count": int64(6)},
				map[string]any{"name": "Scouting Belt", "quality": int64(2), "count": int64(1)},
			},
		}}
		after.Map("xpGains")["periods"] = []any{map[string]any{
			"since": int64(realSessionStart), "until": int64(fixtures.Real0110ReloadAt), "total": int64(9800),
			"kills":  map[string]any{"count": int64(41), "xp": int64(6200)},
			"quests": map[string]any{"count": int64(4), "xp": int64(3400)},
			"other":  map[string]any{"count": int64(1), "xp": int64(200)},
		}}
	}
	return before, after
}

func TestDiffRealDoubleSaveLosesNothing(t *testing.T) {
	// Addon 0.11.0: the real logout dump starts from zero 10 s after the reload, so the session's loot is gone.
	before, after := realSequence(t, false)
	if d := Diff(before, after); d["loot"] != nil || d["xpSources"] != nil {
		t.Fatalf("addon 0.11.0 dump: loot %v, xpSources %v", d["loot"], d["xpSources"])
	}

	// Addon 0.12.0: the closed period travels in the logout dump and is merged.
	before, after = realSequence(t, true)
	d := Diff(before, after)
	loot := d["loot"].(map[string]any)
	if loot["pieces"] != int64(21) || loot["distinctItems"] != 3 || loot["periodsMerged"] != 1 {
		t.Fatalf("loot: %v", loot)
	}
	if loot["partialNote"] != nil || loot["since"] != snapshot.FormatTime(realSessionStart) {
		t.Fatalf("the merged periods cover the whole interval: %v", loot)
	}
	if notable := loot["notable"].([]string); len(notable) != 1 || notable[0] != "Scouting Belt x1 (Uncommon)" {
		t.Fatalf("notable: %v", notable)
	}
	xp := d["xpSources"].(map[string]any)
	if xp["total"] != int64(9800) || xp["periodsMerged"] != 1 || xp["partialNote"] != nil {
		t.Fatalf("xpSources: %v", xp)
	}
}

func TestDiffCarriedPeriodAlreadySeenIsNotCountedTwice(t *testing.T) {
	// The server did read the reload dump (its current period is the session) and is now asked again:
	// the period that the logout dump carries lies before that history snapshot, so nothing is merged.
	_, after := realSequence(t, true)
	reload := fixtures.Real0110(t)
	reload["dumpedAt"] = int64(fixtures.Real0110ReloadAt)
	d := Diff(reload, after)
	if d["loot"] != nil || d["xpSources"] != nil {
		t.Fatalf("already seen: loot %v, xpSources %v", d["loot"], d["xpSources"])
	}
}
