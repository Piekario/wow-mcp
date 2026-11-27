// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package lookup

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

const neverGuess = "This is everything the dump knows. Describe the item or spell only from these fields: World of Warcraft: Forever differs from other game versions, so do not fill gaps from memory."

const noAddonHint = "The dump has no lookups section: it needs addon 0.21.0 or newer and /mcp reload."

// Render builds the wow_lookup answer.
func (r Result) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"query": r.Query, "found": r.Found()}
	if r.Kind != Any {
		out["kind"] = string(r.Kind)
	}
	saved := make([]any, len(r.Saved))
	for i, e := range r.Saved {
		saved[i] = e.render(fresh)
	}
	out["saved"] = saved
	groups := make([]any, len(r.Elsewhere))
	for i, g := range r.Elsewhere {
		groups[i] = g.Render()
	}
	out["elsewhere"] = groups
	if r.Found() {
		out["note"] = neverGuess
	}
	if hint := r.hint(); hint != "" {
		out["ask"] = hint
	}
	if !r.HasLookups {
		out["addonNote"] = noAddonHint
	}
	return out
}

// hint tells the assistant what to ask the player when the dump holds no in-game description.
func (r Result) hint() string {
	if r.HasDescription() {
		return ""
	}
	commands := strings.Join(r.Commands(), " or ")
	if !r.Found() {
		return "Nothing in the dump for this query. Do not answer from memory. Ask the player to type " + commands +
			" in game (an exact name works only for things the client has seen; an ID or a shift-clicked link always works), then /mcp reload, and ask again."
	}
	return "The dump only mentions this by name or ID and holds no full in-game description of it. For the real text ask the player to type " +
		commands + " in game, then /mcp reload."
}

func (e Entry) render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{"kind": string(e.Kind), "id": e.ID, "name": e.Name, "query": e.Query}
	if e.At > 0 {
		out["savedAgo"] = snapshot.FormatAge(fresh.Age(e.At))
	}
	if e.Kind == Spell {
		e.renderSpell(out)
	} else {
		e.renderItem(out)
	}
	if len(e.Tooltip) > 0 {
		out["tooltip"] = e.Tooltip
	}
	return out
}

func (e Entry) renderSpell(out map[string]any) {
	if e.Rank != "" {
		out["rank"] = e.Rank
	}
	if e.Description != "" {
		out["description"] = e.Description
	} else if e.DescriptionMissing {
		out["descriptionMissing"] = "The client had not loaded the description. Ask the player to repeat /mcp spell " + fmt.Sprint(e.ID) + " a moment later, then /mcp reload."
	}
	if e.HasCastTime {
		out["castTime"] = castTime(e.CastTimeMs)
	}
	if costs := e.costText(); len(costs) > 0 {
		out["cost"] = costs
	}
	if e.MaxRange > 0 {
		out["range"] = fmt.Sprintf("%d-%d yards", e.MinRange, e.MaxRange)
	}
}

func castTime(ms int64) string {
	if ms <= 0 {
		return "instant"
	}
	return fmt.Sprintf("%.1f sec", float64(ms)/1000)
}

func (e Entry) costText() []string {
	var out []string
	for _, c := range e.Costs {
		out = append(out, fmt.Sprintf("%d %s", c.Amount, strings.ToLower(c.Power)))
	}
	return out
}

func (e Entry) renderItem(out map[string]any) {
	if name, ok := itemkind.QualityName(e.Quality); ok && e.HasQuality {
		out["quality"] = name
	}
	if e.Type != "" {
		out["type"] = strings.TrimSpace(e.Type + " " + e.SubType)
	}
	if e.EquipLoc != "" {
		out["equipLoc"] = e.EquipLoc
	}
	if e.ItemLevel > 0 {
		out["itemLevel"] = e.ItemLevel
	}
	if e.RequiredLevel > 0 {
		out["requiredLevel"] = e.RequiredLevel
	}
	if e.SellPrice > 0 {
		out["sellPrice"] = snapshot.Money(e.SellPrice).String()
	}
	if len(e.Stats) > 0 {
		out["stats"] = sortedStats(e.Stats)
	}
}

func sortedStats(stats map[string]float64) map[string]any {
	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make(map[string]any, len(names))
	for _, name := range names {
		out[name] = stats[name]
	}
	return out
}
