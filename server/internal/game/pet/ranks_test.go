// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"strings"
	"testing"
	"time"

	"wowmcp/internal/fixtures"
	"wowmcp/internal/snapshot"
)

func lesson1(name, rank, diff string, level int64, points any) map[string]any {
	m := map[string]any{"name": name, "rank": rank, "difficulty": diff, "levelRequired": level}
	if points != nil {
		m["trainingPoints"] = points
	}
	return m
}

// petSnap is a Forever dump with its own Beast Training lessons and free points.
func petSnap(t *testing.T, free any, lessons ...any) snapshot.Snapshot {
	s := fixtures.Forever(t)
	snapshot.AsMap(s["recipes"])["Beast Training"] = map[string]any{"name": "Beast Training", "recipes": lessons}
	if free != nil {
		snapshot.AsMap(s["pet"])["trainingPointsFree"] = free
	}
	return s
}

func TestRankNumber(t *testing.T) {
	for in, want := range map[string]int{"Rank 3": 3, " Rank 12 ": 12, "": 0, "Pet Stance": 0, "Rank X": 0, "Rang 4": 4, "Ранг 2": 2, "7": 0} {
		if got := RankNumber(in); got != want {
			t.Errorf("RankNumber(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestLearnableSkipsUsedAndKnownRanks(t *testing.T) {
	s := petSnap(t, int64(10),
		lesson1("Growl", "Rank 3", "used", 10, nil),
		lesson1("Growl", "Rank 4", "available", 12, int64(6)),
		lesson1("Claw", "Rank 2", "available", 8, int64(5)), // the pet has Claw Rank 2: skipped
		lesson1("Bite", "Rank 3", "available", 24, int64(7)))
	review := Examine(s).Ranks.render()
	learn := snapshot.AsList(review["learnable"])
	if len(learn) != 2 {
		t.Fatalf("to learn: %v", learn)
	}
	first := snapshot.AsMap(learn[0]) // alphabetically Bite, then Growl
	if first["ability"] != "Bite Rank 3" || first["petLevelOk"] != false || first["petHas"] != "none" {
		t.Errorf("Bite: %v", first)
	}
	if snapshot.AsMap(learn[1])["petHas"] != "Rank 3" {
		t.Errorf("Growl: %v", learn[1])
	}
}

func TestTrainingPlanFitsPointsUpgradesFirst(t *testing.T) {
	s := petSnap(t, int64(10),
		lesson1("Growl", "Rank 4", "available", 12, int64(8)), // upgrade, 8 pts
		lesson1("Dash", "Rank 1", "available", 10, int64(3)),  // new, 3 pts
		lesson1("Prowl", "Rank 1", "available", 10, int64(5)), // new, 5 pts: will not fit after Growl
		lesson1("Bite", "Rank 3", "available", 24, int64(7)),  // pet level too low (19)
		lesson1("Cower", "Rank 2", "available", 10, nil))      // no cost
	plan := Examine(s).Plan.render()
	suggested := snapshot.AsList(plan["suggested"])
	if len(suggested) == 0 || snapshot.AsMap(suggested[0])["ability"] != "Growl Rank 4" {
		t.Fatalf("upgrade first: %v", plan)
	}
	if plan["pointsLeftAfter"].(int64) != 2 {
		t.Errorf("points left: %v", plan["pointsLeftAfter"])
	}
	text := fixtures.JSON(t, plan)
	fixtures.WantAll(t, "plan", text, "doesNotFit", "Prowl Rank 1 (5 pts)", "petLevelTooLow", "Bite Rank 3", "costUnknown", "Cower Rank 2")
}

func TestTrainingPlanWithoutFreePointsOrWindow(t *testing.T) {
	s := petSnap(t, nil, lesson1("Dash", "Rank 1", "available", 10, int64(3)))
	if plan := Examine(s).Plan.render(); plan["note"] == nil || plan["suggested"] != nil {
		t.Errorf("without free points: %v", plan)
	}
	s = fixtures.Forever(t)
	delete(s["recipes"].(map[string]any), "Beast Training")
	if plan := Examine(s).Plan.render(); plan["learnableNote"] == nil {
		t.Errorf("without the Beast Training window: %v", plan)
	}
}

func TestNotLearnedByHunterFromTrainerOffers(t *testing.T) {
	s := petSnap(t, int64(10), lesson1("Growl", "Rank 3", "used", 10, nil))
	astaia := snapshot.AsMap(snapshot.AsMap(s["trainers"])["Astaia"])
	astaia["services"] = append(snapshot.AsList(astaia["services"]), map[string]any{
		"name": "Growl", "rank": "Rank 4", "category": "unavailable", "costCopper": int64(900), "levelRequired": int64(30)})
	review := Examine(s).Ranks.render()
	text := fixtures.JSON(t, review["notLearnedByHunter"])
	fixtures.WantAll(t, "trainers", text, "Growl Rank 4", "Astaia", "unavailable")
	if strings.Contains(text, "Disengage") {
		t.Errorf("only pet abilities: %s", text)
	}
}

func TestStableReview(t *testing.T) {
	s := fixtures.Forever(t)
	r := Review(s, fixtures.At(10*time.Minute))
	st := snapshot.AsMap(r["stable"])
	pets := snapshot.AsList(st["pets"])
	if len(pets) != 1 {
		t.Fatalf("stable: %v", st)
	}
	vs := snapshot.AsMap(snapshot.AsMap(pets[0])["vsActive"])
	if vs["levelDiff"].(int64) != -1 || vs["sameFamily"] != false || st["higherLevelThanActive"] != nil {
		t.Errorf("comparison: %v", vs)
	}
	snapshot.AsMap(snapshot.AsList(snapshot.AsMap(s["stable"])["pets"])[1])["level"] = int64(30) // [0] is the active Cat
	st = snapshot.AsMap(Review(s, fixtures.At(0))["stable"])
	if hl, _ := st["higherLevelThanActive"].([]string); len(hl) != 1 {
		t.Errorf("higher level: %v", st)
	}
}

func TestReviewWithoutPetOrStable(t *testing.T) {
	s := snapshot.Snapshot{"pet": map[string]any{"exists": false}}
	if r := Review(s, fixtures.At(0)); len(r) != 0 {
		t.Errorf("without a pet and stable: %v", r)
	}
	s["stable"] = map[string]any{"pets": []any{map[string]any{"name": "Owl", "level": int64(10)}}}
	r := Review(s, fixtures.At(0))
	if r["rankReview"] != nil || snapshot.AsMap(r["stable"])["note"] == nil {
		t.Errorf("stable only: %v", r)
	}
}

// The real 0.7.0 dump: Beast Training saved as a trainer window without an NPC (Arcane Resistance Rank 1, which the pet already has).
func TestBeastTrainingFromTrainerWindow(t *testing.T) {
	s := fixtures.Real(t)
	review := Examine(s).Ranks.render()
	if review["windowNote"] == nil || review["learnableNote"] != "the Beast Training window has no ranks higher than those the pet already has" {
		t.Errorf("ranks from the trainer window: %v", review)
	}
	plan := Examine(s).Plan.render()
	if plan["learnableNote"] != "there are no lessons the Hunter knows that the pet does not have" {
		t.Errorf("plan from the trainer window: %v", plan)
	}
}

func TestTrainingPlanTyped(t *testing.T) {
	s := petSnap(t, int64(10),
		lesson1("Growl", "Rank 4", "available", 12, int64(8)),
		lesson1("Dash", "Rank 1", "available", 10, int64(3)),
		lesson1("Prowl", "Rank 1", "available", 10, int64(5)),
		lesson1("Bite", "Rank 3", "available", 24, int64(7)),
		lesson1("Cower", "Rank 2", "available", 10, nil))
	plan := Examine(s).Plan
	if len(plan.Suggested) != 1 || plan.Suggested[0].Label() != "Growl Rank 4" || plan.PointsLeft != 2 {
		t.Errorf("suggested: %+v", plan)
	}
	if len(plan.DoesNotFit) != 2 || len(plan.LevelTooLow) != 1 || len(plan.CostUnknown) != 1 || plan.NothingToLearn() {
		t.Errorf("the rest: %+v", plan)
	}
}

func TestStableReviewTyped(t *testing.T) {
	s := fixtures.Forever(t)
	review := Examine(s).Stable
	if !review.Present || !review.HasActive || len(review.Pets) != 1 || review.Pets[0].VsActive.LevelDiff != -1 || review.Pets[0].VsActive.SameFamily {
		t.Errorf("stable: %+v", review)
	}
}
