// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package timeline

import (
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// load reads the newest history dumps (oldest first) and appends the current one if it is newer than the last.
func load(name string, current snapshot.Snapshot, h progress.History) ([]snapshot.Snapshot, error) {
	times, err := h.List(name)
	if err != nil {
		return nil, err
	}
	if len(times) > maxSnapshots {
		times = times[len(times)-maxSnapshots:]
	}
	var snaps []snapshot.Snapshot
	for _, at := range times {
		if s, err := h.Load(name, at); err == nil { // a corrupt history file does not spoil the timeline
			snaps = append(snaps, s)
		}
	}
	cur, ok := current.DumpedAt()
	// A dump from an empty client (exit from the game) does not describe the character: we do not append it.
	if last := len(snaps) - 1; ok && !current.IsUnloadedDump() && (last < 0 || dumpedAt(snaps[last]) < cur) {
		snaps = append(snaps, current)
	}
	return snaps, nil
}

func dumpedAt(s snapshot.Snapshot) int64 {
	at, _ := s.DumpedAt()
	return at
}

// recentDeath keeps zone, subzone and level exactly as the addon saved them.
type recentDeath struct {
	At                            int64
	HasAt                         bool
	Zone, SubZone, Level          any
	HasZone, HasSubZone, HasLevel bool
}

func readRecentDeaths(s snapshot.Snapshot) []recentDeath {
	var out []recentDeath
	for _, raw := range snapshot.AsList(s.Map("deaths")["recent"]) {
		r := snapshot.AsMap(raw)
		d := recentDeath{}
		d.At, d.HasAt = snapshot.AsInt(r["at"])
		d.Zone, d.HasZone = r["zone"]
		d.SubZone, d.HasSubZone = r["subZone"]
		d.Level, d.HasLevel = r["level"]
		out = append(out, d)
	}
	return out
}
