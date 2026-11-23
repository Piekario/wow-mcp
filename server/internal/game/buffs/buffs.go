// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package buffs describes active effects (the auras section) and warns about missing
// class-typical standing buffs. The warning is a heuristic: it appears only when the character KNOWS the spell
// (spells section) and no effect from its group is among the auras. "Time left" is computed only when the dump has an
// expiresAt or remaining field (addon 0.7.0+ saves expiresAt and duration when the client gives them; older dumps do not).
//
// Locale independence (addon 0.10.0): the groups of "one of these is enough" (all Aspects, all Blessings) are defined by
// English spell names, and no locale-independent group property exists in the dump (the client has no spell family
// field, and the spell IDs of Forever's ranks are not verified). What the dump does give in every language is each
// spell's icon and the aura's icon and spell ID, so an active aura counts as a known spell's buff when the IDs or
// icons are equal (all ranks of a spell share an icon), and the spells the client flags as self buffs (selfBuff)
// can be checked against the active auras without any name (selfBuffsNotActive).
package buffs

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"wowmcp/internal/snapshot"
)

// Aura is an effect on the character. ID and Icon (addon 0.10.0) are zero when the dump has none.
type Aura struct {
	Name         string
	ID, Icon     int64
	HasID        bool
	Stacks       int64
	HasStacks    bool
	Harmful      bool
	ExpiresAt    int64
	HasExpiresAt bool
	Remaining    int64
	HasRemaining bool
}

// Spell is a known spell. SelfBuff is what the client flags as a self buff (and not passive).
type Spell struct {
	Name     string
	ID, Icon int64
	SelfBuff bool
}

// PetState is what the pet section says; PetUnknown covers a missing section and fields the client left out.
type PetState int

const (
	PetUnknown PetState = iota
	PetAbsent
	PetAlive
	PetDead
)

// Dump is everything the buffs rules need from one character's dump. DumpedAt is the time of the dump.
type Dump struct {
	HasAuras      bool
	Auras         []Aura
	Spells        []Spell
	Class         string
	EnglishClient bool
	Pet           PetState
	DumpedAt      int64
	HasDumpedAt   bool
}

// Timing is when an aura ends. LeftAtDump is set when the dump time is known, Left when the clock is.
type Timing struct {
	ExpiresAt     int64
	HasLeftAtDump bool
	LeftAtDump    int64
	HasLeft       bool
	Expired       bool
	Left          time.Duration
}

// AuraStatus is an aura with its timing; Timing is nil when the dump has no end time for it.
type AuraStatus struct {
	Aura
	Timing *Timing
}

type GapKind int

const (
	GapGroup GapKind = iota
	GapNoPet
	GapDeadPet
)

// Gap is a class buff the character knows but has none active. Spells are the known spells of the group.
type Gap struct {
	Kind   GapKind
	Group  string
	Spells []string
}

// Report is the auras and the missing class buffs. HasAuras is false when the dump has no auras section.
type Report struct {
	HasAuras           bool
	HasDump            bool
	DumpedAt           int64
	HasAge             bool
	DumpAge            time.Duration
	Stale              bool
	Buffs, Debuffs     []AuraStatus
	AnyTiming          bool
	Missing            []Gap
	SelfBuffsNotActive []string
}

// group is a family of spells of which one active is enough (e.g. one Aspect at a time).
type group struct {
	label    string
	prefixes []string // the spell/aura name starts with one of these
	names    []string // or is one of these names
}

func (g group) matches(name string) bool {
	for _, p := range g.prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	for _, n := range g.names {
		if name == n {
			return true
		}
	}
	return false
}

// rules are small data-driven rules per class (key: classToken from the character section).
func rules() map[string][]group {
	return map[string][]group{
		"HUNTER":  {{label: "Aspect", prefixes: []string{"Aspect of the "}}},
		"MAGE":    {{label: "Arcane Intellect", names: []string{"Arcane Intellect"}}, {label: "Armor", names: []string{"Frost Armor", "Ice Armor", "Mage Armor", "Molten Armor"}}},
		"PRIEST":  {{label: "Power Word: Fortitude", names: []string{"Power Word: Fortitude"}}},
		"DRUID":   {{label: "Mark of the Wild", names: []string{"Mark of the Wild"}}},
		"WARRIOR": {{label: "Battle Shout", names: []string{"Battle Shout"}}},
		"PALADIN": {{label: "Blessing", prefixes: []string{"Blessing of "}}},
		"WARLOCK": {{label: "Armor", names: []string{"Demon Skin", "Demon Armor", "Fel Armor"}}},
		"SHAMAN":  {{label: "Shield", names: []string{"Lightning Shield", "Water Shield"}}},
	}
}

