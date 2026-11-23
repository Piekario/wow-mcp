// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package compare puts two characters of the account side by side: level, class, gold,
// play time, professions, equipment slot by slot and the dump age of each.
package compare

import (
	"fmt"
	"strings"

	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_compare_characters.
type Query struct {
	A string `json:"a,omitempty" jsonschema:"first character; empty = the most recently dumped character"`
	B string `json:"b,omitempty" jsonschema:"second character; empty = the only other character on the account (when there is just one)"`
}

// slotOrder is the slot order in the response; slots outside the list go last.
var slotOrder = []string{"head", "neck", "shoulder", "back", "chest", "shirt", "tabard", "wrist", "hands", "waist", "legs", "feet",
	"finger1", "finger2", "trinket1", "trinket2", "mainhand", "offhand", "ranged", "ammo"}

// notGear are the slots not counted toward the average item level.
var notGear = map[string]bool{"shirt": true, "tabard": true, "ammo": true}

var qualityNames = []string{"Poor", "Common", "Uncommon", "Rare", "Epic", "Legendary", "Artifact", "Heirloom"}

// Resolve picks two different characters: empty a = the most recently dumped, empty b = the only other character.
// A name can be the full key ("Piekasz-Realm"), a name without the realm or an unambiguous prefix, case-insensitive.
func Resolve(db *snapshot.Database, q Query) (a, b string, err error) {
	if db == nil || len(db.Characters) == 0 {
		return "", "", fmt.Errorf("no characters in the dump. %s", snapshot.AltsHint)
	}
	if len(db.Characters) < 2 {
		return "", "", fmt.Errorf("the account has only one character (%v), nothing to compare with. %s", db.CharacterNames(), snapshot.AltsHint)
	}
	if a, err = find(db, q.A, db.Last); err != nil {
		return "", "", err
	}
	if q.B == "" {
		var others []string
		for _, n := range db.CharacterNames() {
			if n != a {
				others = append(others, n)
			}
		}
		if len(others) != 1 {
			return "", "", fmt.Errorf("specify the second character (b): the account has several. Available: %v", others)
		}
		return a, others[0], nil
	}
	if b, err = find(db, q.B, ""); err != nil {
		return "", "", err
	}
	if a == b {
		return "", "", fmt.Errorf("that is the same character (%s); give two different ones. Available: %v", a, db.CharacterNames())
	}
	return a, b, nil
}

// find matches a name to a character key; empty query = fallback.
func find(db *snapshot.Database, query, fallback string) (string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		q = fallback
	}
	if _, ok := db.Characters[q]; ok {
		return q, nil
	}
	low := strings.ToLower(q)
	var exact, prefix []string
	for _, n := range db.CharacterNames() {
		ln := strings.ToLower(n)
		short, _, _ := strings.Cut(ln, "-")
		switch {
		case ln == low || short == low:
			exact = append(exact, n)
		case low != "" && strings.HasPrefix(ln, low):
			prefix = append(prefix, n)
		}
	}
	for _, found := range [][]string{exact, prefix} {
		if len(found) == 1 {
			return found[0], nil
		}
		if len(found) > 1 {
			return "", fmt.Errorf("name %q matches several characters: %v. Give the full name", query, found)
		}
	}
	return "", fmt.Errorf("no character %q in the dump. Available: %v", query, db.CharacterNames())
}

func View(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	an, bn, err := Resolve(db, q)
	if err != nil {
		return nil, err
	}
	sa, sb := db.Characters[an], db.Characters[bn]
	ca, cb := card(an, sa, fresh), card(bn, sb, fresh)
	out := map[string]any{
		"characters": []any{ca, cb},
		"slots":      slots(sa, sb),
	}
	out["summary"] = summary(an, bn, ca, cb)
	if note := timeNote(an, bn, sa, sb); note != "" {
		out["dataNote"] = note
	}
	out["note"] = "Average ilvl is computed from equipped items excluding shirt, tabard and ammo. Each character shows data from its own latest dump (ageMinutes)."
	return out, nil
}

func card(name string, s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	c := s.Character()
	level, money := s.LevelAndMoney()
	meta := fresh.SnapshotMeta(name, s)
	e := map[string]any{
		"name": name, "level": level, "class": c["class"], "race": c["race"], "faction": c["faction"],
		"zone": c["zone"], "subZone": c["subZone"],
		"money": snapshot.Money(money).String(), "moneyCopper": money,
		"professions": professions.Summary(s),
		"dumpedAt":    meta["dumpedAt"], "ageMinutes": meta["ageMinutes"],
	}
	if meta["stale"] == true {
		e["stale"] = true
	}
	if rested, ok := snapshot.AsInt(c["restedXp"]); ok {
		e["restedXp"] = rested
	}
	if played := s.Map("played"); played != nil {
		if total, ok := snapshot.AsInt(played["totalSeconds"]); ok {
			e["played"] = snapshot.FormatSeconds(total)
			e["playedSeconds"] = total
		}
	}
	if avg, n := averageItemLevel(s); n > 0 {
		e["avgItemLevel"] = avg
		e["gearPieces"] = n
	}
	return e
}

