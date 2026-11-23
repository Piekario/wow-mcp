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

func (g *Gains) Add(o Gains) {
	g.Kills.add(o.Kills)
	g.Quests.add(o.Quests)
	g.Other.add(o.Other)
}

func (s *Source) add(o Source) {
	s.Count += o.Count
	s.XP += o.XP
}

func source(sec map[string]any, key string) Source {
	m := snapshot.AsMap(sec[key])
	var s Source
	s.Count, _ = snapshot.AsInt(m["count"])
	s.XP, _ = snapshot.AsInt(m["xp"])
	return s
}

// Section returns the XP from one dump's xpGains section and the time it counts from (since);
// false when the dump has no section (an older addon or the section disabled). Only the current period:
// periods carried from earlier saves are merged by Gained.
func Section(s snapshot.Snapshot) (g Gains, since int64, ok bool) {
	sec := s.Map("xpGains")
	if sec == nil {
		return Gains{}, 0, false
	}
	since, _ = snapshot.AsInt(sec["since"])
	return gainsOf(sec), since, true
}

// gainsOf reads the XP of one period (the section or a carried period).
func gainsOf(period map[string]any) Gains {
	return Gains{Kills: source(period, "kills"), Quests: source(period, "quests"), Other: source(period, "other")}
}

// Gained sums the XP gained after baseline (nil: no baseline) from the dumps: their current periods and the periods
// carried from saves the server never read (periods.After); m.Found says whether any dump had the section.
func Gained(baseline snapshot.Snapshot, dumps []snapshot.Snapshot) (total Gains, m periods.Merge) {
	m = periods.After(baseline, dumps, "xpGains")
	for _, p := range m.Periods {
		total.Add(gainsOf(p.Fields))
	}
	return total, m
}

// Describe describes the XP for the response; nil when nothing was gained (empty sources are skipped).
func Describe(g Gains) map[string]any {
	if g.Total() == 0 && g.Kills.Count+g.Quests.Count+g.Other.Count == 0 {
		return nil
	}
	out := map[string]any{"total": g.Total()}
	for _, e := range []struct {
		key string
		src Source
	}{{"kills", g.Kills}, {"quests", g.Quests}, {"other", g.Other}} {
		if e.src.Count > 0 || e.src.XP > 0 {
			out[e.key] = map[string]any{"count": e.src.Count, "xp": e.src.XP}
		}
	}
	return out
}
