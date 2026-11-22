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

const assumption = "Heuristic: I warn about a missing buff only when you know the spell (spells section) and none of its group is among the auras. " +
	"I do not know whether the buff is needed right now (e.g. in combat or in town), and the auras of other players and the pet are not in the dump " +
	"visible other than as player auras. The dump shows the state at save time: after /reload it may already differ."

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

// View describes the character's auras and the missing class buffs.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"assumption": assumption}
	auras := s.List("auras")
	if s["auras"] == nil {
		out["note"] = "The dump has no auras section (old addon or the section was disabled with /mcp off auras)."
		return out
	}
	dumped, hasDump := s.DumpedAt()
	if hasDump {
		out["dumpedAt"] = snapshot.FormatTime(dumped)
		if fresh.Now != nil {
			out["dumpAge"] = snapshot.FormatAge(fresh.Age(dumped))
		}
		if fresh.Now != nil && fresh.IsStale(dumped) {
			out["warning"] = "The dump is old, so auras may have expired or new ones appeared; run /mcp reload."
		}
	}

	var buffList, debuffList []any
	var active []aura
	anyTiming := false
	for _, raw := range auras {
		a := snapshot.AsMap(raw)
		name, _ := a["name"].(string)
		if name == "" {
			continue
		}
		entry, timed := auraEntry(a, name, dumped, hasDump, fresh)
		anyTiming = anyTiming || timed
		if a["harmful"] == true {
			debuffList = append(debuffList, entry)
			continue
		}
		buffList = append(buffList, entry)
		active = append(active, auraOf(a, name))
	}
	out["buffs"] = nonNil(buffList)
	out["debuffs"] = nonNil(debuffList)
	if !anyTiming {
		out["timingNote"] = "The dump has no aura end times (the client did not give them, or the dump comes from an addon older than 0.7.0), so the time left is not given."
	}
	known := knownSpells(s)
	out["missing"] = nonNil(missing(s, known, active))
	// On an English client with rules for the class the name groups are the better answer (they know which buffs exclude
	// each other); otherwise the ID and icon list is all there is.
	if unmatched := selfBuffsNotActive(known, active); len(unmatched) > 0 && (!s.EnglishClient() || len(rules()[classOf(s)]) == 0) {
		out["selfBuffsNotActive"] = unmatched
		out["selfBuffsNote"] = "Spells the client flags as self buffs that you know and that have no active aura with the same spell ID or icon. Matched by IDs and icons, so it works in any game language, but it does not know which buffs exclude each other (e.g. only one Aspect at a time), so not every entry is a real gap."
	}
	s.AddEnglishOnlyNote(out, "Missing class buff groups (the \"missing\" list) are matched by English spell names (e.g. Aspect of the ..., Arcane Intellect), because no language-independent spell grouping is in the dump; the selfBuffsNotActive list uses spell IDs and icons instead")
	return out
}

// aura is an active buff: the name, the spell ID and the icon file ID (addon 0.10.0; 0 = not in the dump).
type aura struct {
	name     string
	id, icon int64
}

func auraOf(a map[string]any, name string) aura {
	id, _ := snapshot.AsInt(a["id"])
	icon, _ := snapshot.AsInt(a["icon"])
	return aura{name: name, id: id, icon: icon}
}

// spell is a known spell: the name, the spell ID, the icon file ID and whether the client flags it as a self buff.
type spell struct {
	name     string
	id, icon int64
	selfBuff bool
}

// isActive says whether an active aura is this spell's buff: the same spell ID or the same icon (every rank of a
// spell has one icon), whatever language the names are in.
func (sp spell) isActive(active []aura) bool {
	for _, a := range active {
		if (sp.id > 0 && a.id == sp.id) || (sp.icon > 0 && a.icon == sp.icon) {
			return true
		}
	}
	return false
}

// selfBuffsNotActive lists the known self-buff spells with no active aura of the same ID or icon: one entry per icon
// (several ranks of a spell are one buff), sorted by name.
func selfBuffsNotActive(known []spell, active []aura) []any {
	seen := map[string]bool{}
	var names []string
	for _, sp := range known {
		if !sp.selfBuff || sp.isActive(active) {
			continue
		}
		key := sp.name
		if sp.icon > 0 {
			key = fmt.Sprint("icon:", sp.icon)
		}
		if !seen[key] {
			seen[key] = true
			names = append(names, sp.name)
		}
	}
	sort.Strings(names)
	out := make([]any, len(names))
	for i, n := range names {
		out[i] = n
	}
	return out
}