// averageItemLevel is the average ilvl of equipped armor and weapon pieces (rounded to 0.1).
func averageItemLevel(s snapshot.Snapshot) (avg float64, pieces int) {
	var sum int64
	for _, raw := range s.Equipment() {
		it := snapshot.AsMap(raw)
		slot, _ := it["slot"].(string)
		lvl, ok := snapshot.AsInt(it["itemLevel"])
		if !ok || notGear[slot] {
			continue
		}
		sum += lvl
		pieces++
	}
	if pieces == 0 {
		return 0, 0
	}
	return float64(sum*10/int64(pieces)) / 10, pieces
}

// slotEntry describes the item in a slot; nil for an empty slot.
func slotEntry(it map[string]any) map[string]any {
	if it == nil {
		return nil
	}
	e := map[string]any{"name": it["name"]}
	if lvl, ok := snapshot.AsInt(it["itemLevel"]); ok {
		e["itemLevel"] = lvl
	}
	if q, ok := snapshot.AsInt(it["quality"]); ok {
		if q >= 0 && int(q) < len(qualityNames) {
			e["quality"] = qualityNames[q]
		}
	}
	return e
}

// slots lists equipment by slot; better says which side has the higher ilvl.
func slots(sa, sb snapshot.Snapshot) []any {
	ea, eb := snapshot.EquippedBySlot(sa.Equipment()), snapshot.EquippedBySlot(sb.Equipment())
	seen := map[string]bool{}
	var order []string
	for _, s := range slotOrder {
		if ea[s] != nil || eb[s] != nil {
			order = append(order, s)
			seen[s] = true
		}
	}
	for _, m := range []map[string]map[string]any{ea, eb} {
		for s := range m {
			if !seen[s] {
				seen[s] = true
				order = append(order, s)
			}
		}
	}
	out := make([]any, 0, len(order))
	for _, slot := range order {
		e := map[string]any{"slot": slot, "a": slotEntry(ea[slot]), "b": slotEntry(eb[slot])}
		if !notGear[slot] {
			e["better"] = better(ea[slot], eb[slot])
		}
		out = append(out, e)
	}
	return out
}

// better compares ilvl: "a", "b" or "=" (equal or no data on either side).
func better(a, b map[string]any) string {
	la, _ := snapshot.AsInt(a["itemLevel"])
	lb, _ := snapshot.AsInt(b["itemLevel"])
	switch {
	case la > lb:
		return "a"
	case lb > la:
		return "b"
	}
	return "="
}

// summary gives short conclusions as text.
func summary(an, bn string, ca, cb map[string]any) []string {
	var out []string
	la, _ := snapshot.AsInt(ca["level"])
	lb, _ := snapshot.AsInt(cb["level"])
	if la != lb {
		hi, lo := an, bn
		if lb > la {
			hi, lo = bn, an
		}
		out = append(out, fmt.Sprintf("Higher level: %s (by %d over %s).", hi, max(la-lb, lb-la), lo))
	}
	ma, _ := snapshot.AsInt(ca["moneyCopper"])
	mb, _ := snapshot.AsInt(cb["moneyCopper"])
	if ma != mb {
		hi := an
		if mb > ma {
			hi = bn
		}
		out = append(out, fmt.Sprintf("More gold: %s (difference %s).", hi, snapshot.Money(max(ma-mb, mb-ma))))
	}
	ia, oka := ca["avgItemLevel"].(float64)
	ib, okb := cb["avgItemLevel"].(float64)
	if oka && okb && ia != ib {
		hi := an
		if ib > ia {
			hi = bn
		}
		out = append(out, fmt.Sprintf("Higher average ilvl: %s (%.1f vs %.1f).", hi, max(ia, ib), min(ia, ib)))
	}
	return out
}

// timeNote warns when the two characters' dumps come from different moments.
func timeNote(an, bn string, sa, sb snapshot.Snapshot) string {
	ta, oka := sa.DumpedAt()
	tb, okb := sb.DumpedAt()
	if !oka || !okb {
		return "One of the dumps has no save time, so it is unknown how far apart the data is."
	}
	gap := max(ta-tb, tb-ta)
	if gap < 600 {
		return ""
	}
	older := an
	if tb < ta {
		older = bn
	}
	return fmt.Sprintf("The dumps come from different moments (difference %s): the data of %s is older. To make the comparison fair, log in on both characters and type /mcp reload.", snapshot.FormatSeconds(gap), older)
}
