// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"fmt"
	"sort"

	"wowmcp/internal/snapshot"
)

// StabledDisplay holds a stabled pet's fields as the dump has them; nil means the dump has no such field.
type StabledDisplay struct {
	Slot, Name, Level, Family, Specialization, Loyalty, Active any
}

// StabledPet is a pet in the stable. The addon saves only the slot, name, level, family and specialization,
// so there is no data on abilities or fit.
type StabledPet struct {
	Shown         StabledDisplay
	Level         int64
	HasLevel      bool
	Family        string
	WithCharacter bool // the pet with the character (slot 0 in the classic API): the same as the active one
	Active        bool // from C_StableInfo.GetActivePetList (addon 0.7.1)
}

type Stable struct {
	Pets     []StabledPet
	Slots    int64
	HasSlots bool
	Captured snapshot.Captured
}

// VsActive compares a stabled pet with the active one.
type VsActive struct {
	LevelDiff     int64
	HasLevelDiff  bool
	SameFamily    bool
	HasSameFamily bool
}

type StabledComparison struct {
	StabledPet
	VsActive VsActive
}

// StableReview compares the stabled pets with the active one (level and family).
type StableReview struct {
	Present      bool
	Pets         []StabledComparison // highest level first
	Slots        int64
	HasSlots     bool
	Captured     snapshot.Captured
	HasActive    bool
	ActiveName   any
	ActiveLevel  int64
	ActiveFamily string
	HigherLevel  []string
}

func reviewStable(stable Stable, present bool, active Pet) StableReview {
	r := StableReview{Present: present}
	if !present {
		return r
	}
	r.Slots, r.HasSlots, r.Captured = stable.Slots, stable.HasSlots, stable.Captured
	r.HasActive, r.ActiveName, r.ActiveLevel, r.ActiveFamily = active.Summoned, active.Shown.Name, active.Level, active.Family
	for _, p := range stable.Pets {
		if p.WithCharacter || (p.Active && r.HasActive && p.Shown.Name == active.Shown.Name) {
			continue
		}
		c := StabledComparison{StabledPet: p}
		if r.HasActive {
			if p.HasLevel {
				c.VsActive.LevelDiff, c.VsActive.HasLevelDiff = p.Level-active.Level, true
				if p.Level > active.Level {
					r.HigherLevel = append(r.HigherLevel, fmt.Sprint(p.Shown.Name))
				}
			}
			if p.Family != "" && active.Family != "" {
				c.VsActive.SameFamily, c.VsActive.HasSameFamily = p.Family == active.Family, true
			}
		}
		r.Pets = append(r.Pets, c)
	}
	sort.SliceStable(r.Pets, func(i, j int) bool { return r.Pets[i].Level > r.Pets[j].Level })
	return r
}

// Checkup is what the pet description adds to its state: ability ranks, the training points plan and the stable.
// Ranks and Plan are filled only for a summoned pet; Stable only when HasStable.
type Checkup struct {
	Pet       Pet
	Ranks     RankReview
	Plan      TrainingPlan
	Stable    StableReview
	HasStable bool
}

func Examine(s snapshot.Snapshot) Checkup {
	c := Checkup{Pet: readPet(s)}
	if c.Pet.Summoned {
		lessons := readLessons(s)
		c.Ranks = reviewRanks(c.Pet, lessons, readTrainerServices(s))
		c.Plan = planTraining(c.Pet, lessons)
	}
	stable, present := readStable(s)
	c.HasStable = present || c.Pet.Summoned
	if c.HasStable {
		c.Stable = reviewStable(stable, present, c.Pet)
	}
	return c
}
