// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package bagplan suggests what to do with full bags: keep, move to the bank, sell to a
// vendor or list on the auction house. It combines ready assessments from other contexts (quests,
// recipes, auctions, pet, equipment); it removes nothing and never advises destroying items.
package bagplan

import (
	"sort"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// Recommended actions. There is deliberately no "destroy" or "discard" here.
const (
	Keep    = "keep"
	Bank    = "bank"
	Sell    = "sell"
	Auction = "auction"
)

// actionOrder is the order in the answer: first what frees space for money.
var actionOrder = []string{Sell, Auction, Bank, Keep}

// Stack is the bag items with one name, summed. Item is the first instance.
type Stack struct {
	Name   string
	Count  int64
	Slots  int64
	Sell   int64 // vendor price per unit
	Vendor int64 // vendor value for all units
	Item   items.Item
}

// Suggestion is the decision for one item.
type Suggestion struct {
	Stack
	Action, Reason string
	AuctionNet     int64 // value at auction after the cut (for Auction)
}

// Total sums the suggestions of one action.
type Total struct {
	Items, Slots, Vendor, Auction int64
}

// BankSpace is what the last bank visit left in the dump.
type BankSpace struct {
	Saved     bool
	FreeSlots int64
}

// Plan is every bag stack with its action, most useful actions first.
type Plan struct {
	Space        items.BagSlots
	Suggestions  []Suggestion
	Totals       map[string]*Total
	Bank         BankSpace
	BindingKnown bool
}

// SlotsFreed is what selling, auctioning and banking would empty.
func (p Plan) SlotsFreed() int64 { return p.slots(Sell) + p.slots(Auction) + p.slots(Bank) }

func (p Plan) slots(action string) int64 {
	if t := p.Totals[action]; t != nil {
		return t.Slots
	}
	return 0
}

// FreeAfter is the free bag slots once the plan is carried out.
func (p Plan) FreeAfter() int64 { return p.Space.Free + p.SlotsFreed() }

func (t Total) fitsBank(b BankSpace) bool { return b.FreeSlots >= t.Slots }

// Analyse assesses every item in the bags.
func Analyse(s snapshot.Snapshot, fresh snapshot.Freshness) Plan {
	bags := readBags(s)
	plan := Plan{Space: bags.Space, Bank: readBank(s), BindingKnown: bags.BindingKnown, Totals: map[string]*Total{}}
	if len(bags.Stacks) == 0 {
		return plan
	}
	c := readContext(s, fresh)
	for _, st := range bags.Stacks {
		plan.Suggestions = append(plan.Suggestions, c.decide(st))
	}
	sort.SliceStable(plan.Suggestions, func(i, j int) bool { return rank(plan.Suggestions[i].Action) < rank(plan.Suggestions[j].Action) })
	for _, sg := range plan.Suggestions {
		t := plan.Totals[sg.Action]
		if t == nil {
			t = &Total{}
			plan.Totals[sg.Action] = t
		}
		t.Items++
		t.Slots += sg.Slots
		t.Vendor += sg.Vendor
		t.Auction += sg.AuctionNet
	}
	return plan
}

func rank(action string) int {
	for i, a := range actionOrder {
		if a == action {
			return i
		}
	}
	return len(actionOrder)
}
