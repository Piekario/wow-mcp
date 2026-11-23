// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package periods

import (
	"testing"

	"wowmcp/internal/snapshot"
)

func period(since, until int64, item string) map[string]any {
	return map[string]any{"since": since, "until": until, "items": []any{map[string]any{"name": item, "count": int64(1)}}}
}

func dump(at, since int64, carried ...any) snapshot.Snapshot {
	sec := map[string]any{"since": since, "items": []any{}}
	if carried != nil {
		sec["periods"] = carried
	}
	return snapshot.Snapshot{"dumpedAt": at, "loot": sec}
}

func spans(m Merge) [][3]int64 {
	out := make([][3]int64, len(m.Periods))
	for i, p := range m.Periods {
		carried := int64(0)
		if p.Carried {
			carried = 1
		}
		out[i] = [3]int64{p.Since, p.Until, carried}
	}
	return out
}

func TestAfterOldDumpsGiveCurrentPeriodsOnly(t *testing.T) {
	m := After(dump(100, 50), []snapshot.Snapshot{dump(200, 100), {}, dump(300, 200)}, "loot")
	if !m.Found || m.Carried != 0 || len(m.Periods) != 2 || m.Earliest() != 100 {
		t.Fatalf("merge: %+v", spans(m))
	}
	if m := After(nil, []snapshot.Snapshot{{}}, "loot"); m.Found || len(m.Periods) != 0 {
		t.Fatalf("no section: %+v", m)
	}
}

func TestAfterMergesCarriedPeriodsAfterTheBaseline(t *testing.T) {
	baseline := dump(1000, 900, period(500, 900, "Old"))
	after := dump(1300, 1200,
		period(500, 900, "Old"),     // already before the baseline (the baseline carried it): not counted
		period(900, 1000, "Base"),   // the baseline's own current period, closed later: before the baseline
		period(1000, 1100, "First"), // never read by the server
		period(1100, 1200, "Second"),
		map[string]any{"since": int64(1150)}, // broken entry: no until
		"garbage")
	m := After(baseline, []snapshot.Snapshot{after}, "loot")
	want := [][3]int64{{1000, 1100, 1}, {1100, 1200, 1}, {1200, 1300, 0}}
	if got := spans(m); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("periods = %v, want %v", got, want)
	}
	if m.Carried != 2 || m.Earliest() != 1000 {
		t.Fatalf("carried %d, earliest %d", m.Carried, m.Earliest())
	}
	out := map[string]any{}
	m.Describe(out)
	if out["periodsMerged"] != 2 || out["periodsNote"] == nil {
		t.Fatalf("describe: %v", out)
	}
}

func TestAfterDeduplicatesAcrossHistorySnapshots(t *testing.T) {
	baseline := dump(1000, 900)
	// The server read the dump at 1100 (its current period 1000-1100), then two saves it never read (1200, 1300).
	seen := dump(1100, 1000)
	later := dump(1400, 1300, period(1000, 1100, "First"), period(1100, 1200, "Second"), period(1200, 1300, "Third"))
	newest := dump(1500, 1400, period(1000, 1100, "First"), period(1100, 1200, "Second"), period(1200, 1300, "Third"))
	m := After(baseline, []snapshot.Snapshot{seen, later, newest}, "loot")
	// 1000 is the current period of the seen dump; 1300 and 1400 are the current periods of the later ones.
	want := [][3]int64{{1000, 1100, 0}, {1100, 1200, 1}, {1200, 1300, 1}, {1300, 1400, 0}, {1400, 1500, 0}}
	got := spans(m)
	if len(got) != len(want) {
		t.Fatalf("periods = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("periods = %v, want %v", got, want)
		}
	}
	if m.Carried != 2 {
		t.Fatalf("carried = %d", m.Carried)
	}
}

func TestAfterKeepsTheLatestUntilOfAContinuedPeriod(t *testing.T) {
	// After a skipped exit dump the period continued: two dumps carry the same period with different ends.
	a := dump(2000, 1900, period(1000, 1500, "Short"))
	b := dump(2100, 2000, period(1000, 1800, "Long"))
	m := After(dump(900, 800), []snapshot.Snapshot{a, b}, "loot")
	if m.Carried != 1 || m.Periods[0].Until != 1800 {
		t.Fatalf("periods = %v", spans(m))
	}
	if m := (Merge{}); m.Earliest() != 0 {
		t.Fatal("empty merge has no earliest")
	}
	out := map[string]any{}
	Merge{}.Describe(out)
	if len(out) != 0 {
		t.Fatalf("nothing carried, nothing described: %v", out)
	}
}

func TestReadClosedSkipsBrokenEntries(t *testing.T) {
	d := dump(100, 50, period(10, 20, "ok"), period(30, 20, "backwards"), "garbage")
	if got := readClosed(d, "loot"); len(got) != 1 || got[0].Since != 10 || !got[0].Carried {
		t.Fatalf("closed: %+v", got)
	}
}
