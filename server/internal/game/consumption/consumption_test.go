// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package consumption

import (
	"testing"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

func stack(n int64) snapshot.Snapshot {
	return snapshot.Snapshot{"n": n}
}

func countN(s snapshot.Snapshot) int64 { n, _ := snapshot.AsInt(s["n"]); return n }

func TestMeasureSumsDropsOnly(t *testing.T) {
	// 100 -> 70 (-30), 70 -> 170 (purchase, does not count), 170 -> 150 (-20): usage 50 in 2 h.
	st := progress.Stretch{Label: "current session", Basis: "play time from /played", ElapsedSeconds: 7200,
		Snapshots: []snapshot.Snapshot{stack(100), stack(70), stack(170), stack(150)}}
	u, ok := Measure(st, countN)
	if !ok || u.Used != 50 || u.PerHour != 25 || u.Snapshots != 4 {
		t.Fatalf("usage: %+v ok=%v", u, ok)
	}
	if left, ok := u.HoursLeft(100); !ok || left != 4 {
		t.Errorf("HoursLeft = %v %v, want 4", left, ok)
	}
}

func TestMeasureNeedsEnoughData(t *testing.T) {
	short := progress.Stretch{ElapsedSeconds: 120, Snapshots: []snapshot.Snapshot{stack(10), stack(5)}}
	one := progress.Stretch{ElapsedSeconds: 7200, Snapshots: []snapshot.Snapshot{stack(10)}}
	for name, st := range map[string]progress.Stretch{"too short": short, "one dump": one} {
		if _, ok := Measure(st, countN); ok {
			t.Errorf("%s: Measure should not give a rate", name)
		}
	}
}

func TestNothingConsumed(t *testing.T) {
	st := progress.Stretch{ElapsedSeconds: 3600, Snapshots: []snapshot.Snapshot{stack(10), stack(10)}}
	u, _ := Measure(st, countN)
	if _, ok := u.HoursLeft(10); ok {
		t.Error("without usage HoursLeft makes no sense")
	}
	if got := u.Describe("how", "nothing decreased")["note"]; got != "nothing decreased" {
		t.Errorf("note = %v", got)
	}
}

func TestWindowsLoadErrorsWithoutHistory(t *testing.T) {
	if _, err := Load(progress.Tracker{}, "A", snapshot.Snapshot{"dumpedAt": int64(1)}); err == nil {
		t.Error("without history Load should return an error")
	}
}

func TestMeasureGainSumsRisesOnly(t *testing.T) {
	// 10 -> 14 (+4), 14 -> 6 (sale, does not count), 6 -> 8 (+2): gain 6 in 2 h.
	st := progress.Stretch{Label: "current session", ElapsedSeconds: 7200,
		Snapshots: []snapshot.Snapshot{stack(10), stack(14), stack(6), stack(8)}}
	u, ok := MeasureGain(st, countN)
	if !ok || u.Used != 6 || u.PerHour != 3 {
		t.Fatalf("gain: %+v ok=%v", u, ok)
	}
	if _, ok := MeasureGain(progress.Stretch{ElapsedSeconds: 60, Snapshots: []snapshot.Snapshot{stack(1), stack(2)}}, countN); ok {
		t.Error("a too-short stretch gives no rate")
	}
}