func nonNil(l []any) []any {
	if l == nil {
		return []any{}
	}
	return l
}

// auraEntry describes one aura; timed = the dump had duration data.
func auraEntry(a map[string]any, name string, dumped int64, hasDump bool, fresh snapshot.Freshness) (map[string]any, bool) {
	entry := map[string]any{"name": name}
	if id, ok := snapshot.AsInt(a["id"]); ok {
		entry["id"] = id
	}
	if n, ok := snapshot.AsInt(a["stacks"]); ok {
		entry["stacks"] = n
	}
	var expires int64
	switch {
	case hasInt(a, "expiresAt"):
		expires, _ = snapshot.AsInt(a["expiresAt"])
	case hasInt(a, "remaining") && hasDump:
		left, _ := snapshot.AsInt(a["remaining"])
		expires = dumped + left
	default:
		return entry, false
	}
	entry["expiresAt"] = snapshot.FormatTime(expires)
	if hasDump {
		entry["leftAtDump"] = snapshot.FormatSeconds(max(expires-dumped, 0))
	}
	if fresh.Now != nil {
		left, passed := fresh.Until(expires)
		if passed {
			entry["expired"] = true
		} else {
			entry["leftNow"] = snapshot.FormatAge(left.Round(time.Second))
		}
	}
	return entry, true
}

func hasInt(m map[string]any, key string) bool {
	_, ok := snapshot.AsInt(m[key])
	return ok
}

// classOf is the class token (HUNTER...); the class name stands in for it only on English clients.
func classOf(s snapshot.Snapshot) string {
	class, _ := s.Character()["classToken"].(string)
	if class == "" && s.EnglishClient() { // the class name is a token only in English (Hunter -> HUNTER)
		class = strings.ToUpper(fmt.Sprint(s.Character()["class"]))
	}
	return class
}

// missing returns the class buff groups whose spell the character knows but with no aura from the group.
func missing(s snapshot.Snapshot, knownList []spell, active []aura) []any {
	class := classOf(s)
	known := map[string]bool{}
	for _, sp := range knownList {
		known[sp.name] = true
	}
	var out []any
	for _, g := range rules()[class] {
		spells := inGroup(g, known)
		if len(spells) == 0 || anyIn(g, active, knownList) {
			continue
		}
		out = append(out, map[string]any{
			"group":       g.label,
			"knownSpells": spells,
			"text":        fmt.Sprintf("No active buff from the %s group, and you know: %s.", g.label, strings.Join(spells, ", ")),
		})
	}
	if class == "HUNTER" {
		out = append(out, petWarnings(s, known)...)
	}
	return out
}

// petWarnings: pet warnings only when the pet section is in the dump and the character knows the relevant spell.
func petWarnings(s snapshot.Snapshot, known map[string]bool) []any {
	if s["pet"] == nil {
		return nil
	}
	pet := s.Map("pet")
	var out []any
	switch {
	case pet["exists"] == false && known["Call Pet"]:
		out = append(out, map[string]any{"group": "Pet", "knownSpells": []string{"Call Pet"}, "text": "No pet with the character, and you know Call Pet."})
	case pet["exists"] == true && pet["dead"] == true && known["Revive Pet"]:
		out = append(out, map[string]any{"group": "Pet", "knownSpells": []string{"Revive Pet"}, "text": "The pet is dead, and you know Revive Pet."})
	}
	return out
}

func knownSpells(s snapshot.Snapshot) []spell {
	var out []spell
	for _, raw := range s.List("spells") {
		m := snapshot.AsMap(raw)
		name, _ := m["name"].(string)
		if name == "" {
			continue
		}
		sp := spell{name: name, selfBuff: m["selfBuff"] == true && m["passive"] != true}
		sp.id, _ = snapshot.AsInt(m["id"])
		sp.icon, _ = snapshot.AsInt(m["icon"])
		out = append(out, sp)
	}
	return out
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
func anyIn(g group, active []aura, known []spell) bool {
	for _, a := range active {
		if g.matches(a.name) {
			return true
		}
	}
	for _, sp := range known {
		if g.matches(sp.name) && sp.isActive(active) {
			return true
		}
	}
	return false
}
