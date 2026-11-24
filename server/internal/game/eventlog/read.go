// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package eventlog

import "wowmcp/internal/snapshot"

// Read returns the entries of the eventLog section, oldest first as the addon wrote them. Entries without a time or
// a kind are skipped.
func Read(s snapshot.Snapshot) []Entry {
	var entries []Entry
	for _, raw := range snapshot.AsList(s.Map("eventLog")["entries"]) {
		m := snapshot.AsMap(raw)
		e := Entry{}
		e.At, _ = snapshot.AsInt(m["at"])
		e.Kind, _ = m["kind"].(string)
		if e.At == 0 || e.Kind == "" {
			continue
		}
		e.Level, _ = snapshot.AsInt(m["level"])
		e.Copper, _ = snapshot.AsInt(m["copper"])
		e.QuestID, _ = snapshot.AsInt(m["questID"])
		e.Title, _ = m["title"].(string)
		e.XP, _ = snapshot.AsInt(m["xp"])
		e.Zone, _ = m["zone"].(string)
		e.Slot, _ = m["slot"].(string)
		e.ItemID, _ = snapshot.AsInt(m["itemID"])
		entries = append(entries, e)
	}
	return entries
}
