// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package periods

import "wowmcp/internal/snapshot"

func readCurrent(d snapshot.Snapshot, section string) (Period, bool) {
	sec := d.Map(section)
	if sec == nil {
		return Period{}, false
	}
	since, _ := snapshot.AsInt(sec["since"])
	until, _ := d.DumpedAt()
	return Period{Since: since, Until: until, Raw: sec}, true
}

func readClosed(d snapshot.Snapshot, section string) []Period {
	var out []Period
	for _, raw := range snapshot.AsList(d.Map(section)["periods"]) {
		fields := snapshot.AsMap(raw)
		since, ok1 := snapshot.AsInt(fields["since"])
		until, ok2 := snapshot.AsInt(fields["until"])
		if ok1 && ok2 && until >= since {
			out = append(out, Period{Since: since, Until: until, Carried: true, Raw: fields})
		}
	}
	return out
}
