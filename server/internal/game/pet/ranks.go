// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"sort"
	"strconv"
	"strings"
)

// RankNumber extracts the rank number from the spell subtext: "Rank 3" on an English client, "Rang 3" on a German one;
// 0 when the text does not end with a number (rankless passives, "Pet Stance").
func RankNumber(rank string) int {
	fields := strings.Fields(rank)
	if len(fields) < 2 { // a bare number is not a rank text
		return 0
	}
	n, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// ranks are the highest ranks of the pet's abilities by lower-case name; an ability without a rank has 0.
func (p Pet) ranks() map[string]int {
	out := map[string]int{}
	for _, a := range p.Learned() {
		name := strings.ToLower(a.Name)
		out[name] = max(out[name], RankNumber(a.Rank))
	}
	return out
}

// Lesson is an ability from the Beast Training window: the Hunter knows it and can teach it to the pet.
type Lesson struct {
	Name          string
	Rank          int
	RankText      string
	Used          bool // the window marks it as already learned (difficulty = used)
	LevelRequired int64
	Points        int64
	HasPoints     bool
}

func (l Lesson) Label() string {
	if l.RankText != "" {
		return l.Name + " " + l.RankText
	}
	return l.Name
}

// Lessons are the Beast Training lessons the dump has; Found is false when the player never opened the window.
// FromTrainer: the window was saved as a trainer window without an NPC (how Forever 1.60.1 shows it).
type Lessons struct {
	List        []Lesson
	Found       bool
	FromTrainer bool
}

// Teachable is a lesson the Hunter knows and the pet has a lower rank of (or none at all).
type Teachable struct {
	Lesson
	PetRank    int
	PetLevelOK bool
}

// teachables returns ranks known to the Hunter that the pet does not have yet. Ranks the window marks as
// learned (used) are skipped, and whether the pet has a lower rank is decided by its ability list.
func teachables(p Pet, all []Lesson) []Teachable {
	ranks := p.ranks()
	var out []Teachable
	for _, l := range all {
		have, known := ranks[strings.ToLower(l.Name)]
		if l.Used || (known && l.Rank <= have) {
			continue
		}
		out = append(out, Teachable{Lesson: l, PetRank: have, PetLevelOK: l.LevelRequired <= p.Level})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Rank < out[j].Rank
	})
	return out
}

// TrainerService is something a remembered trainer teaches or will teach at a higher level.
type TrainerService struct {
	Trainer, Category string
	Name, Rank        string
	Cost              int64
	HasCost           bool
	LevelRequired     int64
	HasLevel          bool
}

// RankReview describes the pet's ranks: what the Hunter can teach it right away and which ranks trainers have
// that the Hunter does not know.
type RankReview struct {
	Lessons    Lessons
	Learnable  []Teachable
	NotLearned []TrainerService
}

func reviewRanks(p Pet, lessons Lessons, services []TrainerService) RankReview {
	r := RankReview{Lessons: lessons, NotLearned: unknownRanks(p, lessons.List, services)}
	if lessons.Found {
		r.Learnable = teachables(p, lessons.List)
	}
	return r
}

// unknownRanks looks among remembered trainers for services named like a pet ability (or the Beast Training window)
// with a rank higher than the pet has or the Hunter knows.
func unknownRanks(p Pet, all []Lesson, services []TrainerService) []TrainerService {
	best := p.ranks()
	names := map[string]bool{}
	for name := range best {
		names[name] = true
	}
	for _, l := range all {
		key := strings.ToLower(l.Name)
		names[key] = true
		best[key] = max(best[key], l.Rank)
	}
	var out []TrainerService
	for _, svc := range services {
		key := strings.ToLower(svc.Name)
		if names[key] && RankNumber(svc.Rank) > best[key] {
			out = append(out, svc)
		}
	}
	return out
}

// TrainingPlan checks which of the currently available lessons fit in the free training points.
// The order is a server heuristic: higher ranks of abilities the pet already has first, then new ones, in both
// groups cheapest first; the player decides how valuable an ability is.
type TrainingPlan struct {
	Free        int64
	HasFree     bool
	HasLessons  bool
	Suggested   []Teachable
	PointsLeft  int64
	DoesNotFit  []Teachable
	LevelTooLow []Teachable
	CostUnknown []Teachable
}

// NothingToLearn: the Hunter knows no lesson the pet lacks.
func (t TrainingPlan) NothingToLearn() bool {
	return len(t.Suggested) == 0 && len(t.DoesNotFit) == 0 && len(t.LevelTooLow) == 0 && len(t.CostUnknown) == 0
}

func planTraining(p Pet, lessons Lessons) TrainingPlan {
	plan := TrainingPlan{Free: p.Points.Free, HasFree: p.Points.HasFree, HasLessons: lessons.Found}
	if !lessons.Found {
		return plan
	}
	var fits []Teachable
	for _, t := range teachables(p, lessons.List) {
		switch {
		case !t.PetLevelOK:
			plan.LevelTooLow = append(plan.LevelTooLow, t)
		case !t.HasPoints:
			plan.CostUnknown = append(plan.CostUnknown, t)
		default:
			fits = append(fits, t)
		}
	}
	sort.SliceStable(fits, func(i, j int) bool {
		a, b := fits[i], fits[j]
		if (a.PetRank > 0) != (b.PetRank > 0) {
			return a.PetRank > 0
		}
		return a.Points < b.Points
	})
	plan.PointsLeft = plan.Free
	for _, t := range fits {
		if !plan.HasFree || t.Points > plan.PointsLeft {
			plan.DoesNotFit = append(plan.DoesNotFit, t)
			continue
		}
		plan.PointsLeft -= t.Points
		plan.Suggested = append(plan.Suggested, t)
	}
	return plan
}
