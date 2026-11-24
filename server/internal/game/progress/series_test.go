// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress_test

import (
	"errors"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/progress"
)

type fakeSeries struct {
	points []progress.Point
	err    error
}

func (f fakeSeries) Points(string) ([]progress.Point, error) { return f.points, f.err }

func seriesPoint(at, money int64) progress.Point {
	return progress.Point{At: int64(fixtures.ForeverDumpedAt) + at, Level: 10, XP: 0, XPMax: 1000, Money: money, Played: -1, Deaths: -1, Honor: -1, HonorableKills: -1}
}

// The long history reaches before the dump files; points at or after the oldest file are the files' own.
func TestSeriesExtendsGoldHistory(t *testing.T) {
	tracker, cur := goldTracker(t, []int64{-day, 0}, []int64{5000, 6000})
	tracker.Series = fakeSeries{points: []progress.Point{
		seriesPoint(-30*day, 100), seriesPoint(-20*day, 300),
		seriesPoint(-day, 999999), seriesPoint(0, 999999), // the same dumps as the files: the files win
	}}
	out, err := tracker.GoldHistory("Piekasz", cur, progress.GoldQuery{Days: 60})
	if err != nil {
		t.Fatal(err)
	}
	if out["snapshots"] != 4 || out["first"] != "1s 0c" || out["last"] != "60s 0c" || out["totalChangeCopper"] != int64(5900) {
		t.Errorf("gold with the long history: %v", out)
	}
	whole, err := tracker.WholeHistory("Piekasz", cur)
	if err != nil || len(whole.Snapshots) != 2 || whole.ElapsedSeconds != day {
		t.Errorf("a stretch hands out only the dumps it has: %+v %v", whole, err)
	}
}

func TestSeriesErrorFallsBackToFiles(t *testing.T) {
	tracker, cur := goldTracker(t, []int64{-day, 0}, []int64{5000, 6000})
	want, _ := tracker.GoldHistory("Piekasz", cur, progress.GoldQuery{Days: 60})
	tracker.Series = fakeSeries{points: []progress.Point{seriesPoint(-30*day, 100)}, err: errors.New("locked")}
	got, err := tracker.GoldHistory("Piekasz", cur, progress.GoldQuery{Days: 60})
	if err != nil || fixtures.JSON(t, got) != fixtures.JSON(t, want) {
		t.Errorf("a Series error must leave the file answer: %v %v", got, err)
	}
}

// Sessions from before the files are in PastSessions only; the journal's Sessions keep to the dumps.
func TestPastSessions(t *testing.T) {
	tracker, cur := goldTracker(t, []int64{-3600, 0}, []int64{5000, 6000})
	older := seriesPoint(-10*day+3600, 1600)
	older.XP = 700
	tracker.Series = fakeSeries{points: []progress.Point{seriesPoint(-10*day, 1000), older, seriesPoint(-5*day, 2000)}}
	past, err := tracker.PastSessions("Piekasz", cur)
	if err != nil || len(past) != 2 {
		t.Fatalf("past sessions: %+v %v", past, err)
	}
	if p := past[0]; p.Points != 2 || p.ElapsedSeconds != 3600 || p.XP != 700 || p.MoneyDelta != 600 {
		t.Errorf("first past session: %+v", p)
	}
	if past[1].Points != 1 {
		t.Errorf("a one-point session: %+v", past[1])
	}
	sessions, err := tracker.Sessions("Piekasz", cur, 0)
	if err != nil || len(sessions) != 1 || len(sessions[0].Snapshots) != 2 {
		t.Errorf("sessions with dumps: %+v %v", sessions, err)
	}
	if none, _ := (progress.Tracker{History: tracker.History}).PastSessions("Piekasz", cur); len(none) != 0 {
		t.Errorf("without Series there are no past sessions: %+v", none)
	}
}

// A long history point without level or gold (-1) is skipped: read as zero it would fake a level-up or an income.
func TestSeriesUnknownNumbers(t *testing.T) {
	tracker, cur := goldTracker(t, []int64{-3600, 0}, []int64{5000, 6000})
	noLevel, noMoney := seriesPoint(-3*3600, 100), seriesPoint(-2*3600, -1)
	noLevel.Level = -1
	tracker.Series = fakeSeries{points: []progress.Point{noLevel, noMoney}}
	out, err := tracker.Pace("Piekasz", cur, progress.PaceQuery{})
	if err != nil || out["snapshotsInHistory"] != 2 || out["levelUps"] != nil {
		t.Errorf("pace: %v %v", out, err)
	}
}

// XP by source cannot reach into the long history: the answer says from which dump it counts.
func TestSeriesXPSourcesCountedFrom(t *testing.T) {
	tracker, cur := goldTracker(t, []int64{-3600, 0}, []int64{5000, 6000})
	cur["xpGains"] = map[string]any{"kills": map[string]any{"count": int64(2), "xp": int64(300)}}
	older := seriesPoint(-2*3600, 4000)
	older.Level = 20
	tracker.Series = fakeSeries{points: []progress.Point{older}}
	out, err := tracker.Pace("Piekasz", cur, progress.PaceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "overall", fixtures.JSON(t, out["overall"]), `"snapshots":3`, `"countedFrom":`, `"partialNote":`)
}
