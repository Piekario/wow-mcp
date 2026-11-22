// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package snapshot is the shared domain core: the character state snapshot saved by
// the WowMcp addon, the database of snapshots of all characters on the account, safe reads
// of values, formatting (money, time) and the data freshness policy.
//
// The snapshot deliberately has no rigid schema: the addon can add a section that is immediately
// visible through wow_section, without changes in the server. That is why Snapshot is a map,
// and only the reads needed by many contexts are typed.
package snapshot

import (
	"sort"
	"strconv"
)

// Snapshot is the snapshot of one character: section name -> value from the SavedVariables file.
// Values have the types produced by the Lua parser: map[string]any, []any, string, int64, float64, bool, nil.
type Snapshot map[string]any

// Snapshot fields that describe the snapshot itself, not the character state.
// timings and disabledSections are addon diagnostics (collection time, disabled sections), not character state;
// lastSkippedLogoutAt and clientUnloaded (0.7.1) describe a dump taken at logout from an empty client.
var metaFields = map[string]bool{
	"errors": true, "dumpedAt": true, "dumpedAtText": true, "reason": true, "addonVersion": true,
	"timings": true, "disabledSections": true, "lastSkippedLogoutAt": true, "clientUnloaded": true,
}

// IsMetaField says whether a field describes the snapshot itself (time, addon errors) rather than the character state.
func IsMetaField(name string) bool { return metaFields[name] }

// SectionNames returns the sorted names of the sections present in the snapshot.
func (s Snapshot) SectionNames() []string {
	names := []string{}
	for name := range s {
		if !metaFields[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// DumpedAt is the dump time (Unix); false when the snapshot has none (old addon).
func (s Snapshot) DumpedAt() (int64, bool) {
	at, ok := s["dumpedAt"].(int64)
	return at, ok
}

// Map returns a section that is a keyed table; nil when there is none.
func (s Snapshot) Map(section string) map[string]any { return AsMap(s[section]) }

// List returns a section that is a list; nil when there is none.
func (s Snapshot) List(section string) []any { return AsList(s[section]) }

// Character is the character section: level, class, gold, XP, zone.
func (s Snapshot) Character() map[string]any { return s.Map("character") }

// Equipment is the list of equipped items (each with a slot field).
func (s Snapshot) Equipment() []any { return s.List("equipment") }

// Bags is the list of bags (each with size, free, items fields).
func (s Snapshot) Bags() []any { return s.List("bags") }

// BankContainers are the bank containers from the last visit.
func (s Snapshot) BankContainers() []any { return AsList(s.Map("bank")["containers"]) }

// LevelAndMoney returns the character's level and gold (in copper) from the character section.
func (s Snapshot) LevelAndMoney() (level, money int64) {
	c := s.Character()
	level, _ = AsInt(c["level"])
	money, _ = AsInt(c["moneyCopper"])
	return level, money
}

// EquippedBySlot indexes equipped items by slot name.
func EquippedBySlot(equipment []any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, raw := range equipment {
		item := AsMap(raw)
		if slot, ok := item["slot"].(string); ok {
			out[slot] = item
		}
	}
	return out
}

// ItemCount is the number of items in a stack; an item without a count field is one item.
func ItemCount(item map[string]any) int64 {
	n, ok := AsInt(item["count"])
	if !ok {
		return 1
	}
	return n
}

// ReagentName is the name of a recipe material; when the client did not provide it (the item is not loaded yet,
// addon 0.7.1 takes it from C_Item.GetItemNameByID), it is replaced with "item <id>" so the gap is not an empty string.
func ReagentName(reagent map[string]any) string {
	if name, _ := reagent["name"].(string); name != "" {
		return name
	}
	if id, ok := AsInt(reagent["id"]); ok {
		return "item " + strconv.FormatInt(id, 10)
	}
	return ""
}
