// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress_test

import (
	"path/filepath"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

func ratesHistory(t *testing.T, gaps ...int64) (progress.Tracker, snapshot.Snapshot) {
	t.Helper()
	h := &storage.History{Dir: filepath.Join(t.TempDir(), "h"), Keep: 50}
	t0 := int64(fixtures.ForeverDumpedAt)
	var cur snapshot.Snapshot
	var played int64 = 100000
	for i, g := range gaps {
		played += g
		s := fixtures.Forever(t)
		s["dumpedAt"] = t0 + int64(i)*3600
		c := snapshot.AsMap(s["character"])
		c["level"], c["xp"], c["moneyCopper"] = int64(20), int64(1000+i*5000), int64(1000+i*3000)
		s["played"] = map[string]any{"totalSeconds": played, "levelSeconds": int64(10)}
		cur = s
		if i < len(gaps)-1 {
			if err := h.Save("Piekasz", s); err != nil {
				t.Fatal(err)
			}
		}
	}
	return progress.Tracker{History: h}, cur
}

func TestRatesFromSession(t *testing.T) {
	tr, cur := ratesHistory(t, 0, 3600, 3600) // 2 h of play, +10000 XP, +6000 copper
	r, err := tr.Rates("Piekasz", cur)
	if err != nil || !r.Known {
		t.Fatalf("Rates: %+v %v", r, err)
	}
	if r.XPPerHour != 5000 || r.MoneyPerHour != 3000 || r.Window != "current session" || r.SessionHours != 2 {
		t.Errorf("rate: %+v", r)
	}
}

func TestRatesUnknownWhenTooShort(t *testing.T) {
	tr, cur := ratesHistory(t, 0, 60) // one minute of play
	r, err := tr.Rates("Piekasz", cur)
	if err != nil || r.Known {
		t.Errorf("too short for a rate: %+v %v", r, err)
	}
	if _, err := (progress.Tracker{}).Rates("Piekasz", cur); err == nil {
		t.Error("without history Rates should return an error")
	}
}

func TestStretchXPGained(t *testing.T) {
	_, cur := ratesHistory(t, 0, 3600, 3600)
	earlier := fixtures.Forever(t)
	c := snapshot.AsMap(earlier["character"])
	c["level"], c["xp"] = int64(20), int64(1000)
	st := progress.Stretch{Snapshots: []snapshot.Snapshot{earlier, cur}}
	if xp, exact := st.XPGained(); xp != 10000 || !exact {
		t.Errorf("XPGained = %d %v", xp, exact)
	}
}
