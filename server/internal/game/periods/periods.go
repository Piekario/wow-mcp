// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package periods merges the periods of the sections the addon collects continuously (loot, xpGains).
//
// Each such section holds the current period: what came in since the previous save to disk (since) up to the dump
// (its dumpedAt). The game may save twice before the server reads the file (/mcp reload, then logout), and the second
// save starts from zero. Since addon 0.12.0 a login that closes a period with content appends it, with until = the
// dumpedAt of the save that closed it, to the section's "periods" list, and the list is carried on (bounded by
// the addon). After merges the current periods of the given dumps with the carried periods that started at or after
// the baseline dump, so nothing that reached the disk is lost and nothing is counted twice. Dumps without the list
// (older addons) give exactly their current periods, as before.
package periods

import (
	"sort"

	"wowmcp/internal/snapshot"
)

// Period is one interval of a continuously collected section.
type Period struct {
	Since, Until int64
	Carried      bool           // a closed period from the section's periods list (false: a dump's current period)
	Fields       map[string]any // the section fields of the period (items, kills, quests...)
}

// Merge is the result of After.
type Merge struct {
	Periods []Period // in the order of Since
	Carried int      // how many carried (closed) periods were merged
	Found   bool     // whether any of the dumps had the section
}

// Earliest is the Since of the earliest merged period (0 when there is none or it is unknown).
func (m Merge) Earliest() int64 {
	var earliest int64
	for _, p := range m.Periods {
		if p.Since > 0 && (earliest == 0 || p.Since < earliest) {
			earliest = p.Since
		}
	}
	return earliest
}

// After returns the periods of section gained after baseline (a dump that is already accounted for; nil when there
// is none): the current period of every dump (as before addon 0.12.0) and the carried periods that started at or after
// the baseline's dumpedAt. A carried period is the same period as a current one with the same since (that current
// period was later closed), so it is skipped; carried periods are de-duplicated by since, keeping the latest until
// (the same period seen in several dumps).
func After(baseline snapshot.Snapshot, dumps []snapshot.Snapshot, section string) Merge {
	var m Merge
	from, _ := baseline.DumpedAt()
	counted := map[int64]bool{}
	for _, d := range dumps {
		sec := d.Map(section)
		if sec == nil {
			continue
		}
		m.Found = true
		since, _ := snapshot.AsInt(sec["since"])
		until, _ := d.DumpedAt()
		m.Periods = append(m.Periods, Period{Since: since, Until: until, Fields: sec})
		counted[since] = true
	}
	carried := map[int64]Period{}
	for _, d := range dumps {
		for _, raw := range snapshot.AsList(d.Map(section)["periods"]) {
			p, ok := closed(raw)
			if !ok || p.Since < from || counted[p.Since] {
				continue
			}
			if seen, dup := carried[p.Since]; !dup || p.Until > seen.Until {
				carried[p.Since] = p
			}
		}
	}
	for _, p := range carried {
		m.Periods = append(m.Periods, p)
	}
	m.Carried = len(carried)
	sort.SliceStable(m.Periods, func(i, j int) bool { return m.Periods[i].Since < m.Periods[j].Since })
	return m
}

// closed reads one entry of a periods list; false when it lacks since or until.
func closed(raw any) (Period, bool) {
	fields := snapshot.AsMap(raw)
	since, ok1 := snapshot.AsInt(fields["since"])
	until, ok2 := snapshot.AsInt(fields["until"])
	if !ok1 || !ok2 || until < since {
		return Period{}, false
	}
	return Period{Since: since, Until: until, Carried: true, Fields: fields}, true
}

// Describe adds to out how many carried periods were merged (nothing when none was).
func (m Merge) Describe(out map[string]any) {
	if m.Carried == 0 || out == nil {
		return
	}
	out["periodsMerged"] = m.Carried
	out["periodsNote"] = "includes periods closed by earlier saves to disk that the server never read (carried by the addon in the section's periods list)"
}
