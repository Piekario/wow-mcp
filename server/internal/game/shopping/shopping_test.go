// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package shopping

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

func view(t *testing.T, s snapshot.Snapshot, tr progress.Tracker, q Query, age time.Duration) map[string]any {
	t.Helper()
	out, err := View("Piekasz", s, tr, q, fixtures.At(age))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func text(v any) string { b, _ := json.Marshal(v); return string(b) }

// history saves dumps every hour of play: arrows decrease by 200/h, used backpack slots grow by 4/h; the last one is current.
func history(t *testing.T) (progress.Tracker, snapshot.Snapshot) {
	t.Helper()
	h := &storage.History{Dir: filepath.Join(t.TempDir(), "history"), Keep: 50}
	arrows := []int64{1200, 1000, 812}
	var cur snapshot.Snapshot
	for i, n := range arrows {
		s := fixtures.Forever(t)
		s["dumpedAt"] = int64(fixtures.ForeverDumpedAt) + int64(i-len(arrows)+1)*3600
		snapshot.AsMap(s["played"])["totalSeconds"] = int64(360000) + int64(i)*3600
		snapshot.EquippedBySlot(s.Equipment())["ammo"]["count"] = n
		snapshot.AsMap(s.Bags()[0])["free"] = int64(10) - int64(i)*4
		cur = s
		if i < len(arrows)-1 {
			h.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": s}})
		}
	}
	for _, b := range cur.Bags() {
		snapshot.AsMap(b)["free"] = int64(0)
	}
	snapshot.AsMap(cur.Bags()[0])["free"] = int64(2)
	return progress.Tracker{History: h}, cur
}

func TestTrainerDedupesSkillsAcrossNpcsAndSumsCost(t *testing.T) {
	out := view(t, fixtures.Forever(t), progress.Tracker{}, Query{}, 10*time.Minute)
	tr := snapshot.AsMap(out["trainer"])
	skills := snapshot.AsList(tr["toLearn"])
	if len(skills) != 2 {
		t.Fatalf("Disengage at two trainers counts once: %v", skills)
	}
	if got := tr["totalCost"]; got != "37s 0c" { // 2100 + 1600
		t.Errorf("totalCost = %v", got)
	}
	d := snapshot.AsMap(skills[1])
	if npcs, _ := d["npcs"].([]string); len(npcs) != 2 {
		t.Errorf("Disengage should have two trainers: %v", d)
	}
	if len(snapshot.AsList(tr["offers"])) != 2 {
		t.Errorf("offers: %v", tr["offers"])
	}
}

func TestStaleTrainerOfferIsFlagged(t *testing.T) {
	s := fixtures.Forever(t)
	out := view(t, s, progress.Tracker{}, Query{}, 5*time.Hour)
	if !strings.Contains(text(out["trainer"]), `"stale":true`) {
		t.Errorf("stale offer without the flag: %s", text(out["trainer"]))
	}
}

func TestOfferMaybeOutdatedWhenLevelPassedRequirement(t *testing.T) {
	s := fixtures.Forever(t)
	snapshot.AsMap(s["character"])["level"] = int64(45) // Volley requires 40 and was unavailable when saved
	out := view(t, s, progress.Tracker{}, Query{}, 10*time.Minute)
	if !strings.Contains(text(out["trainer"]), "maybeOutdated") || !strings.Contains(text(out["trainer"]), "Volley Rank 1") {
		t.Errorf("missing outdated-offer flag: %s", text(out["trainer"]))
	}
}

func TestBudgetFollowsPriorityOrder(t *testing.T) {
	s := fixtures.Forever(t)
	snapshot.AsMap(s["character"])["moneyCopper"] = int64(3000) // enough for Disengage (2100), not for the next ones
	out := view(t, s, progress.Tracker{}, Query{}, 10*time.Minute)
	b := snapshot.AsMap(out["budget"])
	if b["canAffordAll"] != false || b["shortBy"] == nil {
		t.Fatalf("budget: %v", b)
	}
	steps := snapshot.AsList(b["steps"])
	first := snapshot.AsMap(steps[0])
	if first["what"] != "trainer skills" || first["covered"] != false {
		t.Errorf("the trainer is first and the money does not cover everything: %v", first)
	}
}

func TestAmmoAndBagsForecastFromHistory(t *testing.T) {
	tr, cur := history(t)
	out := view(t, cur, tr, Query{Hours: 4}, 10*time.Minute)
	a := snapshot.AsMap(out["ammo"])
	if a["name"] != "Sharp Arrow" || a["toBuy"] == nil {
		t.Fatalf("ammo: %v", a)
	}
	if a["toBuy"].(int64) != 0 && a["buyAt"] == nil {
		t.Errorf("purchase without a vendor: %v", a)
	}
	town := snapshot.AsMap(out["returnToTown"])
	bags := snapshot.AsMap(town["bags"])
	if bags["hoursUntilFull"] == noData || bags["gainedSlotsPerHour"] == nil {
		t.Errorf("bags: %v", bags)
	}
	if town["soonestBecause"] == nil {
		t.Errorf("nearest reason: %v", town)
	}
	if now := text(town["returnNow"]); !strings.Contains(now, "bags almost full") {
		t.Errorf("returnNow: %s", now)
	}
}

func TestNoHistoryMeansNoData(t *testing.T) {
	out := view(t, fixtures.Forever(t), progress.Tracker{}, Query{}, 10*time.Minute)
	town := snapshot.AsMap(out["returnToTown"])
	if snapshot.AsMap(town["bags"])["hoursUntilFull"] != noData {
		t.Errorf("bags without history: %v", town["bags"])
	}
	if snapshot.AsMap(town["ammo"])["hoursLeft"] != noData {
		t.Errorf("ammo without history: %v", town["ammo"])
	}
}

func TestNextTrainerSkills(t *testing.T) {
	s := fixtures.Forever(t)
	outlook := outlookTrainer(readOffers(s), 20)
	next := outlook.Next
	if next == nil || next.Level != 40 || next.LevelsAway != 20 || !strings.Contains(text(next.Skills), "Volley Rank 1") || len(next.Trainers) != 2 {
		t.Errorf("nearest skills: %+v", outlook)
	}
	if len(outlook.AvailableNow) == 0 {
		t.Error("there are skills available to buy right now")
	}
	if outlook := outlookTrainer(readOffers(snapshot.Snapshot{}), 20); !outlook.NoOffers {
		t.Errorf("without offers: %+v", outlook)
	}
}

func TestNegativeHoursRejected(t *testing.T) {
	if _, err := View("Piekasz", fixtures.Forever(t), progress.Tracker{}, Query{Hours: -1}, fixtures.At(0)); err == nil {
		t.Error("hours < 0 must be an error")
	}
}

func TestBudgetRuleOnTypedList(t *testing.T) {
	s := fixtures.Forever(t)
	snapshot.AsMap(s["character"])["moneyCopper"] = int64(3000)
	l, err := Inspect("Piekasz", s, progress.Tracker{}, Query{}, fixtures.At(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if l.Hours != defaultHours || l.Budget.CanAffordAll || !l.Budget.Over || l.Budget.Lines[0].What != "trainer skills" || l.Budget.Lines[0].Covered {
		t.Errorf("budget: %+v", l.Budget)
	}
	if l.Budget.ShortBy != l.Budget.Total-3000 {
		t.Errorf("shortBy %d for total %d", l.Budget.ShortBy, l.Budget.Total)
	}
}
