// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/character"
	"wowmcp/internal/game/loot"
	"wowmcp/internal/game/periods"
	"wowmcp/internal/game/talents"
	"wowmcp/internal/game/xpgains"
	"wowmcp/internal/snapshot"
)

// Change is a number in the older and the newer dump.
type Change struct{ Before, After int64 }

func (c Change) Delta() int64 { return c.After - c.Before }

// XPChange is the experience in both dumps; Gained is known within one level and across a single level-up
// (then the XP of the previous level is estimated from its xpMax).
type XPChange struct {
	LevelBefore, LevelAfter, XPBefore, XPAfter int64
	Gained                                     int64
	GainedKnown                                bool
}

func (x XPChange) LevelUp() bool { return x.LevelAfter > x.LevelBefore }

// ZoneChange keeps the zones as the client gave them.
type ZoneChange struct{ Before, After any }

type ItemChange struct {
	Name string
	ID   int64
	Change
}

type GearChange struct{ Slot, Before, After string }

// QuestRef keeps the quest's ID, title and zone exactly as the dump holds them.
type QuestRef struct {
	ID, Title, Zone any
	HasZone         bool
}

type ObjectiveChange struct {
	Text, Need any
	Change
}

type QuestProgress struct {
	ID         int64
	Title      any
	Objectives []ObjectiveChange
}

type QuestChanges struct {
	New, ReadyToTurnIn, TurnedIn, RemovedFromLog []QuestRef
	Progress                                     []QuestProgress
	CompletedCountChange                         int
}

func (q QuestChanges) Empty() bool {
	return len(q.New)+len(q.ReadyToTurnIn)+len(q.TurnedIn)+len(q.RemovedFromLog)+len(q.Progress) == 0 && q.CompletedCountChange == 0
}

// ReputationChange is a faction met for the first time (New) or one whose value moved.
type ReputationChange struct {
	Name                     string
	Standing, StandingBefore any
	New, StandingChanged     bool
	Change                   int64
}

type Spell struct {
	Name, Rank any
	HasRank    bool
}

func (s Spell) key() string { return fmt.Sprintf("%v|%v", s.Name, s.Rank) }

// SkillChange is a profession or skill rank change; a New one has no Before.
type SkillChange struct {
	Name          string
	Before, After int64
	Max           any
	New           bool
}

type CurrencyChange struct {
	Name string
	Change
}

// LootGained is the player's own loot gained after the older dump, with the carried periods merged into it.
type LootGained struct {
	Items []loot.Item
	Merge periods.Merge
}

type XPGained struct {
	Gains xpgains.Gains
	Merge periods.Merge
}

// Comparison is what changed between two dumps of the same character. Nil and empty fields did not change.
type Comparison struct {
	From, To       int64
	HasFrom, HasTo bool
	Money          *Change
	XP             *XPChange
	Zone           *ZoneChange
	Gained, Lost   []ItemChange
	Equipment      []GearChange
	Quests         QuestChanges
	Reputation     []ReputationChange
	NewSpells      []Spell
	Professions    []SkillChange
	Currencies     []CurrencyChange
	PlayedBetween  int64
	Deaths         *Change
	Loot           *LootGained
	XPSources      *XPGained
	TalentPoints   *Change
	TalentsLearned []any
}

// HasChanges says whether anything changed besides the time span itself.
func (c Comparison) HasChanges() bool {
	return c.Money != nil || c.XP != nil || c.Zone != nil || len(c.Gained)+len(c.Lost) > 0 || len(c.Equipment) > 0 ||
		!c.Quests.Empty() || len(c.Reputation) > 0 || len(c.NewSpells) > 0 || len(c.Professions) > 0 ||
		len(c.Currencies) > 0 || c.Deaths != nil || c.Loot != nil || c.XPSources != nil || c.TalentPoints != nil ||
		len(c.TalentsLearned) > 0
}

