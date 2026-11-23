// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package ammo

import (
	"path/filepath"
	"testing"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
	"wowmcp/internal/storage"
)

func setAmmo(s snapshot.Snapshot, n int64) {
	snapshot.EquippedBySlot(s.Equipment())["ammo"]["count"] = n
}

// ammoHistory records dumps every game hour (played grows) with the given arrow count in the slot; the last one is current.
func ammoHistory(t *testing.T, counts ...int64) (progress.Tracker, snapshot.Snapshot) {
	t.Helper()
	history := &storage.History{Dir: filepath.Join(t.TempDir(), "history"), Keep: 50}
	var cur snapshot.Snapshot
	for i, n := range counts {
		s := fixtures.Forever(t)
		s["dumpedAt"] = int64(fixtures.ForeverDumpedAt) + int64(i-len(counts)+1)*3600
		snapshot.AsMap(s["played"])["totalSeconds"] = int64(360000) + int64(i)*3600
		setAmmo(s, n)
		cur = s
		if i < len(counts)-1 {
			history.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": s}})
		}
	}
	return progress.Tracker{History: history}, cur
}

func TestStock(t *testing.T) {
	s := fixtures.Forever(t)
	main, others, ok := MainStock(s)
	if !ok || main.Name != "Sharp Arrow" || main.Equipped != 812 || main.InBags != 0 || len(others) != 0 {
		t.Fatalf("fixture stock: %+v %v %v", main, others, ok)
	}
	bag := snapshot.AsMap(snapshot.AsList(s["bags"])[0])
	bag["items"] = append(snapshot.AsList(bag["items"]),
		map[string]any{"name": "Sharp Arrow", "count": int64(100), "type": "Projectile", "subType": "Arrow"},
		map[string]any{"name": "Rough Bullet", "count": int64(30), "type": "Projectile", "subType": "Bullet"})
	main, others, _ = MainStock(s)
	if main.Total() != 912 || main.InBags != 100 || len(others) != 1 || others[0].Name != "Rough Bullet" {
		t.Errorf("stock from bags: %+v %+v", main, others)
	}

	// Empty slot: the most numerous ammo from bags.
	eq := snapshot.AsList(s["equipment"])
	var rest []any
	for _, raw := range eq {
		if snapshot.AsMap(raw)["slot"] != "ammo" {
			rest = append(rest, raw)
		}
	}
	s["equipment"] = rest
	main, _, ok = MainStock(s)
	if !ok || main.Name != "Sharp Arrow" || main.Equipped != 0 || main.Total() != 100 {
		t.Errorf("empty slot: %+v %v", main, ok)
	}
	for _, raw := range snapshot.AsList(s["bags"]) {
		snapshot.AsMap(raw)["items"] = []any{}
	}
	if _, _, ok := MainStock(s); ok {
		t.Error("without ammo")
	}
}

func TestViewConsumptionAndPlan(t *testing.T) {
	// 1212 -> 1012 -> 812: 400 pieces in 2 h of play, i.e. 200 per hour; 812 left = 4.1 h.
	tr, cur := ammoHistory(t, 1212, 1012, 812)
	out, err := View("Piekasz", cur, tr, Query{Hours: 5})
	if err != nil {
		t.Fatal(err)
	}
	text := fixtures.JSON(t, out)
	fixtures.WantAll(t, "ammo", text, `"total":812`, `"perHour":200`, `"used":400`, `"window":"current session"`,
		`"basis":"play time from /played"`, `"hoursLeft":4.1`, `"timeLeft":"4 h 3 min"`, `"needed":1000`, `"toBuy":188`, `"stack":200`, `"npc":"Einris Brightspear"`)

	// Default 2 hours: 400 needed, 812 on hand.
	out, _ = View("Piekasz", cur, tr, Query{})
	plan := snapshot.AsMap(out["plan"])
	if plan["toBuy"] != int64(0) || plan["note"] == nil || plan["hours"] != float64(2) {
		t.Errorf("default plan: %v", plan)
	}
}

