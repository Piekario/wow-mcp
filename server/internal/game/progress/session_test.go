// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"testing"

	"wowmcp/internal/snapshot"
)

// Without /played: the gap is counted from the time between dumps.
func TestSessionStartWithoutPlayed(t *testing.T) {
	noPlayed := []progressPoint{{at: 0, played: -1}, {at: 3600, played: -1}, {at: 3600 + 4*3600, played: -1}, {at: 3600 + 5*3600, played: -1}}
	if got := sessionStart(noPlayed, 0); got != 2 {
		t.Errorf("sessionStart without /played = %d, want 2", got)
	}
}

// Splitting into sessions: the same rule as sessionStart, and the last session starts where sessionStart points.
func TestSessionSpansMatchSessionStart(t *testing.T) {
	points := []progressPoint{{at: 0, played: -1}, {at: 3600, played: -1}, {at: 3600 + 4*3600, played: -1}, {at: 3600 + 5*3600, played: -1}}
	spans := sessionSpans(points, 0)
	if len(spans) != 2 || spans[0] != (span{0, 1}) || spans[1] != (span{2, 3}) {
		t.Fatalf("sessions: %+v", spans)
	}
	if spans[len(spans)-1].from != sessionStart(points, 0) {
		t.Errorf("the last session starts at %d, sessionStart says %d", spans[len(spans)-1].from, sessionStart(points, 0))
	}
	// With /played the gap is offline time: 3 h on the clock, but the player was in game the whole time.
	played := []progressPoint{{at: 0, played: 0}, {at: 3 * 3600, played: 3*3600 - 600}}
	if got := sessionSpans(played, 0); len(got) != 1 {
		t.Errorf("sessions with /played: %+v", got)
	}
	if sessionSpans(nil, 0) != nil {
		t.Error("no dumps means no sessions")
	}
}

func TestAverageSessionHours(t *testing.T) {
	// Session 1: 2 h of play, session 2: 1 h, session 3: one dump (does not count).
	points := []progressPoint{
		{at: 0, played: 0}, {at: 7200, played: 7200},
		{at: 200000, played: 7300}, {at: 203600, played: 10900},
		{at: 500000, played: 11000},
	}
	if got := averageSessionHours(points); got != 1.5 {
		t.Errorf("average session = %v, want 1.5", got)
	}
	if averageSessionHours(points[:1]) != 0 {
		t.Error("one dump: average unknown")
	}
}

func TestRangeStatsXPSourcesSkipFirstSnapshot(t *testing.T) {
	src := func(kills, quests int64) snapshot.Snapshot {
		return snapshot.Snapshot{"xpGains": map[string]any{
			"kills":  map[string]any{"count": int64(1), "xp": kills},
			"quests": map[string]any{"count": int64(1), "xp": quests},
		}}
	}
	mk := func(at int64, s snapshot.Snapshot) progressPoint {
		return progressPoint{at: at, char: characterState{Level: 10, XP: at, HasLevel: true, HasXP: true}, played: -1, deaths: -1, source: s}
	}
	points := []progressPoint{mk(0, src(999, 999)), mk(600, src(100, 400)), mk(1200, src(50, 250))}
	r := rangeStats(points, 0, 2)
	if r.XPSources == nil || r.XPSources.Gains.Total() != 800 || r.XP != 1200 || !r.RateKnown || r.XPPerHour != 3600 {
		t.Fatalf("rate: %+v", r)
	}
	points[1].source, points[2].source = snapshot.Snapshot{}, snapshot.Snapshot{}
	if r := rangeStats(points, 0, 2); r.XPSources != nil {
		t.Error("without the xpGains section the field is omitted")
	}
}
