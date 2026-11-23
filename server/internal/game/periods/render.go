// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package periods

// Describe adds to out how many carried periods were merged (nothing when none was).
func (m Merge) Describe(out map[string]any) {
	if m.Carried == 0 || out == nil {
		return
	}
	out["periodsMerged"] = m.Carried
	out["periodsNote"] = "includes periods closed by earlier saves to disk that the server never read (carried by the addon in the section's periods list)"
}
