// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strconv"
	"strings"

	"wowmcp/internal/game/itemkind"
)

// filter holds the shared item filters for wow_bank and wow_vendor.
type filter struct {
	query      string // name fragment, case-insensitive
	kind       string // type or subtype, e.g. Weapon, Armor, Trade Goods, Dagger
	minQuality int64  // 0 = no filter
}

func (f filter) matches(it Item) bool {
	if f.query != "" && !strings.Contains(strings.ToLower(it.Name), strings.ToLower(f.query)) {
		return false
	}
	if f.kind != "" {
		want := strings.ToLower(f.kind)
		byClass := false
		if class, ok := itemkind.ClassByEnglishName(want); ok && it.HasClass {
			byClass = it.Is(class) // the class ID matches the English type name on any client
		}
		if !byClass && !sameKind(it.Type, want) && !sameKind(it.SubType, want) {
			return false
		}
	}
	return f.minQuality <= 0 || it.Quality >= f.minQuality
}

// sameKind compares a type text with the wanted one, ignoring case and a plural s, because the client names
// subclasses in the plural ("Bows", "Daggers") and a player asks for "Bow".
func sameKind(text, want string) bool {
	text = strings.TrimSuffix(strings.ToLower(text), "s")
	return text != "" && text == strings.TrimSuffix(want, "s")
}

// IsOfType says whether the item is of the given type or subtype (e.g. Weapon, Armor, Bow); by class ID where the
// dump has it, otherwise by the type text.
func (i Item) IsOfType(kind string) bool { return filter{kind: kind}.matches(i) }

// matcher matches an item by ID (when the query is a number) or by name fragment.
func matcher(query string) func(Item) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	id, err := strconv.ParseInt(strings.TrimSpace(query), 10, 64)
	return func(it Item) bool {
		if !it.present {
			return false
		}
		if err == nil {
			return it.ID == id
		}
		return strings.Contains(strings.ToLower(it.Name), q)
	}
}
