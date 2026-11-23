// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal

import (
	"testing"

	"wowmcp/internal/snapshot"
)

func TestRecordsPickLongestAndBestGold(t *testing.T) {
	all := []Session{
		{Start: 1, ElapsedSeconds: 3600, MoneyDelta: 500},
		{Start: 2, ElapsedSeconds: 7200, MoneyDelta: -100},
		{Start: 3, ElapsedSeconds: 60, MoneyDelta: 9000}, // too short for a record
	}
	r := records(all, nil)
	if r.Longest == nil || r.Longest.Start != 2 || r.BestGold == nil || r.BestGold.Start != 1 || r.FastestLevel != nil {
		t.Errorf("records: %+v", r)
	}
	if !records(nil, nil).Empty() {
		t.Error("no sessions, no records")
	}
}

func TestFastestLevel(t *testing.T) {
	dump := func(at, level, total, atLevel int64) snapshot.Snapshot {
		return snapshot.Snapshot{"dumpedAt": at, "character": map[string]any{"level": level},
			"played": map[string]any{"totalSeconds": total, "levelSeconds": atLevel}}
	}
	got, ok := fastestLevel([]snapshot.Snapshot{dump(100, 19, 1000, 700), dump(200, 20, 2000, 300)})
	if !ok || got != (LevelTime{Level: 19, Seconds: 1400, ReachedAt: 200}) {
		t.Errorf("fastest level: %+v %v", got, ok)
	}
}