// Compare describes what changed between two dumps of the same character: gold, level and XP, bag items, equipment,
// quests, reputation, new abilities, professions, currencies, deaths, loot and talents.
func Compare(before, after snapshot.Snapshot) Comparison {
	var c Comparison
	c.From, c.HasFrom = before.DumpedAt()
	if c.HasFrom {
		c.To, c.HasTo = after.DumpedAt()
	}
	cb, ca := readCharacter(before), readCharacter(after)
	if cb.HasMoney && ca.HasMoney && ca.Money != cb.Money {
		c.Money = &Change{cb.Money, ca.Money}
	}
	c.XP = diffXP(cb, ca)
	if cb.Zone != ca.Zone && ca.Zone != nil {
		c.Zone = &ZoneChange{cb.Zone, ca.Zone}
	}
	c.Gained, c.Lost = diffItems(readBagCounts(before), readBagCounts(after))
	c.Equipment = diffEquipment(readEquipment(before), readEquipment(after))
	c.Quests = diffQuests(before, after)
	c.Reputation = diffReputation(readFactions(before), readFactions(after))
	c.NewSpells = newSpells(readSpells(before), readSpells(after))
	c.Professions = diffSkills(readSkills(before), readSkills(after))
	c.Currencies = diffCurrencies(readCurrencies(before), readCurrencies(after))
	if pb, ok := readPlayedTotal(before); ok {
		if pa, ok := readPlayedTotal(after); ok && pa > pb {
			c.PlayedBetween = pa - pb
		}
	}
	c.Deaths = diffDeaths(before, after)
	c.Loot = diffLoot(before, after)
	c.XPSources = diffXPGains(before, after)
	if sb, ok := readTalentsSpent(before); ok {
		if sa, ok := readTalentsSpent(after); ok && sa != sb {
			c.TalentPoints = &Change{sb, sa}
		}
	}
	c.TalentsLearned = talents.Diff(before["talents"], after["talents"])
	return c
}

// diffDeaths is nil when the counter did not increase or is missing from one of the dumps.
func diffDeaths(before, after snapshot.Snapshot) *Change {
	nb, ok1 := character.DeathCount(before)
	na, ok2 := character.DeathCount(after)
	if !ok1 || !ok2 || na <= nb {
		return nil
	}
	return &Change{nb, na}
}

// diffXP is nil when a dump lacks the numbers or nothing changed.
func diffXP(before, after characterState) *XPChange {
	if !(before.HasLevel && after.HasLevel && before.HasXP && after.HasXP) {
		return nil
	}
	if before.Level == after.Level && before.XP == after.XP {
		return nil
	}
	x := &XPChange{LevelBefore: before.Level, LevelAfter: after.Level, XPBefore: before.XP, XPAfter: after.XP}
	switch {
	case after.Level == before.Level:
		x.Gained, x.GainedKnown = after.XP-before.XP, true
	case after.Level == before.Level+1 && before.HasXPMax:
		x.Gained, x.GainedKnown = (before.XPMax-before.XP)+after.XP, true
	}
	return x
}

// xpGained is the XP between two dumps; false when a jump of 2+ levels makes it impossible to compute.
func xpGained(before, after characterState) (int64, bool) {
	x := diffXP(before, after)
	if x == nil {
		return 0, true
	}
	return x.Gained, x.GainedKnown
}

type itemCount struct {
	ID    int64
	Name  string
	Count int64
}

