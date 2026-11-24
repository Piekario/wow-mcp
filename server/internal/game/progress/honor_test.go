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

func withPvP(t *testing.T, at, honor, kills int64) snapshot.Snapshot {
	t.Helper()
	s := fixtures.Forever(t)
	s["dumpedAt"] = at
	delete(s, "played") // sessions are cut by the clock
	pvp := snapshot.AsMap(s["pvp"])
	pvp["honor"], pvp["honorableKills"] = honor, kills
	return s
}

func TestDiffHonor(t *testing.T) {
	t0 := int64(fixtures.ForeverDumpedAt)
	d := progress.Diff(withPvP(t, t0, 340, 1000), withPvP(t, t0+60, 420, 1003))
	fixtures.WantAll(t, "honor", fixtures.JSON(t, d), `"honor":{"after":420,"before":340,"change":80}`, `"honorableKills":{"after":1003,"before":1000,"new":3}`)
	same := progress.Diff(withPvP(t, t0, 340, 1000), withPvP(t, t0+60, 340, 1000))
	if same["honor"] != nil || same["honorableKills"] != nil {
		t.Errorf("unchanged pvp numbers are not a change: %v", same)
	}
	older := withPvP(t, t0, 340, 1000)
	delete(older, "pvp")
	if got := progress.Diff(older, withPvP(t, t0+60, 420, 1003)); got["honor"] != nil || got["honorableKills"] != nil {
		t.Errorf("without a pvp section in one dump there is no change: %v", got)
	}
}

func TestPaceCountsHonorOnlyWithPvPSection(t *testing.T) {
	history := &storage.History{Dir: filepath.Join(t.TempDir(), "history"), Keep: 50}
	tracker := progress.Tracker{History: history}
	t0 := int64(fixtures.ForeverDumpedAt)
	// The second step is an honor level-up: honor restarts, which adds nothing for that step.
	snaps := []snapshot.Snapshot{withPvP(t, t0, 100, 10), withPvP(t, t0+3600, 400, 14), withPvP(t, t0+7200, 50, 20)}
	for _, s := range snaps[:2] {
		history.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": s}})
	}
	out, err := tracker.Pace("Piekasz", snaps[2], progress.PaceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	session := snapshot.AsMap(out["session"])
	if session["honorGained"] != int64(300) || session["honorableKills"] != int64(10) {
		t.Errorf("session pvp: %v", session)
	}

	plain := &storage.History{Dir: filepath.Join(t.TempDir(), "history"), Keep: 50}
	for _, s := range snaps[:2] {
		delete(s, "pvp")
		plain.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": s}})
	}
	cur := snaps[2]
	delete(cur, "pvp")
	out, _ = (progress.Tracker{History: plain}).Pace("Piekasz", cur, progress.PaceQuery{})
	if s := snapshot.AsMap(out["session"]); s["honorGained"] != nil || s["honorableKills"] != nil {
		t.Errorf("no pvp section, no pvp numbers: %v", s)
	}
}
