// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package character

import (
	"wowmcp/internal/snapshot"
)

// DeathCount is the character's death counter; false when the dump has no deaths section (old addon or disabled).
func DeathCount(s snapshot.Snapshot) (int64, bool) {
	return snapshot.AsInt(s.Map("deaths")["count"])
}

// Deaths describes the character's deaths: the counter and the last death (zone, level, how long ago). nil without a deaths section.
func Deaths(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	count, ok := DeathCount(s)
	if !ok {
		return nil
	}
	out := map[string]any{"count": count}
	recent := snapshot.AsList(s.Map("deaths")["recent"])
	if len(recent) == 0 {
		return out
	}
	out["last"] = deathPlace(snapshot.AsMap(recent[len(recent)-1]), fresh)
	return out
}

// deathPlace describes one death: zone, subzone, level and the time elapsed.
func deathPlace(d map[string]any, fresh snapshot.Freshness) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"zone", "subZone", "level"} {
		if v, ok := d[key]; ok {
			out[key] = v
		}
	}
	if at, ok := snapshot.AsInt(d["at"]); ok {
		out["at"] = snapshot.FormatTime(at)
		out["ago"] = snapshot.FormatAge(fresh.Age(at)) + " ago"
	}
	return out
}
