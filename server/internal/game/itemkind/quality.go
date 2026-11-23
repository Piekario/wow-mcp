// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package itemkind

// qualityNames are the in-game item qualities, indexed by the game's quality number (Enum.ItemQuality).
var qualityNames = []string{"Poor", "Common", "Uncommon", "Rare", "Epic", "Legendary", "Artifact", "Heirloom"}

// QualityName returns the English name of an item quality; false for a quality the game does not have.
func QualityName(q int64) (string, bool) {
	if q < 0 || int(q) >= len(qualityNames) {
		return "", false
	}
	return qualityNames[q], true
}
