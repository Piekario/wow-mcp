// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package rested

import "wowmcp/internal/snapshot"

func read(s snapshot.Snapshot) Character {
	c := s.Character()
	var ch Character
	ch.XPMax, _ = snapshot.AsInt(c["xpMax"])
	ch.Rested, ch.HasRested = snapshot.AsInt(c["restedXp"])
	ch.XP, _ = snapshot.AsInt(c["xp"])
	ch.Level, _ = snapshot.AsInt(c["level"])
	ch.Resting, ch.HasResting = c["resting"].(bool)
	ch.BindLocation, _ = c["bindLocation"].(string)
	return ch
}
