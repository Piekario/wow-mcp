// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package loot

import "fmt"

// Describe describes loot for the response: counts, notable entries (up to maxListed) and a note about unknown quality.
// nil when there was no loot at all.
func Describe(items []Item, maxListed int) map[string]any {
	sum := Summarise(items, maxListed)
	if sum == nil {
		return nil
	}
	out := map[string]any{"distinctItems": sum.Distinct, "pieces": sum.Pieces}
	if len(sum.Notable) > 0 {
		labels := make([]string, len(sum.Notable))
		for i, it := range sum.Notable {
			labels[i] = it.Label()
		}
		out["notable"] = labels
		if sum.More > 0 {
			out["more"] = sum.More
		}
	}
	if sum.UnknownQuality > 0 {
		out["unknownQuality"] = sum.UnknownQuality
		out["qualityNote"] = fmt.Sprintf("the addon gave no quality for %d items, so they are not included in the notable list", sum.UnknownQuality)
	}
	return out
}
