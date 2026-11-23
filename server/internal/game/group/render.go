// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package group

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// View describes the group composition with warnings or, with the player parameter, the full Inspect of one player.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	if q.Player != "" {
		player, err := InspectionOf(s, q.Player, fresh)
		if err != nil {
			return nil, err
		}
		out := map[string]any{"player": player.Raw}
		snapshot.Merge(out, renderInspectSummary(player))
		return out, nil
	}
	return Analyse(s, fresh).Render(), nil
}

func (r Report) Render() map[string]any {
	out := map[string]any{}
	if !r.InGroup {
		out["note"] = "The character was not in a group at dump time. Join a party or raid and type /mcp reload."
	} else {
		renderComposition(r.Composition, out)
	}
	if r.HasItemLevel {
		out["myAverageItemLevel"] = r.ItemLevel
	}
	if len(r.Inspected) > 0 {
		list := []any{}
		for _, i := range r.Inspected {
			e := map[string]any{"name": i.Name, "level": i.Level, "class": i.Class}
			snapshot.Merge(e, renderInspectSummary(i))
			list = append(list, e)
		}
		out["inspected"] = list
		out["inspectedNote"] = "A player's full gear: wow_group with the player parameter."
	}
	return out
}

func renderInspectSummary(i Inspection) map[string]any {
	out := map[string]any{}
	if i.HasAverage {
		out["averageItemLevel"] = i.AverageItemLevel
	}
	if i.HasCapturedAt {
		out["inspectedAgo"] = snapshot.FormatAge(i.Age)
	}
	if i.HasGuild {
		out["guild"] = i.Guild
	}
	return out
}

func renderComposition(c Composition, out map[string]any) {
	members := []any{}
	for _, m := range c.Members {
		members = append(members, renderMember(m))
	}
	out["type"] = c.Party.Type
	out["size"] = len(members)
	out["members"] = members
	classes := map[string]int{}
	for class, n := range c.Classes {
		classes[class] = n
	}
	out["classes"] = classes
	if len(c.Roles) > 0 {
		roles := map[string]int{}
		for role, n := range c.Roles {
			roles[string(role)] = n
		}
		out["roles"] = roles
	}
	if c.Levels != nil {
		out["levels"] = map[string]any{"min": c.Levels.Min, "max": c.Levels.Max, "average": c.Levels.Average}
	}
	if len(c.Problems) > 0 {
		warnings := make([]string, len(c.Problems))
		for i, p := range c.Problems {
			warnings[i] = p.Text()
		}
		out["warnings"] = warnings
	}
	if c.Party.HasLootMethod {
		out["lootMethod"] = c.Party.LootMethod
	}
	if c.Party.HasLootThreshold {
		out["lootThreshold"] = c.Party.LootThreshold
	}
}

func renderMember(m MemberStatus) map[string]any {
	e := map[string]any{}
	for k, v := range snapshot.AsMap(m.Raw) {
		e[k] = v
	}
	if m.Role != "" {
		e["role"] = string(m.Role)
	}
	if m.Inspection != nil {
		e["inspect"] = renderInspectSummary(*m.Inspection)
	}
	return e
}

func (p Problem) Text() string {
	switch p.Kind {
	case MemberOffline:
		return p.Member + " is offline"
	case MemberDead:
		return p.Member + " is dead"
	case MemberOutOfRange:
		return p.Member + " is out of range"
	case LevelGap:
		return fmt.Sprintf("large level difference (%d-%d)", p.Min, p.Max)
	case NoTank:
		return "nobody has the tank role"
	case NoHealer:
		return "nobody has the healer role"
	}
	return ""
}
