// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package xpgains

import (
	"testing"

	"wowmcp/internal/snapshot"
)

func snap(kills, quests, other [2]int64, since int64) snapshot.Snapshot {
	src := func(v [2]int64) map[string]any { return map[string]any{"count": v[0], "xp": v[1]} }
	return snapshot.Snapshot{"xpGains": map[string]any{"since": since, "kills": src(kills), "quests": src(quests), "other": src(other)}}
}

func TestSectionAbsent(t *testing.T) {
	if _, _, ok := Section(snapshot.Snapshot{}); ok {
		t.Fatal("a missing section should give ok=false")
	}
	if _, m := Gained(nil, []snapshot.Snapshot{{}, {}}); m.Found {
		t.Fatal("no section in the dumps: found=false")
	}
}

func TestSumAcrossSnapshots(t *testing.T) {
	a := snap([2]int64{2, 180}, [2]int64{1, 500}, [2]int64{0, 0}, 100)
	b := snap([2]int64{3, 270}, [2]int64{1, 650}, [2]int64{1, 40}, 200)
	g, m := Gained(nil, []snapshot.Snapshot{a, {}, b})
	if !m.Found {
		t.Fatal("found")
	}
	if g.Kills != (Source{5, 450}) || g.Quests != (Source{2, 1150}) || g.Other != (Source{1, 40}) {
		t.Fatalf("wrong sums: %+v", g)
	}
	if g.Total() != 1640 {
		t.Fatalf("total=%d", g.Total())
	}
}

func TestDescribe(t *testing.T) {
	if Describe(Gains{}) != nil {
		t.Fatal("empty XP: nil")
	}
	d := Describe(Gains{Kills: Source{2, 180}, Quests: Source{1, 500}})
	if d["total"] != int64(680) {
		t.Fatalf("total: %v", d["total"])
	}
	if _, has := d["other"]; has {
		t.Fatal("an empty source should be skipped")
	}
	if k := d["kills"].(map[string]any); k["count"] != int64(2) || k["xp"] != int64(180) {
		t.Fatalf("kills: %v", k)
	}
}

func TestEmpty(t *testing.T) {
	if !(Gains{}).Empty() || (Gains{Other: Source{Count: 1}}).Empty() {
		t.Fatal("a gain with an event is not empty")
	}
}
