// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package ammo covers ammunition (arrows and bullets): how much is in the equipped slot and in bags,
// how much is used per hour of play (from dump history) and how much to buy before heading out.
package ammo

import (
	"fmt"
	"math"
	"sort"

	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_ammo.
type Query struct {
	Character string  `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Hours     float64 `json:"hours,omitempty" jsonschema:"how many hours of play to plan the ammo purchase for; default 2"`
}

// defaultHours is the default play time the ammo purchase is planned for.
const defaultHours = 2

// Stock is the supply of one kind of ammo.
type Stock struct {
	Name     string
	Equipped int64 // in the ammo slot
	InBags   int64
}

func (s Stock) Total() int64 { return s.Equipped + s.InBags }

// bagAmmo counts ammo in bags by name.
func bagAmmo(s snapshot.Snapshot) map[string]int64 {
	out := map[string]int64{}
	for _, held := range items.StockOf(s, func(it items.Item) bool { return it.IsAmmo() }) {
		if held.Bags > 0 {
			out[held.Item.Name] += held.Bags
		}
	}
	return out
}

// MainStock picks the main ammo kind: the equipped one, or the most numerous in bags when the slot is empty.
// The second result is the other kinds from bags. false when the character has no ammo at all.
func MainStock(s snapshot.Snapshot) (main Stock, others []Stock, ok bool) {
	inBags := bagAmmo(s)
	if eq := items.InspectBags(s).Ammo; eq != nil {
		main = Stock{Name: eq.Name, Equipped: eq.Count}
	} else if name, found := mostNumerous(inBags); found {
		main = Stock{Name: name}
	} else {
		return Stock{}, nil, false
	}
	main.InBags = inBags[main.Name]
	delete(inBags, main.Name)
	for name, n := range inBags {
		others = append(others, Stock{Name: name, InBags: n})
	}
	sort.Slice(others, func(i, j int) bool { return others[i].Name < others[j].Name })
	return main, others, true
}

func mostNumerous(counts map[string]int64) (string, bool) {
	best, found := "", false
	for name, n := range counts {
		if !found || n > counts[best] || (n == counts[best] && name < best) {
			best, found = name, true
		}
	}
	return best, found
}

// countOf counts ammo of a given kind in the dump (ammo slot and bags).
func countOf(s snapshot.Snapshot, name string) int64 {
	var n int64
	if eq := items.InspectBags(s).Ammo; eq != nil && eq.Name == name {
		n += eq.Count
	}
	return n + bagAmmo(s)[name]
}

// VendorOffer is the last vendor window's offer of the ammo; Stack is shown as the dump has it.
type VendorOffer struct {
	Npc, Stack any
}

// PurchasePlan is how much to buy to last Hours hours of play.
type PurchasePlan struct {
	Hours               float64
	Needed, Have, ToBuy int64
	Vendor              *VendorOffer
}

// Report is the ammo stock, its usage over the history and the purchase plan. Present is false when the
// character has no ammo at all. UseErr says why there is no usage (history disabled or too short); HoursLeft and
// Plan exist only when ammo is actually being used up.
type Report struct {
	Present      bool
	Main         Stock
	Others       []Stock
	SlotEmpty    bool
	Use          consumption.Usage
	UseErr       string
	HoursLeft    float64
	HasHoursLeft bool
	Plan         PurchasePlan
}

// Inspect measures the ammo stock and plans a purchase for q.Hours hours of play (default 2).
func Inspect(name string, s snapshot.Snapshot, tr progress.Tracker, q Query) (Report, error) {
	hours := q.Hours
	switch {
	case hours == 0:
		hours = defaultHours
	case hours < 0 || math.IsNaN(hours):
		return Report{}, fmt.Errorf("hours must be positive (default %d), got %g", defaultHours, q.Hours)
	}
	main, others, ok := MainStock(s)
	if !ok {
		return Report{}, nil
	}
	r := Report{Present: true, Main: main, Others: others, SlotEmpty: items.InspectBags(s).Ammo == nil}
	use, err := consumption.Over(tr, name, s, func(snap snapshot.Snapshot) int64 { return countOf(snap, main.Name) })
	if err != nil {
		r.UseErr = err.Error()
		return r, nil
	}
	r.Use = use
	if left, ok := use.HoursLeft(main.Total()); ok {
		r.HoursLeft, r.HasHoursLeft = left, true
		r.Plan = purchasePlan(main, use.PerHour, hours, s)
	}
	return r, nil
}

// purchasePlan computes how much to buy to last hours hours of play; adds the stack size if the last vendor sells this ammo.
func purchasePlan(main Stock, perHour, hours float64, s snapshot.Snapshot) PurchasePlan {
	needed := int64(math.Ceil(perHour * hours))
	plan := PurchasePlan{Hours: hours, Needed: needed, Have: main.Total(), ToBuy: max(needed-main.Total(), 0)}
	if offer, ok := readVendorOffer(s, main.Name); ok {
		plan.Vendor = &offer
	}
	return plan
}

// Forecast is the ammo supply forecast: how much there is and, when history allows, how many hours of play it lasts.
type Forecast struct {
	Present   bool // the character has any ammo at all
	Stock     Stock
	RateKnown bool // the hourly usage is known
	PerHour   float64
	HoursLeft float64 // meaningful only when RateKnown and usage is above zero
	Burning   bool    // ammo is actually being used up (PerHour > 0)
	Note      string  // why there is no rate (history too short, history disabled)
}

// Left computes the forecast of the main ammo stock from dump history (the same window as wow_ammo).
func Left(name string, s snapshot.Snapshot, tr progress.Tracker) Forecast {
	main, _, ok := MainStock(s)
	if !ok {
		return Forecast{}
	}
	f := Forecast{Present: true, Stock: main}
	use, err := consumption.Over(tr, name, s, func(snap snapshot.Snapshot) int64 { return countOf(snap, main.Name) })
	if err != nil {
		f.Note = err.Error()
		return f
	}
	f.RateKnown, f.PerHour = true, use.PerHour
	if left, ok := use.HoursLeft(main.Total()); ok {
		f.Burning, f.HoursLeft = true, left
	}
	return f
}
