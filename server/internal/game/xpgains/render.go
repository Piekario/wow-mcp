// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package xpgains

// Describe describes the XP for the response; nil when nothing was gained (empty sources are skipped).
func Describe(g Gains) map[string]any {
	if g.Empty() {
		return nil
	}
	out := map[string]any{"total": g.Total()}
	for _, e := range []struct {
		key string
		src Source
	}{{"kills", g.Kills}, {"quests", g.Quests}, {"other", g.Other}} {
		if e.src.Count > 0 || e.src.XP > 0 {
			out[e.key] = map[string]any{"count": e.src.Count, "xp": e.src.XP}
		}
	}
	return out
}
