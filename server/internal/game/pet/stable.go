// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"fmt"
	"sort"

	"wowmcp/internal/snapshot"
)

// stableReview compares the stabled pets with the active one (level and family). The addon saves for stabled pets
// only the slot, name, level, family and specialization, so there is no data on abilities or fit.
func stableReview(s snapshot.Snapshot, active map[string]any, hasActive bool, fresh snapshot.Freshness) map[string]any {
	stable := s.Map("stable")
	if stable == nil {
		return map[string]any{"note": "no data: open the stable window (Stable Master) in game and run /mcp reload"}
	}
	activeLevel, _ := snapshot.AsInt(active["level"])
	activeFamily, _ := active["family"].(string)

	pets := []any{}
	var higher []string
	for _, raw := range snapshot.AsList(stable["pets"]) {
		p := snapshot.AsMap(raw)
		if p["current"] == true {
			continue // the pet with the character (slot 0 in the classic API) is the same as the active one
		}
		// Active slots from C_StableInfo.GetActivePetList (addon 0.7.1): the one with the character is the active pet.
		if p["active"] == true && hasActive && p["name"] == active["name"] {
			continue
		}
		e := map[string]any{}
		for _, k := range []string{"slot", "name", "level", "family", "specialization", "loyalty", "active"} {
			if v, ok := p[k]; ok {
				e[k] = v
			}
		}
		if hasActive {
			cmp := map[string]any{}
			if level, ok := snapshot.AsInt(p["level"]); ok {
				cmp["levelDiff"] = level - activeLevel
				if level > activeLevel {
					higher = append(higher, fmt.Sprint(p["name"]))
				}
			}
			if family, _ := p["family"].(string); family != "" && activeFamily != "" {
				cmp["sameFamily"] = family == activeFamily
			}
			e["vsActive"] = cmp
		}
		pets = append(pets, e)
	}
	sort.SliceStable(pets, func(i, j int) bool {
		li, _ := snapshot.AsInt(snapshot.AsMap(pets[i])["level"])
		lj, _ := snapshot.AsInt(snapshot.AsMap(pets[j])["level"])
		return li > lj
	})
	out := map[string]any{"pets": pets, "count": len(pets)}
	if slots, ok := snapshot.AsInt(stable["slots"]); ok {
		out["slots"] = slots
	}
	snapshot.Merge(out, fresh.CapturedMeta(stable))
	if hasActive {
		out["active"] = map[string]any{"name": active["name"], "level": activeLevel, "family": activeFamily}
		if len(higher) > 0 {
			out["higherLevelThanActive"] = higher
		}
	} else {
		out["note"] = "there is no pet with the character right now, so there is nothing to compare levels with"
	}
	out["limits"] = "the addon saves for stabled pets only the slot, name, level, family and specialization; there is no data on their abilities, loyalty and diet"
	return out
}

// Review collects the pet description extras: ability ranks (rankReview), the training points plan and the stable comparison.
// Sections whose data is not in the dump say what to open in game. For a character without an active pet it returns only the stable.
func Review(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{}
	p := s.Map("pet")
	active := Exists(s)
	if active {
		out["rankReview"] = rankReview(s, p)
		out["trainingPlan"] = trainingPlan(s, p)
	}
	if s.Map("stable") != nil || active {
		out["stable"] = stableReview(s, p, active, fresh)
	}
	return out
}

// Summarize adds Review to the wow_pet response.
func Summarize(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	snapshot.Merge(out, Review(s, fresh))
}
