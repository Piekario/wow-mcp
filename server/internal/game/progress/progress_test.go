// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

func TestDiffSnapshots(t *testing.T) {
	d := progress.Diff(fixtures.Forever(t), fixtures.Later(t))
	js, _ := json.Marshal(d)
	text := string(js)
	for _, want := range []string{
		`"change":"+1g 0s 0c"`, // gold
		`"levelUp":true`,
		`"xpGained":20600`, // 21300-1200 + 500
		`"Death Cap"`,
		`"readyToTurnIn":[{"id":947`,
		`"turnedIn":[{"id":4740`,
		`"change":250`, // reputation
		`"Disengage"`,
		`"elapsed":"1 h"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s in %s", want, text)
		}
	}
	same := progress.Diff(fixtures.Forever(t), fixtures.Forever(t))
	if same["note"] == nil {
		t.Errorf("identical dumps should give a no-changes note: %v", same)
	}
}

func TestDiffCurrenciesAndPlayed(t *testing.T) {
	a, b := fixtures.Forever(t), fixtures.Forever(t)
	snapshot.AsMap(snapshot.AsList(b["currencies"])[0])["quantity"] = int64(1600)
	snapshot.AsMap(b["played"])["totalSeconds"] = int64(363600)
	d := progress.Diff(a, b)
	fixtures.WantAll(t, "diff", fixtures.JSON(t, d), `"change":80`, `"playedBetween":"1 h"`)
}

func TestProgress(t *testing.T) {
	dir := t.TempDir()
	history := &storage.History{Dir: filepath.Join(dir, "history"), Keep: 50}
	tracker := progress.Tracker{History: history}

	// Yesterday: one dump. Today's session: 3 dumps every game hour, level-up 20 -> 21.
	mk := func(at, played, level, xp, money int64) snapshot.Snapshot {
		s := fixtures.Forever(t)
		s["dumpedAt"] = at
		c := snapshot.AsMap(s["character"])
		c["level"], c["xp"], c["moneyCopper"] = level, xp, money
		s["played"] = map[string]any{"totalSeconds": played, "levelSeconds": int64(100)}
		return s
	}
	t0 := int64(fixtures.ForeverDumpedAt)
	snaps := []snapshot.Snapshot{
		mk(t0-86400, 300000, 19, 15000, 1000),
		mk(t0, 303600, 20, 1200, 5212),
		mk(t0+3600, 307200, 20, 11200, 15212),
		mk(t0+7200, 310800, 21, 3500, 25212),
	}
	for _, s := range snaps[:3] {
		history.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": s}})
	}
	out, err := tracker.Pace("Piekasz", snaps[3], progress.PaceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	text := fixtures.JSON(t, out)
	// Session: 10000 + (21300-11200) + 3500 = 23600 XP in 2 h of play.
	fixtures.WantAll(t, "progress", text, `"xpGained":23600`, `"xpPerHour":11800`, `"basis":"play time from /played"`,
		`"moneyPerHour":"+1g 0s 0c"`, `"levelsGained":1`, `"snapshotsInHistory":4`, `"level":22`)
	session := snapshot.AsMap(out["session"])
	if session["snapshots"] != 3 {
		t.Errorf("the session should have 3 dumps (yesterday's is another session): %v", session)
	}
	if _, ok := session["xpPerHourValue"]; ok {
		t.Error("the helper field xpPerHourValue should not reach the response")
	}
	next := snapshot.AsMap(out["nextLevel"])
	if next["etaAtSessionRate"] == nil {
		t.Errorf("no time to level-up: %v", next)
	}
	if w, _ := tracker.Pace("Piekasz", snaps[3], progress.PaceQuery{SinceHours: 3}); snapshot.AsMap(w["overall"])["snapshots"] != 3 {
		t.Errorf("sinceHours: %v", w["overall"])
	}
}

func TestDiffDeathsAndCompletedCount(t *testing.T) {
	before, after := fixtures.Forever(t), fixtures.Forever(t)
	snapshot.AsMap(before["deaths"])["count"] = int64(0)
	d := progress.Diff(before, after)
	fixtures.WantAll(t, "deaths", fixtures.JSON(t, d["deaths"]), `"new":1`, `"before":0`, `"after":1`)
	if progress.Diff(after, before)["deaths"] != nil {
		t.Error("a counter decrease is not a new death")
	}
	delete(before, "deaths")
	if progress.Diff(before, after)["deaths"] != nil {
		t.Error("without a counter in the older dump it is unknown how many deaths were added")
	}

	// An earlier dump with an empty completed-quests list also gives a change.
	before, after = fixtures.Forever(t), fixtures.Forever(t)
	before["completedQuests"] = []any{}
	q := snapshot.AsMap(progress.Diff(before, after)["quests"])
	if q["completedCountChange"] != 3 {
		t.Errorf("completedCountChange with an empty list: %v", q)
	}
	delete(before, "completedQuests")
	if q := snapshot.AsMap(progress.Diff(before, after)["quests"]); q["completedCountChange"] != nil {
		t.Errorf("without a list in the older dump there is no change: %v", q)
	}
}

func TestPaceNeverLeaksHelperFields(t *testing.T) {
	dir := t.TempDir()
	history := &storage.History{Dir: filepath.Join(dir, "history"), Keep: 50}
	tracker := progress.Tracker{History: history}
	t0 := int64(fixtures.ForeverDumpedAt)
	var last snapshot.Snapshot
	for i := int64(0); i < 3; i++ {
		s := fixtures.Forever(t)
		s["dumpedAt"] = t0 + i*3600
		snapshot.AsMap(s["character"])["xpMax"] = int64(0) // max level: no XP to the next one
		snapshot.AsMap(s["deaths"])["count"] = i
		snapshot.AsMap(s["played"])["totalSeconds"] = int64(360000) + i*3600
		last = s
		if i < 2 {
			history.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": s}})
		}
	}
	out, err := tracker.Pace("Piekasz", last, progress.PaceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	text := fixtures.JSON(t, out)
	if strings.Contains(text, "PerHourValue") || out["nextLevel"] != nil {
		t.Errorf("helper field in the response without xpMax: %s", text)
	}
	session := snapshot.AsMap(out["session"])
	if session["deaths"] != int64(2) {
		t.Errorf("deaths in the session: %v", session)
	}
}

func TestDiffShowsLootFromNewerSnapshot(t *testing.T) {
	before, after := fixtures.Forever(t), fixtures.Later(t)
	after["loot"] = map[string]any{"since": int64(fixtures.ForeverDumpedAt + 600), "items": []any{
		map[string]any{"name": "Wolfskin Pants", "count": int64(1), "quality": int64(3)},
		map[string]any{"name": "Linen Cloth", "count": int64(7), "quality": int64(1)},
	}}
	d := progress.Diff(before, after)
	text := fixtures.JSON(t, d["loot"])
	fixtures.WantAll(t, "loot", text, "Wolfskin Pants x1 (Rare)", `"pieces":8`, "partialNote", `"since"`)
	if progress.Diff(before, fixtures.Later(t))["loot"] != nil {
		t.Error("a dump without a loot section has no loot in the changes")
	}
	after["loot"] = map[string]any{"since": int64(fixtures.ForeverDumpedAt), "items": []any{}}
	if progress.Diff(before, after)["loot"] != nil {
		t.Error("empty loot is not a change")
	}
}

func TestDiffShowsXPSourcesFromNewerSnapshot(t *testing.T) {
	before, after := fixtures.Forever(t), fixtures.Later(t)
	delete(before, "xpGains")
	delete(after, "xpGains")
	if progress.Diff(before, after)["xpSources"] != nil {
		t.Error("dump without an xpGains section: field omitted")
	}
	src := func(c, xp int64) map[string]any { return map[string]any{"count": c, "xp": xp} }
	after["xpGains"] = map[string]any{"since": int64(fixtures.ForeverDumpedAt + 600), "total": int64(1370),
		"kills": src(2, 180), "quests": src(2, 1150), "other": src(1, 40)}
	text := fixtures.JSON(t, progress.Diff(before, after)["xpSources"])
	fixtures.WantAll(t, "xp", text, `"total":1370`, `"kills":{"count":2,"xp":180}`, `"quests":{"count":2,"xp":1150}`, "partialNote", `"since"`)
	after["xpGains"] = map[string]any{"since": int64(fixtures.ForeverDumpedAt), "kills": src(0, 0), "quests": src(0, 0), "other": src(0, 0)}
	if progress.Diff(before, after)["xpSources"] != nil {
		t.Error("zero XP is not a change")
	}
}

// Real history from 15.11.2026: a good dump, then a dump from exiting the game (0 gold, empty bags),
// then a good one again. An empty dump must not drive gold to zero or spoil changes and the rate.
func TestHistoryIgnoresUnloadedDump(t *testing.T) {
	history := &storage.History{Dir: filepath.Join(t.TempDir(), "history"), Keep: 50}
	tracker := progress.Tracker{History: history}
	earlier := fixtures.Real(t)
	earlier["dumpedAt"] = int64(fixtures.RealUnloadedDumpedAt - 3600)
	snapshot.AsMap(earlier["character"])["moneyCopper"] = int64(30000)
	garbage := fixtures.RealUnloaded(t)
	for _, s := range []snapshot.Snapshot{earlier, garbage} {
		if err := history.Save(fixtures.RealCharacter, s); err != nil {
			t.Fatal(err)
		}
	}
	current := fixtures.Real(t)

	gold, err := tracker.GoldHistory(fixtures.RealCharacter, current, progress.GoldQuery{Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	js := fixtures.JSON(t, gold)
	fixtures.WantAll(t, "gold", js, `"snapshots":2`, `"totalChangeCopper":6897`, `"skippedUnloadedSnapshots":1`)
	if strings.Contains(js, `"last":"0c"`) || strings.Contains(js, `"first":"0c"`) {
		t.Errorf("empty dump in the gold history: %s", js)
	}
	changes, err := tracker.Changes(fixtures.RealCharacter, current, progress.ChangesQuery{})
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "changes", fixtures.JSON(t, changes), `"change":"+68s 97c"`, `"skippedUnloadedSnapshots":1`)

	// The current dump from an empty client: a note instead of a comparison, and the rate is computed without it.
	changes, err = tracker.Changes(fixtures.RealCharacter, garbage, progress.ChangesQuery{})
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "changes from empty", fixtures.JSON(t, changes), `exiting the game`, `"skippedUnloadedSnapshots":1`)
	pace, err := tracker.Pace(fixtures.RealCharacter, garbage, progress.PaceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	fixtures.WantAll(t, "pace from empty", fixtures.JSON(t, pace), `"snapshotsInHistory":1`)
}