func diffItems(before, after map[string]*itemCount) (gained, lost []ItemChange) {
	names := map[string]bool{}
	for n := range before {
		names[n] = true
	}
	for n := range after {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	for _, n := range sorted {
		entry := ItemChange{Name: n}
		if c := before[n]; c != nil {
			entry.Before, entry.ID = c.Count, c.ID
		}
		if c := after[n]; c != nil {
			entry.After, entry.ID = c.Count, c.ID
		}
		switch {
		case entry.After > entry.Before:
			gained = append(gained, entry)
		case entry.After < entry.Before:
			lost = append(lost, entry)
		}
	}
	return gained, lost
}

func diffEquipment(b, a map[string]string) []GearChange {
	slots := map[string]bool{}
	for s := range b {
		slots[s] = true
	}
	for s := range a {
		slots[s] = true
	}
	var names []string
	for s := range slots {
		if b[s] != a[s] {
			names = append(names, s)
		}
	}
	sort.Strings(names)
	var out []GearChange
	for _, s := range names {
		out = append(out, GearChange{Slot: s, Before: b[s], After: a[s]})
	}
	return out
}

type objective struct {
	text, need any
	have       int64
}

type quest struct {
	QuestRef
	complete   bool
	objectives []objective
}

func diffQuests(before, after snapshot.Snapshot) QuestChanges {
	qb, qa := readQuests(before), readQuests(after)
	completedBefore, listedBefore := readCompleted(before)
	completedAfter, listedAfter := readCompleted(after)
	var out QuestChanges
	for _, id := range sortedIDs(qa) {
		q := qa[id]
		old, had := qb[id]
		if !had {
			out.New = append(out.New, q.QuestRef)
			continue
		}
		if q.complete && !old.complete {
			out.ReadyToTurnIn = append(out.ReadyToTurnIn, q.QuestRef)
		}
		if changes := objectiveChanges(old, q); len(changes) > 0 {
			out.Progress = append(out.Progress, QuestProgress{ID: id, Title: q.Title, Objectives: changes})
		}
	}
	for _, id := range sortedIDs(qb) {
		if _, still := qa[id]; still {
			continue
		}
		if completedAfter[id] && !completedBefore[id] {
			out.TurnedIn = append(out.TurnedIn, qb[id].QuestRef)
		} else {
			out.RemovedFromLog = append(out.RemovedFromLog, qb[id].QuestRef)
		}
	}
	if listedBefore && listedAfter {
		if n := len(completedAfter) - len(completedBefore); n > 0 {
			out.CompletedCountChange = n
		}
	}
	return out
}

func sortedIDs(m map[int64]quest) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func objectiveChanges(before, after quest) []ObjectiveChange {
	var out []ObjectiveChange
	for i, a := range after.objectives {
		if i >= len(before.objectives) {
			break
		}
		if b := before.objectives[i]; a.have != b.have {
			out = append(out, ObjectiveChange{Text: a.text, Need: a.need, Change: Change{b.have, a.have}})
		}
	}
	return out
}

type faction struct {
	name            string
	named, hasValue bool
	value           int64
	standing        any
}

func diffReputation(before, after []faction) []ReputationChange {
	known := map[string]faction{}
	for _, f := range before {
		if f.named {
			known[f.name] = f
		}
	}
	var out []ReputationChange
	for _, f := range after {
		if f.name == "" || !f.hasValue {
			continue
		}
		old, had := known[f.name]
		if !had {
			out = append(out, ReputationChange{Name: f.name, Standing: f.standing, New: true})
			continue
		}
		if f.value == old.value {
			continue
		}
		out = append(out, ReputationChange{Name: f.name, Standing: f.standing, Change: f.value - old.value,
			StandingBefore: old.standing, StandingChanged: old.standing != f.standing})
	}
	return out
}

func newSpells(before, after []Spell) []Spell {
	known := map[string]bool{}
	for _, s := range before {
		known[s.key()] = true
	}
	var out []Spell
	for _, s := range after {
		if !known[s.key()] {
			out = append(out, s)
		}
	}
	return out
}

type skill struct {
	name  string
	named bool
	rank  int64
	max   any
}

func diffSkills(before, after []skill) []SkillChange {
	ranks := map[string]int64{}
	for _, p := range before {
		if p.named {
			ranks[p.name] = p.rank
		}
	}
	var out []SkillChange
	for _, p := range after {
		if old, ok := ranks[p.name]; ok && old != p.rank {
			out = append(out, SkillChange{Name: p.name, Before: old, After: p.rank, Max: p.max})
		} else if !ok && p.name != "" {
			out = append(out, SkillChange{Name: p.name, After: p.rank, Max: p.max, New: true})
		}
	}
	return out
}

type currency struct {
	name     string
	named    bool
	quantity int64
}

func diffCurrencies(before, after []currency) []CurrencyChange {
	quantities := map[string]int64{}
	for _, c := range before {
		if c.named {
			quantities[c.name] = c.quantity
		}
	}
	var out []CurrencyChange
	for _, c := range after {
		if qb := quantities[c.name]; c.name != "" && c.quantity != qb {
			out = append(out, CurrencyChange{Name: c.name, Change: Change{qb, c.quantity}})
		}
	}
	return out
}

// diffLoot: the loot section of the newer dump carries what dropped since the previous write to disk, and since
// addon 0.12.0 also the closed periods of saves the server never read (merged when they started at or after the older dump).
func diffLoot(before, after snapshot.Snapshot) *LootGained {
	items, m := loot.Gained(before, []snapshot.Snapshot{after})
	if !m.Found || len(items) == 0 {
		return nil
	}
	return &LootGained{items, m}
}

func diffXPGains(before, after snapshot.Snapshot) *XPGained {
	g, m := xpgains.Gained(before, []snapshot.Snapshot{after})
	if !m.Found || g.Empty() {
		return nil
	}
	return &XPGained{g, m}
}
