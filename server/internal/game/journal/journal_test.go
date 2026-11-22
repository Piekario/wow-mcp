// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal_test

import (
	"path/filepath"
	"strings"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/journal"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

const t0 = int64(fixtures.ForeverDumpedAt)

// step describes one dump to build from the Forever fixture.
type step struct {
	at, played, levelSecs int64
	level, xp, money      int64
	zone                  string
	deaths                int64
	completed             []int64
	rareItem              string
	extraQuest            *map[string]any
	loot                  []any // entries of the dump's loot section
	xpKills, xpQuests     int64 // XP from kills and quests in the xpGains section (both 0 = no section)
}

func build(t *testing.T, st step) snapshot.Snapshot {
	t.Helper()
	s := fixtures.Forever(t)
	s["dumpedAt"] = st.at
	c := snapshot.AsMap(s["character"])
	c["level"], c["xp"], c["moneyCopper"], c["zone"] = st.level, st.xp, st.money, st.zone
	s["played"] = map[string]any{"totalSeconds": st.played, "levelSeconds": st.levelSecs}
	s["deaths"] = map[string]any{"count": st.deaths, "recent": []any{}}
	if st.deaths > 0 {
		s["deaths"] = map[string]any{"count": st.deaths, "recent": []any{map[string]any{"zone": "Wetlands", "at": st.at - 60}}}
	}
	done := []any{}
	for _, id := range st.completed {
		done = append(done, id)
	}
	s["completedQuests"] = done
	if st.rareItem != "" {
		bag := snapshot.AsMap(snapshot.AsList(s["bags"])[0])
		bag["items"] = append(snapshot.AsList(bag["items"]), map[string]any{"name": st.rareItem, "id": int64(777), "count": int64(1), "quality": int64(3)})
	}
	if st.loot != nil {
		s["loot"] = map[string]any{"since": st.at - 3600, "items": st.loot}
	}
	delete(s, "xpGains") // the fixture may have it; the test sets its own
	if st.xpKills+st.xpQuests > 0 {
		s["xpGains"] = map[string]any{"since": st.at - 3600,
			"kills":  map[string]any{"count": int64(2), "xp": st.xpKills},
			"quests": map[string]any{"count": int64(1), "xp": st.xpQuests}}
	}
	return s
}

// history saves dumps into history (except the last) and returns the tracker and the current dump.
func history(t *testing.T, steps ...step) (progress.Tracker, snapshot.Snapshot) {
	t.Helper()
	h := &storage.History{Dir: filepath.Join(t.TempDir(), "h"), Keep: 50}
	var cur snapshot.Snapshot
	for i, st := range steps {
		cur = build(t, st)
		if i < len(steps)-1 {
			if err := h.Save("Piekasz", cur); err != nil {
				t.Fatal(err)
			}
		}
	}
	return progress.Tracker{History: h}, cur
}

// Three sessions: from two days ago (one hour of play), yesterday (two hours and a level-up) and today's single dump.
func threeSessions(t *testing.T) (progress.Tracker, snapshot.Snapshot) {
	day := int64(86400)
	return history(t,
		step{at: t0 - 2*day, played: 100000, levelSecs: 3000, level: 19, xp: 1000, money: 1000, zone: "Darkshore"},
		step{at: t0 - 2*day + 3600, played: 103600, levelSecs: 6600, level: 19, xp: 9000, money: 21000, zone: "Ashenvale", completed: []int64{4740}},
		step{at: t0 - day, played: 104000, levelSecs: 7000, level: 19, xp: 9500, money: 21000, zone: "Ashenvale", completed: []int64{4740}},
		step{at: t0 - day + 3600, played: 107600, levelSecs: 600, level: 20, xp: 600, money: 31000, zone: "Wetlands", deaths: 2, completed: []int64{4740, 947}, rareItem: "Blue Sword"},
		step{at: t0 - day + 7200, played: 111200, levelSecs: 4200, level: 20, xp: 9000, money: 30000, zone: "Wetlands", deaths: 2, completed: []int64{4740, 947}},
		step{at: t0, played: 111300, levelSecs: 4300, level: 20, xp: 9100, money: 30000, zone: "Wetlands", deaths: 2, completed: []int64{4740, 947}},
	)
}

func TestSessionsDescribed(t *testing.T) {
	tr, cur := threeSessions(t)
	out, err := journal.View("Piekasz", cur, tr, journal.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if out["totalSessions"] != 3 {
		t.Fatalf("sessions: %v", out["totalSessions"])
	}
	sessions := out["sessions"].([]any)
	if newest := sessions[0].(map[string]any); newest["snapshots"] != 1 || newest["note"] == nil {
		t.Errorf("newest (one dump): %v", newest)
	}
	middle := sessions[1].(map[string]any)
	fixtures.WantAll(t, "middle", fixtures.JSON(t, middle),
		`"snapshots":3`, `"duration":"2 h"`, `"levelsGained":1`, `"goldChange":"+90s 0c"`, "Blue Sword x1 (Rare)",
		`"deaths":{"count":2,"zones":["Wetlands"]}`, `"zones":["Ashenvale","Wetlands"]`)
	if !strings.Contains(fixtures.JSON(t, middle["questsCompleted"]), `"count":1`) {
		t.Errorf("completed quests: %v", middle["questsCompleted"])
	}
	oldest := sessions[2].(map[string]any)
	if oldest["xpGained"] != int64(8000) || oldest["goldChange"] != "+2g 0s 0c" || oldest["duration"] != "1 h" {
		t.Errorf("oldest: %v", oldest)
	}
}

func TestSessionLimit(t *testing.T) {
	tr, cur := threeSessions(t)
	out, err := journal.View("Piekasz", cur, tr, journal.Query{Sessions: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(out["sessions"].([]any)) != 1 || out["olderSessionsNotShown"] != 2 {
		t.Errorf("sessions=1: %v", out)
	}
	for _, bad := range []int{-1, 51} {
		if _, err := journal.View("Piekasz", cur, tr, journal.Query{Sessions: bad}); err == nil {
			t.Errorf("sessions=%d should be an error", bad)
		}
	}
	if _, err := journal.View("Piekasz", cur, progress.Tracker{}, journal.Query{}); err == nil {
		t.Error("without history there should be an error")
	}
}

func TestRecords(t *testing.T) {
	tr, cur := threeSessions(t)
	out, _ := journal.View("Piekasz", cur, tr, journal.Query{})
	rec := out["records"].(map[string]any)
	if longest := rec["longestSession"].(map[string]any); longest["duration"] != "2 h" {
		t.Errorf("longestSession: %v", longest)
	}
	best := rec["bestGoldSession"].(map[string]any)
	if best["goldChange"] != "+2g 0s 0c" {
		t.Errorf("bestGoldSession: %v", best)
	}
	// Level-up 19 -> 20: level 19 lasted 7000 + (107600-104000) - 600 = 10000 s.
	fast := rec["fastestLevel"].(map[string]any)
	if fast["level"] != int64(19) || fast["playTime"] != "2 h 46 min" {
		t.Errorf("fastestLevel: %v", fast)
	}
}

func TestMarkdown(t *testing.T) {
	tr, cur := threeSessions(t)
	out, _ := journal.View("Piekasz", cur, tr, journal.Query{Markdown: true, Sessions: 2})
	text, _ := out["text"].(string)
	fixtures.WantAll(t, "markdown", text, "# Journal: Piekasz", "## ", "- Level: 19 -> 20", "- Deaths: 2 (Wetlands)", "Blue Sword x1 (Rare)", "## Records")
	if plain, _ := journal.View("Piekasz", cur, tr, journal.Query{}); plain["text"] != nil {
		t.Error("without markdown=true there is no text field")
	}
}

func TestSingleSnapshotSession(t *testing.T) {
	tr, cur := history(t, step{at: t0, played: 100, levelSecs: 10, level: 20, xp: 1, money: 5, zone: "Darkshore"})
	out, err := journal.View("Piekasz", cur, tr, journal.Query{})
	if err != nil {
		t.Fatal(err)
	}
	s := out["sessions"].([]any)[0].(map[string]any)
	if s["note"] == nil || out["records"] != nil {
		t.Errorf("single dump: %v", out)
	}
}

func TestLootInSession(t *testing.T) {
	day := int64(86400)
	tr, cur := history(t,
		// The first dump's loot predates the session and does not count.
		step{at: t0 - day, played: 100000, level: 19, xp: 1000, money: 1000, zone: "Darkshore",
			loot: []any{map[string]any{"name": "Old Rare", "count": int64(1), "quality": int64(3)}}},
		step{at: t0 - day + 3600, played: 103600, level: 19, xp: 2000, money: 2000, zone: "Darkshore",
			loot: []any{map[string]any{"name": "Wolfskin Pants", "count": int64(1), "quality": int64(3)}, map[string]any{"name": "Linen Cloth", "count": int64(5), "quality": int64(1)}}},
		step{at: t0 - day + 7200, played: 107200, level: 19, xp: 3000, money: 3000, zone: "Darkshore",
			loot: []any{map[string]any{"name": "Linen Cloth", "count": int64(3), "quality": int64(1)}, map[string]any{"name": "Mystery", "count": int64(1)}}},
	)
	out, err := journal.View("Piekasz", cur, tr, journal.Query{Markdown: true})
	if err != nil {
		t.Fatal(err)
	}
	session := out["sessions"].([]any)[0].(map[string]any)
	text := fixtures.JSON(t, session["loot"])
	fixtures.WantAll(t, "session loot", text, `"pieces":10`, "Wolfskin Pants x1 (Rare)", `"unknownQuality":1`)
	if strings.Contains(text, "Old Rare") {
		t.Errorf("loot from before the session: %s", text)
	}
	if !strings.Contains(out["text"].(string), "- Loot: Wolfskin Pants x1 (Rare)") {
		t.Errorf("markdown: %v", out["text"])
	}
}

func TestXPSourcesInSession(t *testing.T) {
	day := int64(86400)
	tr, cur := history(t,
		// The first dump's XP predates the session and does not count.
		step{at: t0 - day, played: 100000, level: 19, xp: 1000, money: 1000, zone: "Darkshore", xpKills: 999, xpQuests: 999},
		step{at: t0 - day + 3600, played: 103600, level: 19, xp: 2000, money: 2000, zone: "Darkshore", xpKills: 100, xpQuests: 400},
		step{at: t0 - day + 7200, played: 107200, level: 19, xp: 3000, money: 3000, zone: "Darkshore", xpKills: 50, xpQuests: 250},
	)
	out, err := journal.View("Piekasz", cur, tr, journal.Query{})
	if err != nil {
		t.Fatal(err)
	}
	session := out["sessions"].([]any)[0].(map[string]any)
	fixtures.WantAll(t, "session xp", fixtures.JSON(t, session["xpSources"]), `"total":800`, `"kills":{"count":4,"xp":150}`, `"quests":{"count":2,"xp":650}`)

	tr, cur = history(t,
		step{at: t0 - day, played: 100000, level: 19, xp: 1000, money: 1000, zone: "Darkshore"},
		step{at: t0 - day + 3600, played: 103600, level: 19, xp: 2000, money: 2000, zone: "Darkshore"},
	)
	out, _ = journal.View("Piekasz", cur, tr, journal.Query{})
	if out["sessions"].([]any)[0].(map[string]any)["xpSources"] != nil {
		t.Error("without the xpGains section the field is omitted")
	}
}

// The real sequence of 2026-11-16 in a session: the server read the session start, then (in a later call) the reload
// dump, and the logout dump carries the reload's period as well. The session counts it once and says nothing was merged;
// without the reload dump in the history the carried period fills the gap and the session says one period was merged.
func TestSessionMergesCarriedPeriodsOnce(t *testing.T) {
	const start, reloadAt, logoutAt = fixtures.Real0110ReloadAt - 3600, fixtures.Real0110ReloadAt, fixtures.Real0110DumpedAt
	silk := []any{map[string]any{"name": "Silk Cloth", "quality": int64(1), "count": int64(14)}}
	dumpAt := func(at, since int64, items []any, carried ...any) snapshot.Snapshot {
		s := fixtures.Real0110(t)
		s["dumpedAt"] = at
		s["loot"] = map[string]any{"since": since, "items": items, "periods": carried}
		s["xpGains"] = map[string]any{"since": since, "kills": map[string]any{"count": int64(0), "xp": int64(0)}}
		return s
	}
	first := dumpAt(start, start-600, []any{})
	reload := dumpAt(reloadAt, start, silk)
	closed := map[string]any{"since": int64(start), "until": int64(reloadAt), "items": silk}
	logout := dumpAt(logoutAt, reloadAt, []any{}, closed)

	seen := journal.Describe(progress.Stretch{Snapshots: []snapshot.Snapshot{first, reload, logout}})
	if len(seen.Loot) != 1 || seen.Loot[0].Count != 14 || seen.LootPeriods.Carried != 0 {
		t.Fatalf("reload dump in the history: loot %+v, carried %d", seen.Loot, seen.LootPeriods.Carried)
	}
	unseen := journal.Describe(progress.Stretch{Snapshots: []snapshot.Snapshot{first, logout}})
	if len(unseen.Loot) != 1 || unseen.Loot[0].Count != 14 || unseen.LootPeriods.Carried != 1 {
		t.Fatalf("reload dump never read: loot %+v, carried %d", unseen.Loot, unseen.LootPeriods.Carried)
	}
	view := unseen.View()
	if l := view["loot"].(map[string]any); l["periodsMerged"] != 1 {
		t.Fatalf("view: %v", l)
	}
}
