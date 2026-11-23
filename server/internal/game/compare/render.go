// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package compare

import (
	"fmt"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// View puts two characters of the account side by side for wow_compare_characters.
func View(db *snapshot.Database, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	report, err := Compare(db, q, fresh)
	if err != nil {
		return nil, err
	}
	return report.Render(), nil
}

func (r Report) Render() map[string]any {
	slots := make([]any, 0, len(r.Slots))
	for _, c := range r.Slots {
		slots = append(slots, renderSlot(c))
	}
	out := map[string]any{
		"characters": []any{renderCard(r.A), renderCard(r.B)},
		"slots":      slots,
		"summary":    renderEdges(r.Edges),
	}
	if note := r.Skew.note(); note != "" {
		out["dataNote"] = note
	}
	out["note"] = "Average ilvl is computed from equipped items excluding shirt, tabard and ammo. Each character shows data from its own latest dump (ageMinutes)."
	return out
}

func renderCard(c Card) map[string]any {
	var professionLabels []string
	for _, p := range c.Professions {
		professionLabels = append(professionLabels, p.Label())
	}
	var dumpedAt, ageMinutes any
	if c.HasDumpedAtText {
		dumpedAt = c.DumpedAtText
	}
	if c.HasAge {
		ageMinutes = c.AgeMinutes
	}
	e := map[string]any{
		"name": c.Name, "level": c.Level, "class": c.Class, "race": c.Race, "faction": c.Faction,
		"zone": c.Zone, "subZone": c.SubZone,
		"money": snapshot.Money(c.Money).String(), "moneyCopper": c.Money,
		"professions": professionLabels,
		"dumpedAt":    dumpedAt, "ageMinutes": ageMinutes,
	}
	if c.Stale {
		e["stale"] = true
	}
	if c.HasRestedXP {
		e["restedXp"] = c.RestedXP
	}
	if c.HasPlayed {
		e["played"] = snapshot.FormatSeconds(c.PlayedSeconds)
		e["playedSeconds"] = c.PlayedSeconds
	}
	if c.GearPieces > 0 {
		e["avgItemLevel"] = c.AverageItemLevel
		e["gearPieces"] = c.GearPieces
	}
	return e
}

func renderSlot(c SlotComparison) map[string]any {
	e := map[string]any{"slot": c.Slot, "a": renderPiece(c.A), "b": renderPiece(c.B)}
	if c.Compared {
		e["better"] = string(c.Better)
	}
	return e
}

// renderPiece describes the item in a slot; nil for an empty slot.
func renderPiece(p *Piece) map[string]any {
	if p == nil {
		return nil
	}
	e := map[string]any{"name": p.Name}
	if p.HasItemLevel {
		e["itemLevel"] = p.ItemLevel
	}
	if p.HasQuality {
		if name, known := itemkind.QualityName(p.Quality); known {
			e["quality"] = name
		}
	}
	return e
}

func renderEdges(edges []Edge) []string {
	var out []string
	for _, e := range edges {
		switch e.Kind {
		case HigherLevel:
			out = append(out, fmt.Sprintf("Higher level: %s (by %d over %s).", e.Leader, e.Gap, e.Other))
		case MoreGold:
			out = append(out, fmt.Sprintf("More gold: %s (difference %s).", e.Leader, snapshot.Money(e.Gap)))
		case HigherItemLevel:
			out = append(out, fmt.Sprintf("Higher average ilvl: %s (%.1f vs %.1f).", e.Leader, e.High, e.Low))
		}
	}
	return out
}

// note warns when the two characters' dumps come from different moments.
func (k Skew) note() string {
	switch {
	case !k.Known:
		return "One of the dumps has no save time, so it is unknown how far apart the data is."
	case !k.Unfair():
		return ""
	}
	return fmt.Sprintf("The dumps come from different moments (difference %s): the data of %s is older. To make the comparison fair, log in on both characters and type /mcp reload.", snapshot.FormatSeconds(k.GapSeconds), k.Older)
}
