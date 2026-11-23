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

// goldTracker saves into history dumps with the given gold at offset seconds from the fixture dump time.
func goldTracker(t *testing.T, offsets []int64, money []int64) (progress.Tracker, snapshot.Snapshot) {
	t.Helper()
	history := &storage.History{Dir: filepath.Join(t.TempDir(), "history"), Keep: 50}
	var last snapshot.Snapshot
	for i, off := range offsets {
		s := fixtures.Forever(t)
		s["dumpedAt"] = int64(fixtures.ForeverDumpedAt) + off
		snapshot.AsMap(s["character"])["moneyCopper"] = money[i]
		delete(s, "played") // time counted from the clock
		last = s
		if i < len(offsets)-1 {
			history.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": s}})
		}
	}
	return progress.Tracker{History: history}, last
}

const day = 86400

func TestGoldHistory(t *testing.T) {
	// Three days: -3 d (1000), -2 d morning (1000) and evening (21000), yesterday (4000), now (4500).
	tracker, cur := goldTracker(t,
		[]int64{-3 * day, -2 * day, -2*day + 3600, -day, 0},
		[]int64{1000, 1000, 21000, 4000, 4500})
	out, err := tracker.GoldHistory("Piekasz", cur, progress.GoldQuery{})
	if err != nil {
		t.Fatal(err)
	}
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "gold", text,
		`"window":"last 14 days"`, `"snapshots":5`, `"first":"10s 0c"`, `"last":"45s 0c"`,
		`"totalChange":"+35s 0c"`, `"totalChangeCopper":3500`)
	days := snapshot.AsList(out["days"])
	if len(days) != 4 {
		t.Fatalf("days: %v", days)
	}
	if d := snapshot.AsMap(days[1]); d["snapshots"] != 2 || d["delta"] != "+2g 0s 0c" || d["first"] != "10s 0c" {
		t.Errorf("second day: %v", d)
	}
	gains, losses := snapshot.AsList(out["biggestGains"]), snapshot.AsList(out["biggestLosses"])
	if len(gains) != 2 || snapshot.AsMap(gains[0])["change"] != "+2g 0s 0c" || snapshot.AsMap(gains[1])["change"] != "+5s 0c" {
		t.Errorf("gains: %v", gains)
	}
	if len(losses) != 1 || snapshot.AsMap(losses[0])["change"] != "-1g 70s 0c" {
		t.Errorf("expenses: %v", losses)
	}
}

func TestGoldHistoryWindowAndErrors(t *testing.T) {
	tracker, cur := goldTracker(t, []int64{-10 * day, -day, 0}, []int64{100, 5000, 6000})
	out, err := tracker.GoldHistory("Piekasz", cur, progress.GoldQuery{Days: 3})
	if err != nil {
		t.Fatal(err)
	}
	// The 3-day window cuts off the dump from 10 days ago.
	if out["snapshots"] != 2 || out["window"] != "last 3 days" || out["totalChangeCopper"] != int64(1000) {
		t.Errorf("3-day window: %v", out)
	}
	tracker, cur = goldTracker(t, []int64{-10 * day, 0}, []int64{100, 6000})
	one, _ := tracker.GoldHistory("Piekasz", cur, progress.GoldQuery{Days: 3})
	if one["snapshots"] != 1 || one["note"] == nil || one["days"] != nil {
		t.Errorf("one dump in the window: %v", one)
	}
	if _, err := tracker.GoldHistory("Piekasz", cur, progress.GoldQuery{Days: -2}); err == nil {
		t.Error("negative days should give an error")
	}
	if _, err := (progress.Tracker{}).GoldHistory("Piekasz", cur, progress.GoldQuery{}); err == nil {
		t.Error("disabled history should give an error")
	}
	noTime := snapshot.Snapshot{}
	if _, err := tracker.GoldHistory("Nobody", noTime, progress.GoldQuery{}); err == nil {
		t.Error("a dump without a time and without history should give an error")
	}
}

func TestStretches(t *testing.T) {
	tracker, cur := goldTracker(t, []int64{-5 * day, -3600, 0}, []int64{1, 2, 3})
	// Without /played the clock time counts.
	whole, err := tracker.WholeHistory("Piekasz", cur)
	if err != nil || len(whole.Snapshots) != 3 || whole.ElapsedSeconds != 5*day || whole.Label != "whole history" {
		t.Errorf("whole history: %+v %v", whole, err)
	}
	session, err := tracker.CurrentSession("Piekasz", cur)
	if err != nil || len(session.Snapshots) != 2 || session.ElapsedSeconds != 3600 || session.Hours() != 1 {
		t.Errorf("session: %+v %v", session, err)
	}
	if _, err := (progress.Tracker{}).CurrentSession("Piekasz", cur); err == nil {
		t.Error("without history there should be an error")
	}
	if _, err := tracker.WholeHistory("Nobody", snapshot.Snapshot{}); err == nil {
		t.Error("a dump without a time and without history should give an error")
	}
}

func TestCompareTypedChanges(t *testing.T) {
	dump := func(level, xp, money int64, spells ...string) snapshot.Snapshot {
		list := []any{}
		for _, s := range spells {
			list = append(list, map[string]any{"name": s})
		}
		return snapshot.Snapshot{"character": map[string]any{"level": level, "xp": xp, "xpMax": int64(1000), "moneyCopper": money}, "spells": list}
	}
	c := progress.Compare(dump(10, 900, 100, "Attack"), dump(11, 50, 40, "Attack", "Arcane Shot"))
	if c.XP == nil || !c.XP.LevelUp() || !c.XP.GainedKnown || c.XP.Gained != 150 {
		t.Errorf("xp: %+v", c.XP)
	}
	if c.Money == nil || c.Money.Delta() != -60 || len(c.NewSpells) != 1 || c.NewSpells[0].Name != "Arcane Shot" || !c.HasChanges() {
		t.Errorf("comparison: %+v", c)
	}
	if skip := progress.Compare(dump(10, 0, 0), dump(12, 0, 0)); skip.XP.GainedKnown {
		t.Error("two levels at once: XP unknown")
	}
	if progress.Compare(dump(10, 5, 5), dump(10, 5, 5)).HasChanges() {
		t.Error("identical dumps")
	}
}
