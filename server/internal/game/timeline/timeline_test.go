// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package timeline

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/eventlog"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

type fakeHistory map[int64]snapshot.Snapshot

func (h fakeHistory) List(string) ([]int64, error) {
	var out []int64
	for at := range h {
		out = append(out, at)
	}
	for i := range out { // simple sort
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (h fakeHistory) Load(_ string, at int64) (snapshot.Snapshot, error) {
	if s, ok := h[at]; ok {
		return s, nil
	}
	return nil, errors.New("missing")
}

const hour = int64(3600)

func snap(at int64, level, xp int64, mod func(s snapshot.Snapshot)) snapshot.Snapshot {
	s := snapshot.Snapshot{
		"dumpedAt":        at,
		"character":       map[string]any{"level": level, "xp": xp, "xpMax": int64(1000), "moneyCopper": int64(0)},
		"equipment":       []any{map[string]any{"slot": "head", "name": "Old Helm"}},
		"spells":          []any{map[string]any{"name": "Attack"}},
		"quests":          []any{map[string]any{"id": int64(1), "title": "Wolves", "zone": "Elwynn Forest"}},
		"completedQuests": []any{},
		"professions":     []any{map[string]any{"name": "Mining", "rank": int64(70), "maxRank": int64(150)}},
		"deaths":          map[string]any{"count": int64(0), "recent": []any{}},
	}
	if mod != nil {
		mod(s)
	}
	return s
}

func history(t *testing.T) (progress.Tracker, snapshot.Snapshot) {
	t.Helper()
	t0 := int64(1_000_000)
	h := fakeHistory{
		t0: snap(t0, 10, 100, nil),
		t0 + hour: snap(t0+hour, 11, 50, func(s snapshot.Snapshot) {
			s["equipment"] = []any{map[string]any{"slot": "head", "name": "New Helm"}}
			s["spells"] = []any{map[string]any{"name": "Attack"}, map[string]any{"name": "Arcane Shot", "rank": "Rank 2"}}
		}),
	}
	cur := snap(t0+2*hour, 11, 400, func(s snapshot.Snapshot) {
		s["equipment"] = []any{map[string]any{"slot": "head", "name": "New Helm"}}
		s["spells"] = []any{map[string]any{"name": "Attack"}, map[string]any{"name": "Arcane Shot", "rank": "Rank 2"}}
		s["quests"] = []any{}
		s["completedQuests"] = []any{int64(1)}
		s["professions"] = []any{map[string]any{"name": "Mining", "rank": int64(80), "maxRank": int64(150)}}
		s["deaths"] = map[string]any{"count": int64(1), "recent": []any{map[string]any{"at": t0 + 2*hour - 600, "zone": "Westfall", "level": int64(11)}}}
	})
	return progress.Tracker{History: h}, cur
}

func kinds(out map[string]any) []string {
	var ks []string
	for _, e := range out["events"].([]any) {
		ks = append(ks, e.(map[string]any)["kind"].(string))
	}
	return ks
}

func TestViewBuildsEvents(t *testing.T) {
	tr, cur := history(t)
	out, err := View("X", cur, tr, Query{})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(kinds(out), ",")
	// the death has an exact time before the dump, so it stands before the turned-in quest and the profession milestone (70 -> 80 crosses 75)
	if want := "level_up,new_spell,equipment,death,quest_turned_in,profession"; got != want {
		t.Errorf("kinds = %s, want %s", got, want)
	}
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "timeline", text, "level 10 -\\u003e 11", "Arcane Shot (Rank 2)", "Old Helm -\\u003e New Helm", "Death: Westfall", "Quest turned in: Wolves (Elwynn Forest)", "assumption")
}

func TestViewFilters(t *testing.T) {
	tr, cur := history(t)
	tests := []struct {
		name string
		q    Query
		want []string
	}{
		{"kind", Query{Kind: KindEquipment}, []string{"equipment"}},
		{"limit keeps the newest", Query{Limit: 2}, []string{"quest_turned_in", "profession"}},
		{"days: only the last half hour", Query{Days: 0.5 / 24}, []string{"death", "quest_turned_in", "profession"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := View("X", cur, tr, tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(kinds(out), ","); got != strings.Join(tc.want, ",") {
				t.Errorf("kinds = %s, want %v", got, tc.want)
			}
		})
	}
}

func TestViewErrorsAndEdges(t *testing.T) {
	tr, cur := history(t)
	for _, q := range []Query{{Kind: "x"}, {Limit: -1}, {Limit: 9999}, {Days: -1}} {
		if _, err := View("X", cur, tr, q); err == nil {
			t.Errorf("%+v: an error is expected", q)
		}
	}
	if _, err := View("X", cur, progress.Tracker{}, Query{}); err == nil {
		t.Error("without history an error is expected")
	}
	out, err := View("X", cur, progress.Tracker{History: fakeHistory{}}, Query{})
	if err != nil || out["note"] == nil {
		t.Errorf("one dump: %v %v", out, err)
	}
}

func TestProfessionMilestoneAndRealSnapshot(t *testing.T) {
	a := snap(1, 10, 0, nil)
	b := snap(2, 10, 0, func(s snapshot.Snapshot) {
		s["professions"] = []any{map[string]any{"name": "Mining", "rank": int64(76), "maxRank": int64(150)}, map[string]any{"name": "Herbalism", "rank": int64(1)}}
	})
	ev := Build([]snapshot.Snapshot{a, b})
	want := []ProfessionStep{{Name: "Mining", Before: 70, After: 76, Milestone: 75}, {Name: "Herbalism", After: 1, New: true}}
	if len(ev) != 2 || ev[0].What != want[0] || ev[1].What != want[1] || ev[0].Kind != KindProfession {
		t.Errorf("profession events: %+v", ev)
	}
	// the real dump does not break Build or View
	real := fixtures.Real(t)
	if ev := Build([]snapshot.Snapshot{real, real}); len(ev) != 0 {
		t.Errorf("identical dumps: %v", ev)
	}
	if ev := Build([]snapshot.Snapshot{{}, nil}); len(ev) != 0 {
		t.Errorf("empty dumps: %v", ev)
	}
}

type fakeLog struct {
	entries []eventlog.Entry
	err     error
}

func (f fakeLog) Events(_ string, from, to int64) ([]eventlog.Entry, error) {
	var out []eventlog.Entry
	for _, e := range f.entries {
		if e.At >= from && e.At <= to {
			out = append(out, e)
		}
	}
	return out, f.err
}

const day = 24 * hour

func logged() fakeLog {
	t0 := int64(1_000_000)
	return fakeLog{entries: []eventlog.Entry{
		{At: t0 - 10*day, Kind: eventlog.KindLevel, Level: 10},
		{At: t0 - 6*day, Kind: eventlog.KindQuestAccepted, QuestID: 7, Title: "Old Errand"},
		{At: t0 - 5*day, Kind: eventlog.KindQuestTurnedIn, QuestID: 7},
		{At: t0 - 4*day, Kind: eventlog.KindQuestTurnedIn, QuestID: 8},
		{At: t0 - 4*day, Kind: eventlog.KindMoney, Copper: 500},
		{At: t0 + 1800, Kind: eventlog.KindLevel, Level: 11},
		{At: t0 + 2*hour - 100, Kind: eventlog.KindQuestTurnedIn, QuestID: 1},
	}}
}

func TestLogGivesExactTimesWithoutDuplicates(t *testing.T) {
	tr, cur := history(t)
	tr.Log = logged()
	r, err := build("X", cur, tr, Query{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range r.Events {
		got = append(got, fmt.Sprintf("%s@%d/%v", e.Kind, e.At-1_000_000, e.Exact))
	}
	want := []string{
		"level_up@-864000/true", "quest_turned_in@-432000/true", "quest_turned_in@-345600/true",
		"level_up@1800/true", "new_spell@3600/false", "equipment@3600/false",
		"death@6600/false", "quest_turned_in@7100/true", "profession@7200/false",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("events\n got %v\nwant %v", got, want)
	}
	if r.Events[0].What != (LevelUp{9, 10}) || r.Events[1].What.(TurnedInQuest).Title != "Old Errand" || r.Events[2].What.(TurnedInQuest).Title != "quest 8" {
		t.Fatalf("log-only facts: %+v", r.Events[:3])
	}
	if q := r.Events[7].What.(TurnedInQuest); q.Title != "Wolves" || q.Zone != "Elwynn Forest" {
		t.Fatalf("the dump's quest keeps its title and zone: %+v", q)
	}
	out, _ := View("X", cur, tr, Query{Kind: KindLevelUp, Limit: 1})
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "filtered", text, `"total":2`, `"olderNotShown":1`, `"exact":true`, "Level up: level 10 -", "event log")
}

func TestLogSpanningLevelUpAndOneDump(t *testing.T) {
	a := snap(100, 10, 0, nil)
	b := snap(200, 12, 0, nil)
	ev := withLog(Build([]snapshot.Snapshot{a, b}), []eventlog.Entry{{At: 150, Kind: eventlog.KindLevel, Level: 11}})
	if len(ev) != 1 || ev[0].What != (LevelUp{10, 12}) || ev[0].At != 200 || ev[0].Exact {
		t.Fatalf("a level inside a two-level jump is not listed again: %+v", ev)
	}
	tr := progress.Tracker{History: fakeHistory{}, Log: logged()}
	r, err := build("X", snap(1_000_000+3*hour, 11, 0, nil), tr, Query{})
	if err != nil || r.SnapshotsRead != 1 || r.Total != 5 {
		t.Fatalf("one dump with a log: %+v %v", r, err)
	}
}

func TestLogErrorFallsBackToDumpTimes(t *testing.T) {
	tr, cur := history(t)
	plain, _ := View("X", cur, tr, Query{})
	tr.Log = fakeLog{err: errors.New("database is locked")}
	out, err := View("X", cur, tr, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if fixtures.JSON(t, out["events"]) != fixtures.JSON(t, plain["events"]) || !strings.Contains(fmt.Sprint(out["eventLogError"]), "database is locked") {
		t.Fatalf("fallback: %v", out)
	}
}
