// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package character is the character sheet: gold, play time, durability and average
// gear level, addon version and the overview of all characters on the account.
package character

import (
	"wowmcp/internal/snapshot"
)

// SummarizeSheet adds gold, play time, the player's note and deaths to the wow_character response.
func SummarizeSheet(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	if c, ok := s["character"].(map[string]any); ok {
		if copper, ok := c["moneyCopper"].(int64); ok {
			out["money"] = snapshot.Money(copper).String()
		}
	}
	if played := Played(s); played != nil {
		out["playedTime"] = played
	}
	if note := Note(s, fresh); note != nil {
		out["note"] = note
	}
	if deaths := Deaths(s, fresh); deaths != nil {
		out["deaths"] = deaths
	}
}

// SummarizeEquipment adds a durability summary to the wow_equipment response.
func SummarizeEquipment(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	if d := Durability(s, fresh); d != nil {
		out["durability"] = d
	}
}

// Played formats the played section (from /played) into a readable form.
func Played(s snapshot.Snapshot) map[string]any {
	p := s.Map("played")
	total, ok := snapshot.AsInt(p["totalSeconds"])
	if !ok {
		return nil
	}
	out := map[string]any{"total": snapshot.FormatSeconds(total), "totalSeconds": total}
	if lvl, ok := snapshot.AsInt(p["levelSeconds"]); ok {
		out["thisLevel"] = snapshot.FormatSeconds(lvl)
	}
	return out
}

// AverageItemLevel computes the character's average item level (excluding shirt, tabard and ammo).
func AverageItemLevel(s snapshot.Snapshot) (float64, bool) {
	var sum, n float64
	for _, raw := range s.Equipment() {
		item := snapshot.AsMap(raw)
		slot, _ := item["slot"].(string)
		if slot == "shirt" || slot == "tabard" || slot == "ammo" {
			continue
		}
		if lvl, ok := snapshot.AsFloat(item["itemLevel"]); ok {
			sum += lvl
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return snapshot.Round1(sum / n), true
}

// EquipmentNames is a short list of equipped items: slot -> name (item level).
func EquipmentNames(s snapshot.Snapshot) map[string]any {
	out := map[string]any{}
	for _, raw := range s.Equipment() {
		item := snapshot.AsMap(raw)
		slot, _ := item["slot"].(string)
		if slot == "" {
			continue
		}
		if lvl, ok := snapshot.AsInt(item["itemLevel"]); ok {
			out[slot] = map[string]any{"name": item["name"], "itemLevel": lvl}
		} else {
			out[slot] = item["name"]
		}
	}
	return out
}
