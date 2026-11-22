// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package cooldowns covers long cooldowns: abilities, items (Hearthstone)
// and profession recipes, for one character or the whole account.
package cooldowns

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_cooldowns.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	All       bool   `json:"all,omitempty" jsonschema:"true = cooldowns of all characters on the account"`
}

// For combines the cooldowns from the dump (abilities and items) with recipe cooldowns
// and computes how much is left as of now. A cooldown that has already passed has ready = true.
// A non-empty character name is put on every entry (the whole-account view).
func For(character string, s snapshot.Snapshot, fresh snapshot.Freshness) []any {
	var out []any
	seen := map[string]bool{}
	add := func(name string, kind string, readyAt int64, extra map[string]any) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		left, ready := fresh.Until(readyAt)
		e := map[string]any{"name": name, "kind": kind, "ready": ready, "readyAt": snapshot.FormatTime(readyAt)}
		if !ready {
			e["remaining"] = snapshot.FormatAge(left + time.Minute - time.Nanosecond) // rounded up to the minute
		}
		if character != "" {
			e["character"] = character
		}
		snapshot.Merge(e, extra)
		out = append(out, e)
	}
	for _, raw := range s.List("cooldowns") {
		cd := snapshot.AsMap(raw)
		if at, ok := snapshot.AsInt(cd["readyAt"]); ok {
			name, _ := cd["name"].(string)
			kind, _ := cd["kind"].(string)
			add(name, kind, at, nil)
		}
	}
	recipes := s.Map("recipes")
	for _, prof := range slices.Sorted(maps.Keys(recipes)) {
		for _, raw := range snapshot.AsList(snapshot.AsMap(recipes[prof])["recipes"]) {
			r := snapshot.AsMap(raw)
			if at, ok := snapshot.AsInt(r["readyAt"]); ok {
				name, _ := r["name"].(string)
				add(name, "profession", at, map[string]any{"profession": prof})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(snapshot.AsMap(out[i])["readyAt"]) < fmt.Sprint(snapshot.AsMap(out[j])["readyAt"])
	})
	return out
}

// ReadyNames lists the names of the cooldowns that have already passed.
func ReadyNames(cooldowns []any) []string {
	var ready []string
	for _, raw := range cooldowns {
		if cd := snapshot.AsMap(raw); cd["ready"] == true {
			ready = append(ready, fmt.Sprint(cd["name"]))
		}
	}
	return ready
}

// View shows the cooldowns of one character or (All) the whole account, with the list of ready ones.
func View(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	names := db.CharacterNames()
	if !q.All {
		who, _, err := db.Lookup(q.Character)
		if err != nil {
			return nil, err
		}
		names = []string{who}
	}
	list := []any{}
	var ready []string
	for _, name := range names {
		who := ""
		if q.All {
			who = name
		}
		cds := For(who, db.Characters[name], fresh)
		list = append(list, cds...)
		for _, label := range ReadyNames(cds) {
			if q.All {
				label = name + ": " + label
			}
			ready = append(ready, label)
		}
	}
	out := map[string]any{
		"cooldowns": list,
		"count":     len(list),
		"note":      "The list contains cooldowns longer than 90 s that were running at dump time (abilities, items like Hearthstone, recipes from the profession window). Entries with ready = true have already passed. Recipe cooldowns refresh when the profession window is opened.",
	}
	if len(ready) > 0 {
		out["readyNow"] = ready
	}
	return out, nil
}