// Inspect judges the auras against the clock and finds the class buffs that are missing.
func Inspect(d Dump, fresh snapshot.Freshness) Report {
	r := Report{HasAuras: d.HasAuras}
	if !d.HasAuras {
		return r
	}
	if d.HasDumpedAt {
		r.HasDump, r.DumpedAt = true, d.DumpedAt
		if fresh.Now != nil {
			r.HasAge, r.DumpAge, r.Stale = true, fresh.Age(d.DumpedAt), fresh.IsStale(d.DumpedAt)
		}
	}
	var active []Aura
	for _, a := range d.Auras {
		status := AuraStatus{Aura: a, Timing: timingOf(a, d, fresh)}
		r.AnyTiming = r.AnyTiming || status.Timing != nil
		if a.Harmful {
			r.Debuffs = append(r.Debuffs, status)
			continue
		}
		r.Buffs = append(r.Buffs, status)
		active = append(active, a)
	}
	r.Missing = missing(d, active)
	// On an English client with rules for the class the name groups are the better answer (they know which buffs exclude
	// each other); otherwise the ID and icon list is all there is.
	if !d.EnglishClient || len(rules()[d.Class]) == 0 {
		r.SelfBuffsNotActive = selfBuffsNotActive(d.Spells, active)
	}
	return r
}

func timingOf(a Aura, d Dump, fresh snapshot.Freshness) *Timing {
	var expires int64
	switch {
	case a.HasExpiresAt:
		expires = a.ExpiresAt
	case a.HasRemaining && d.HasDumpedAt:
		expires = d.DumpedAt + a.Remaining
	default:
		return nil
	}
	t := &Timing{ExpiresAt: expires}
	if d.HasDumpedAt {
		t.HasLeftAtDump, t.LeftAtDump = true, max(expires-d.DumpedAt, 0)
	}
	if fresh.Now != nil {
		t.HasLeft = true
		t.Left, t.Expired = fresh.Until(expires)
	}
	return t
}

// isActive says whether an active aura is this spell's buff: the same spell ID or the same icon (every rank of a
// spell has one icon), whatever language the names are in.
func (sp Spell) isActive(active []Aura) bool {
	for _, a := range active {
		if (sp.ID > 0 && a.ID == sp.ID) || (sp.Icon > 0 && a.Icon == sp.Icon) {
			return true
		}
	}
	return false
}

// selfBuffsNotActive lists the known self-buff spells with no active aura of the same ID or icon: one entry per icon
// (several ranks of a spell are one buff), sorted by name.
func selfBuffsNotActive(known []Spell, active []Aura) []string {
	seen := map[string]bool{}
	var names []string
	for _, sp := range known {
		if !sp.SelfBuff || sp.isActive(active) {
			continue
		}
		key := sp.Name
		if sp.Icon > 0 {
			key = fmt.Sprint("icon:", sp.Icon)
		}
		if !seen[key] {
			seen[key] = true
			names = append(names, sp.Name)
		}
	}
	sort.Strings(names)
	return names
}

// missing returns the class buff groups whose spell the character knows but with no aura from the group.
func missing(d Dump, active []Aura) []Gap {
	known := map[string]bool{}
	for _, sp := range d.Spells {
		known[sp.Name] = true
	}
	var out []Gap
	for _, g := range rules()[d.Class] {
		spells := inGroup(g, known)
		if len(spells) == 0 || anyIn(g, active, d.Spells) {
			continue
		}
		out = append(out, Gap{Kind: GapGroup, Group: g.label, Spells: spells})
	}
	if d.Class == "HUNTER" {
		out = append(out, petGaps(d.Pet, known)...)
	}
	return out
}

func petGaps(pet PetState, known map[string]bool) []Gap {
	switch {
	case pet == PetAbsent && known["Call Pet"]:
		return []Gap{{Kind: GapNoPet, Group: "Pet", Spells: []string{"Call Pet"}}}
	case pet == PetDead && known["Revive Pet"]:
		return []Gap{{Kind: GapDeadPet, Group: "Pet", Spells: []string{"Revive Pet"}}}
	}
	return nil
}

// inGroup returns the known spells of the group (each once, in a fixed order: by name).
func inGroup(g group, known map[string]bool) []string {
	var out []string
	for name := range known {
		if g.matches(name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// anyIn says whether an active aura belongs to the group: its name matches, or it is the buff (same ID or icon) of a
// known spell that is in the group.
func anyIn(g group, active []Aura, known []Spell) bool {
	for _, a := range active {
		if g.matches(a.Name) {
			return true
		}
	}
	for _, sp := range known {
		if g.matches(sp.Name) && sp.isActive(active) {
			return true
		}
	}
	return false
}
