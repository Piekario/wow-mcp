// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package items covers items: bags, bank, vendor offer, comparison
// with equipped gear and searching for an item across all characters on the account.
package items

import (
	"strconv"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// filter holds the shared item filters for wow_bank and wow_vendor.
type filter struct {
	query      string // name fragment, case-insensitive
	kind       string // type or subtype, e.g. Weapon, Armor, Trade Goods, Dagger
	minQuality int64  // 0 = no filter
}

func (f filter) match(item map[string]any) bool {
	if f.query != "" {
		name, _ := item["name"].(string)
		if !strings.Contains(strings.ToLower(name), strings.ToLower(f.query)) {
			return false
		}
	}
	if f.kind != "" {
		t, _ := item["type"].(string)
		st, _ := item["subType"].(string)
		want := strings.ToLower(f.kind)
		byClass := false
		if class, ok := itemkind.ClassByEnglishName(want); ok && itemkind.HasIDs(item) {
			byClass = itemkind.Is(item, class) // the class ID matches the English type name on any client
		}
		if !byClass && !sameKind(t, want) && !sameKind(st, want) {
			return false
		}
	}
	if f.minQuality > 0 {
		q, _ := snapshot.AsInt(item["quality"])
		if q < f.minQuality {
			return false
		}
	}
	return true
}

// sameKind compares a type text with the wanted one, ignoring case and a plural s, because the client names
// subclasses in the plural ("Bows", "Daggers") and a player asks for "Bow".
func sameKind(text, want string) bool {
	text = strings.TrimSuffix(strings.ToLower(text), "s")
	return text != "" && text == strings.TrimSuffix(want, "s")
}

// matcher matches an item by ID (when the query is a number) or by name fragment.
func matcher(query string) func(map[string]any) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	id, err := strconv.ParseInt(strings.TrimSpace(query), 10, 64)
	return func(item map[string]any) bool {
		if item == nil {
			return false
		}
		if err == nil {
			n, _ := snapshot.AsInt(item["id"])
			return n == id
		}
		name, _ := item["name"].(string)
		return strings.Contains(strings.ToLower(name), q)
	}
}

// qualityName turns quality 0-7 into the in-game name; an empty string when there is none.
func qualityName(item map[string]any) string {
	q, ok := snapshot.AsInt(item["quality"])
	if !ok {
		return ""
	}
	name, _ := itemkind.QualityName(q)
	return name
}

// copyItem makes a shallow copy of an item to which the view adds computed fields.
func copyItem(item map[string]any) map[string]any {
	entry := map[string]any{}
	for k, v := range item {
		entry[k] = v
	}
	return entry
}
