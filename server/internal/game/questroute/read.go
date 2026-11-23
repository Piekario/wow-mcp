// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questroute

import "wowmcp/internal/snapshot"

// readPosition is the zone the character stands in and its map ID (0 when the dump has none).
func readPosition(s snapshot.Snapshot) (zone string, mapID int64) {
	c := s.Character()
	zone, _ = c["zone"].(string)
	mapID, _ = snapshot.AsInt(c["mapID"])
	return zone, mapID
}
