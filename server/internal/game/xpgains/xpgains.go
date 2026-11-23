// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package xpgains reads the xpGains section from the dump: the character's own experience collected by the addon since the previous save
// to disk (like loot, with the periods of earlier saves it carries since addon 0.12.0), split into creature kills, turned-in quests and the rest (e.g. exploration).
// wow_changes shows it for one interval, the session journal sums it from the session's dumps.
package xpgains

import (
	"wowmcp/internal/game/periods"
	"wowmcp/internal/snapshot"
)

// Source is one source of experience: the number of events and the XP sum.
type Source struct {
	Count int64
	XP    int64
}

// Gains is experience split by source.
type Gains struct {
	Kills, Quests, Other Source
}

func (g Gains) Total() int64 { return g.Kills.XP + g.Quests.XP + g.Other.XP }

func (g Gains) Empty() bool {
	return g.Total() == 0 && g.Kills.Count+g.Quests.Count+g.Other.Count == 0
}

func (g *Gains) Add(o Gains) {
	g.Kills.add(o.Kills)
	g.Quests.add(o.Quests)
	g.Other.add(o.Other)
}

func (s *Source) add(o Source) {
	s.Count += o.Count
	s.XP += o.XP
}

// Gained sums the XP gained after baseline (nil: no baseline) from the dumps: their current periods and the periods
// carried from saves the server never read (periods.After); m.Found says whether any dump had the section.
func Gained(baseline snapshot.Snapshot, dumps []snapshot.Snapshot) (total Gains, m periods.Merge) {
	m = periods.After(baseline, dumps, "xpGains")
	for _, p := range m.Periods {
		total.Add(gainsOf(p.Raw))
	}
	return total, m
}
