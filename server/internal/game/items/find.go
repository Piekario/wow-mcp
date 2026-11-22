// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// FindQuery is the input of wow_find_item.
type FindQuery struct {
	Item      string `json:"item" jsonschema:"item name (or a fragment of it) or its ID"`
	Character string `json:"character,omitempty" jsonschema:"only this character; empty = all characters on the account"`
}

// FindOnAccount searches for an item across all characters on the account at once:
// bags, equipped gear, bank and mail, with a total for each item.
func FindOnAccount(db *snapshot.Database, q FindQuery, fresh snapshot.Freshness) (map[string]any, error) {
	if strings.TrimSpace(q.Item) == "" {
		return nil, fmt.Errorf("give an item name or ID in the item parameter")
	}
	names := db.CharacterNames()
	if q.Character != "" {
		if _, _, err := db.Lookup(q.Character); err != nil {
			return nil, err
		}
		names = []string{q.Character}
	}
	match := matcher(q.Item)
	results := []any{}
	totals := map[string]int64{}
	var grand int64
	for _, name := range names {
		for _, e := range locations(db.Characters[name], match, fresh) {
			e["character"] = name
			n, _ := snapshot.AsInt(e["count"])
			totals[fmt.Sprint(e["name"])] += n
			grand += n
			results = append(results, e)
		}
	}
	out := map[string]any{
		"query":   q.Item,
		"matches": results,
		"count":   len(results),
		"totals":  totals,
		"total":   grand,
		"searched": map[string]any{
			"characters": names,
			"where":      "bags, equipment, bank (state from the last visit) and mail (state from the last time the mailbox was opened)",
		},
	}
	if len(results) == 0 {
		out["note"] = "Nothing found. The bank and mail appear in the dump only after they are opened in game and /mcp reload is run. " + snapshot.AltsHint
	}
	return out, nil
}

// locations returns all places of an item on one character: bags, bank, equipped, mail.
func locations(s snapshot.Snapshot, match func(map[string]any) bool, fresh snapshot.Freshness) []map[string]any {
	var out []map[string]any
	add := func(where string, item map[string]any, extra map[string]any) {
		e := map[string]any{"where": where, "name": item["name"], "id": item["id"], "count": snapshot.ItemCount(item)}
		snapshot.Merge(e, extra)
		out = append(out, e)
	}
	for _, rawBag := range s.Bags() {
		bag := snapshot.AsMap(rawBag)
		for _, raw := range snapshot.AsList(bag["items"]) {
			if item := snapshot.AsMap(raw); match(item) {
				add("bags", item, map[string]any{"container": bag["name"]})
			}
		}
	}
	bank := s.Map("bank")
	for _, rawC := range snapshot.AsList(bank["containers"]) {
		c := snapshot.AsMap(rawC)
		for _, raw := range snapshot.AsList(c["items"]) {
			if item := snapshot.AsMap(raw); match(item) {
				extra := map[string]any{"container": c["name"]}
				if at, ok := fresh.CapturedMeta(bank)["capturedAt"]; ok {
					extra["seenAt"] = at
				}
				add("bank", item, extra)
			}
		}
	}
	for _, raw := range s.Equipment() {
		if item := snapshot.AsMap(raw); match(item) {
			add("equipped", item, map[string]any{"slot": item["slot"]})
		}
	}
	mail := s.Map("mail")
	for _, rawM := range snapshot.AsList(mail["messages"]) {
		m := snapshot.AsMap(rawM)
		for _, raw := range snapshot.AsList(m["items"]) {
			if item := snapshot.AsMap(raw); match(item) {
				add("mail", item, map[string]any{"from": m["sender"], "subject": m["subject"]})
			}
		}
	}
	return out
}
