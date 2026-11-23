// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import "wowmcp/internal/snapshot"

// Container is one bag or bank bag. Name is passed through as the client gave it.
type Container struct {
	Name       any
	Size, Free int64
	Items      []Item
}

type BagSlots struct{ Total, Free int64 }

func slotsOf(containers []Container) BagSlots {
	var b BagSlots
	for _, c := range containers {
		b.Total += c.Size
		b.Free += c.Free
	}
	return b
}

// Space is the total of bag slots.
func Space(s snapshot.Snapshot) BagSlots { return slotsOf(readBags(s)) }

// BagContents is what wow_bags adds to the bags: slot totals, Poor quality junk and the equipped ammo.
type BagContents struct {
	BagSlots
	Junk      []Item
	JunkValue int64
	Ammo      *Item
}

func InspectBags(s snapshot.Snapshot) BagContents {
	bags := readBags(s)
	r := BagContents{BagSlots: slotsOf(bags)}
	for _, bag := range bags {
		for _, it := range bag.Items {
			if it.IsJunk() {
				r.Junk = append(r.Junk, it)
				r.JunkValue += it.SellValue()
			}
		}
	}
	for _, it := range readEquipment(s) {
		if it.HasSlot && it.Slot == "ammo" {
			r.Ammo = &it
		}
	}
	return r
}
