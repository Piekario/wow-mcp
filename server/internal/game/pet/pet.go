// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package pet covers the character's pet (Hunter): happiness, loyalty, training points,
// diet, learned abilities and the food the character has in bags.
package pet

import "wowmcp/internal/snapshot"

// Display holds the fields the answer shows exactly as the dump has them; nil means the dump has no such field.
type Display struct {
	Name, Family, Level, Dead, DamagePercent, LoyaltyRate, Loyalty, FoodTypes any
	HappinessInfo, LoyaltyInfo, TrainingPointsInfo                            any
}

type Ability struct {
	Name, Rank string
	Control    bool
}

// TrainingPoints are the pet's points as the addon saved them; a Has flag is false for a figure it did not save.
type TrainingPoints struct {
	Total, Spent, Free          int64
	HasTotal, HasSpent, HasFree bool
}

func (t TrainingPoints) Known() bool { return t.HasTotal || t.HasSpent || t.HasFree }

type BarSlot struct {
	Name        string
	AutoCast    bool
	HasAutoCast bool
}

// Pet is the pet section. Summoned is false when the section exists but no pet was out at dump time.
type Pet struct {
	Present, Summoned bool
	Shown             Display
	Level             int64
	Family            string
	Happiness         int64
	HasHappiness      bool
	XP, XPMax         int64
	HasXP             bool
	Points            TrainingPoints
	Abilities         []Ability
	Bar               []BarSlot
}

// Inspect reads the pet section.
func Inspect(s snapshot.Snapshot) Pet { return readPet(s) }

// Exists says whether the dump has a pet that is summoned.
func Exists(s snapshot.Snapshot) bool { return readPet(s).Summoned }

// HappinessName names the classic happiness states (1-3); the addon saves the number.
func HappinessName(h int64) string {
	switch h {
	case 1:
		return "Unhappy"
	case 2:
		return "Content"
	case 3:
		return "Happy"
	}
	return ""
}

// XPPercent is the progress to the next pet level; false without both figures.
func (p Pet) XPPercent() (float64, bool) {
	if !p.HasXP || p.XPMax <= 0 {
		return 0, false
	}
	return float64(p.XP*1000/p.XPMax) / 10, true
}

// Learned are the pet's abilities without stances and commands, which are only pet controls.
func (p Pet) Learned() []Ability {
	var out []Ability
	for _, a := range p.Abilities {
		if !a.Control {
			out = append(out, a)
		}
	}
	return out
}

// FoodStack is a stack of food in the bags that the pet may eat.
type FoodStack struct {
	Name  any
	Count int64
	ID    any
}

// BagFood is the food from bags (the Food & Drink subclass; for old dumps a subtype containing "Food").
// The server has no table of which food is meat, fish etc., so matching to the diet is left to the reader.
func BagFood(s snapshot.Snapshot) []FoodStack { return readBagFood(s) }

func BagFoodCount(s snapshot.Snapshot) int64 {
	var n int64
	for _, f := range readBagFood(s) {
		n += f.Count
	}
	return n
}
