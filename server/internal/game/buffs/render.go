// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package buffs

import (
	"fmt"
	"strings"
	"time"

	"wowmcp/internal/snapshot"
)

const assumption = "Heuristic: I warn about a missing buff only when you know the spell (spells section) and none of its group is among the auras. " +
	"I do not know whether the buff is needed right now (e.g. in combat or in town), and the auras of other players and the pet are not in the dump " +
	"visible other than as player auras. The dump shows the state at save time: after /reload it may already differ."

// View describes the character's auras and the missing class buffs.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	return Inspect(read(s), fresh).Render(s)
}

func (r Report) Render(s snapshot.Snapshot) map[string]any {
	out := map[string]any{"assumption": assumption}
	if !r.HasAuras {
		out["note"] = "The dump has no auras section (old addon or the section was disabled with /mcp off auras)."
		return out
	}
	if r.HasDump {
		out["dumpedAt"] = snapshot.FormatTime(r.DumpedAt)
	}
	if r.HasAge {
		out["dumpAge"] = snapshot.FormatAge(r.DumpAge)
		if r.Stale {
			out["warning"] = "The dump is old, so auras may have expired or new ones appeared; run /mcp reload."
		}
	}
	out["buffs"] = renderAuras(r.Buffs, r.HasDump)
	out["debuffs"] = renderAuras(r.Debuffs, r.HasDump)
	if !r.AnyTiming {
		out["timingNote"] = "The dump has no aura end times (the client did not give them, or the dump comes from an addon older than 0.7.0), so the time left is not given."
	}
	gaps := []any{}
	for _, g := range r.Missing {
		gaps = append(gaps, g.render())
	}
	out["missing"] = gaps
	if len(r.SelfBuffsNotActive) > 0 {
		names := make([]any, len(r.SelfBuffsNotActive))
		for i, n := range r.SelfBuffsNotActive {
			names[i] = n
		}
		out["selfBuffsNotActive"] = names
		out["selfBuffsNote"] = "Spells the client flags as self buffs that you know and that have no active aura with the same spell ID or icon. Matched by IDs and icons, so it works in any game language, but it does not know which buffs exclude each other (e.g. only one Aspect at a time), so not every entry is a real gap."
	}
	s.AddEnglishOnlyNote(out, "Missing class buff groups (the \"missing\" list) are matched by English spell names (e.g. Aspect of the ..., Arcane Intellect), because no language-independent spell grouping is in the dump; the selfBuffsNotActive list uses spell IDs and icons instead")
	return out
}

func renderAuras(auras []AuraStatus, hasDump bool) []any {
	out := []any{}
	for _, a := range auras {
		out = append(out, a.render(hasDump))
	}
	return out
}

func (a AuraStatus) render(hasDump bool) map[string]any {
	entry := map[string]any{"name": a.Name}
	if a.HasID {
		entry["id"] = a.ID
	}
	if a.HasStacks {
		entry["stacks"] = a.Stacks
	}
	t := a.Timing
	if t == nil {
		return entry
	}
	entry["expiresAt"] = snapshot.FormatTime(t.ExpiresAt)
	if t.HasLeftAtDump {
		entry["leftAtDump"] = snapshot.FormatSeconds(t.LeftAtDump)
	}
	if t.HasLeft {
		if t.Expired {
			entry["expired"] = true
		} else {
			entry["leftNow"] = snapshot.FormatAge(t.Left.Round(time.Second))
		}
	}
	return entry
}

func (g Gap) render() map[string]any {
	text := ""
	switch g.Kind {
	case GapGroup:
		text = fmt.Sprintf("No active buff from the %s group, and you know: %s.", g.Group, strings.Join(g.Spells, ", "))
	case GapNoPet:
		text = "No pet with the character, and you know Call Pet."
	case GapDeadPet:
		text = "The pet is dead, and you know Revive Pet."
	}
	return map[string]any{"group": g.Group, "knownSpells": g.Spells, "text": text}
}
