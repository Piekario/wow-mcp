// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package consumption

import "wowmcp/internal/snapshot"

// Describe describes usage in the response; how explains what a loss means for the given kind of thing,
// and unchanged is the note used when nothing decreased within the stretch.
func (u Usage) Describe(how, unchanged string) map[string]any {
	out := map[string]any{
		"window": u.Window, "basis": u.Basis, "snapshots": u.Snapshots,
		"used": u.Used, "playHours": snapshot.Round1(u.Hours), "perHour": snapshot.Round1(u.PerHour),
		"how": how,
	}
	if u.PerHour == 0 {
		out["note"] = unchanged
	}
	return out
}
