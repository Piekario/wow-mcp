// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package xpgains

import "wowmcp/internal/snapshot"

func readSource(period map[string]any, key string) Source {
	m := snapshot.AsMap(period[key])
	var s Source
	s.Count, _ = snapshot.AsInt(m["count"])
	s.XP, _ = snapshot.AsInt(m["xp"])
	return s
}

func readGains(period map[string]any) Gains {
	return Gains{Kills: readSource(period, "kills"), Quests: readSource(period, "quests"), Other: readSource(period, "other")}
}

func gainsOf(raw any) Gains { return readGains(snapshot.AsMap(raw)) }

// Section returns the XP from one dump's xpGains section and the time it counts from (since);
// false when the dump has no section (an older addon or the section disabled). Only the current period:
// periods carried from earlier saves are merged by Gained.
func Section(s snapshot.Snapshot) (g Gains, since int64, ok bool) {
	sec := s.Map("xpGains")
	if sec == nil {
		return Gains{}, 0, false
	}
	since, _ = snapshot.AsInt(sec["since"])
	return readGains(sec), since, true
}