func TestPurchasesAreNotConsumption(t *testing.T) {
	// 500 -> 900 (purchase) -> 800: only the drop of 100 counts.
	tr, cur := ammoHistory(t, 500, 900, 800)
	out, _ := View("Piekasz", cur, tr, Query{})
	if use := snapshot.AsMap(out["consumption"]); use["used"] != int64(100) {
		t.Errorf("a purchase is not consumption: %v", use)
	}
	// No drops: rate 0, no hoursLeft.
	tr, cur = ammoHistory(t, 500, 600, 700)
	out, _ = View("Piekasz", cur, tr, Query{})
	if use := snapshot.AsMap(out["consumption"]); use["perHour"] != float64(0) || use["note"] == nil || out["hoursLeft"] != nil {
		t.Errorf("no consumption: %v", out)
	}
}

func TestViewEdgeCases(t *testing.T) {
	cur := fixtures.Forever(t)
	// History disabled: the stock is there, usage is not.
	out, err := View("Piekasz", cur, progress.Tracker{}, Query{})
	if err != nil || snapshot.AsMap(out["consumption"])["note"] == nil || out["ammo"] == nil {
		t.Errorf("without history: %v %v", out, err)
	}
	// One dump: not enough data.
	tr, one := ammoHistory(t, 800)
	out, _ = View("Piekasz", one, tr, Query{})
	if snapshot.AsMap(out["consumption"])["note"] == nil {
		t.Errorf("one dump: %v", out)
	}
	if _, err := View("Piekasz", cur, tr, Query{Hours: -1}); err == nil {
		t.Error("negative hours")
	}
	noAmmo := fixtures.Forever(t)
	noAmmo["equipment"] = []any{}
	noAmmo["bags"] = []any{}
	out, _ = View("Piekasz", noAmmo, tr, Query{})
	if out["note"] == nil || out["ammo"] != nil {
		t.Errorf("without ammo: %v", out)
	}
}

func TestFallsBackToWholeHistory(t *testing.T) {
	// Two dumps with a 10 h offline gap (played grows by 1 h): the session breaks apart, leaving the whole history.
	history := &storage.History{Dir: filepath.Join(t.TempDir(), "history"), Keep: 50}
	old := fixtures.Forever(t)
	old["dumpedAt"] = int64(fixtures.ForeverDumpedAt) - 11*3600
	snapshot.AsMap(old["played"])["totalSeconds"] = int64(360000) - 3600
	setAmmo(old, 1012)
	history.Record(&snapshot.Database{Characters: map[string]snapshot.Snapshot{"Piekasz": old}})
	cur := fixtures.Forever(t)
	setAmmo(cur, 812)
	out, _ := View("Piekasz", cur, progress.Tracker{History: history}, Query{})
	use := snapshot.AsMap(out["consumption"])
	if use["window"] != "whole history" || use["perHour"] != float64(200) {
		t.Errorf("whole history: %v", use)
	}
}

func TestLeft(t *testing.T) {
	tr, cur := ammoHistory(t, 1000, 800, 600)
	f := Left("Piekasz", cur, tr)
	if !f.Present || !f.RateKnown || !f.Burning || f.PerHour != 200 {
		t.Fatalf("forecast: %+v", f)
	}
	if f.HoursLeft*200 != float64(f.Stock.Total()) {
		t.Errorf("HoursLeft = %v for stock %d", f.HoursLeft, f.Stock.Total())
	}
	if noHistory := Left("Piekasz", fixtures.Forever(t), progress.Tracker{}); !noHistory.Present || noHistory.RateKnown || noHistory.Note == "" {
		t.Errorf("without history: %+v", noHistory)
	}
	empty := fixtures.Forever(t)
	delete(empty, "equipment")
	delete(empty, "bags")
	if Left("Piekasz", empty, tr).Present {
		t.Error("character without ammo")
	}
}

func TestInspectPurchasePlanTyped(t *testing.T) {
	tr, cur := ammoHistory(t, 1212, 1012, 812)
	r, err := Inspect("Piekasz", cur, tr, Query{Hours: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Present || r.SlotEmpty || r.Use.Used != 400 || !r.HasHoursLeft || r.Plan.Needed != 1000 || r.Plan.ToBuy != 188 || r.Plan.Vendor == nil {
		t.Errorf("report: %+v", r)
	}
	if _, err := Inspect("Piekasz", cur, tr, Query{Hours: -2}); err == nil {
		t.Error("negative hours")
	}
}
