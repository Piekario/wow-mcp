// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package achievements

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// RewardEntry is an achievement whose reward text matched, with its category as "Parent > Child".
type RewardEntry struct {
	Achievement
	Category string
	// Missing lists the criteria still to do for an incomplete achievement; MoreMissing counts those not listed.
	Missing     []string
	MoreMissing int64
}

// WithReward lists the achievements of the dump whose reward text contains fragment (case-insensitive), in the dump's
// order. found is false when the dump has no achievements section. Other contexts use it to read rewards (for
// example the Legacy Points of wow_legacy).
func WithReward(s snapshot.Snapshot, fragment string) (entries []RewardEntry, found bool) {
	section, found := readAchievements(s)
	if !found {
		return nil, false
	}
	catalog := newCatalog(section.Categories)
	fragment = strings.ToLower(fragment)
	for _, a := range section.List {
		if a.Reward == "" || !strings.Contains(strings.ToLower(a.Reward), fragment) {
			continue
		}
		entry := RewardEntry{Achievement: a, Category: catalog.Path(a.CategoryID)}
		if !a.Completed {
			entry.Missing, entry.MoreMissing = a.Missing()
		}
		entries = append(entries, entry)
	}
	return entries, true
}
