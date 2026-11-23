// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// BuybackSection is the buyback list from the last vendor window. The client keeps it only until logout.
type BuybackSection struct {
	Npc      any
	Items    []Item
	Captured snapshot.Captured
	At       int64 // capturedAt read as any number, for the age of the list
	HasAt    bool
}

type BuybackLine struct {
	Item
	CanAfford bool
}

type BuybackReport struct {
	BuybackSection
	Lines []BuybackLine
	Total int64
}

// InspectBuyback narrows the buyback list by item name and judges each price against the character's money;
// false when the dump has no buyback section.
func InspectBuyback(s snapshot.Snapshot, query string) (BuybackReport, bool) {
	sec, ok := readBuyback(s)
	if !ok {
		return BuybackReport{}, false
	}
	_, money := s.LevelAndMoney()
	needle := strings.ToLower(query)
	r := BuybackReport{BuybackSection: sec}
	for _, it := range sec.Items {
		if needle != "" && !strings.Contains(strings.ToLower(it.Name), needle) {
			continue
		}
		line := BuybackLine{Item: it}
		if it.HasPrice {
			line.CanAfford = it.Price <= money
			r.Total += it.Price
		}
		r.Lines = append(r.Lines, line)
	}
	return r, true
}

// soldBy says whether the list came from the NPC whose name contains the fragment (case-insensitive).
func (b BuybackSection) soldBy(fragment string) bool {
	name, _ := b.Npc.(string)
	return strings.Contains(strings.ToLower(name), strings.ToLower(fragment))
}
