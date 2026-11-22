// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package bagplan suggests what to do with full bags: keep, move to the bank, sell to a
// vendor or list on the auction house. It combines ready assessments from other contexts (quests,
// recipes, auctions, pet, equipment); it removes nothing and never advises destroying items.
package bagplan

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"wowmcp/internal/game/auctions"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/leftovers"
	"wowmcp/internal/game/pet"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// Recommended actions. There is deliberately no "destroy" or "discard" here.
const (
	Keep    = "keep"
	Bank    = "bank"
	Sell    = "sell"
	Auction = "auction"
)

// actionOrder is the order in the response: first what frees space for money.
var actionOrder = []string{Sell, Auction, Bank, Keep}

// stack is the bag items with one name, summed.
type stack struct {
	item   map[string]any // first instance (type, quality, tooltip)
	name   string
	count  int64
	slots  int64
	sell   int64 // sell price per unit
	vendor int64 // vendor value for all units
}

// Suggestion is the decision for one item.
type Suggestion struct {
	stack
	Action, Reason string
	AuctionNet     int64 // value at auction after the cut (for Auction)
}

// stacks sums the bags by name, in order of first appearance.
func stacks(s snapshot.Snapshot) []*stack {
	byName := map[string]*stack{}
	var order []*stack
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			if name == "" {
				continue
			}
			st := byName[name]
			if st == nil {
				st = &stack{item: item, name: name}
				st.sell, _ = snapshot.AsInt(item["sellPrice"])
				byName[name] = st
				order = append(order, st)
			}
			n := snapshot.ItemCount(item)
			st.count += n
			st.slots++
			st.vendor += st.sell * n
		}
	}
	return order
}

// context holds the dump data needed to assess many items at once.
type context struct {
	snap       snapshot.Snapshot
	questKeep  map[string]string // name -> reason (quest items from the quest log)
	questOther map[string]bool   // quest items without a quest in the log
	reagents   map[string]string // known recipes: material -> recipe
	offers     map[string]auctions.Offer
	rangedKind string // "Arrow", "Bullet" or "" when unknown
	hasPet     bool
	petFood    []any
}

func newContext(s snapshot.Snapshot, fresh snapshot.Freshness) *context {
	c := &context{
		snap: s, questKeep: map[string]string{}, questOther: map[string]bool{},
		reagents: professions.KnownReagents(s), offers: auctions.Offers(s, fresh),
		rangedKind: ammoFor(s), hasPet: pet.Exists(s), petFood: snapshot.AsList(s.Map("pet")["foodTypes"]),
	}
	verdicts, _ := leftovers.Assess(s)
	for _, v := range verdicts {
		if v.Keep {
			c.questKeep[v.Name] = v.Reason
		} else {
			c.questOther[v.Name] = true
		}
	}
	return c
}

// ammoFor says which ammo the equipped ranged weapon uses (only certain cases: bows, crossbows and guns).
func ammoFor(s snapshot.Snapshot) string {
	return itemkind.RangedAmmo(snapshot.EquippedBySlot(s.Equipment())["ranged"])
}

// professionTool is a profession tool: recognised by item ID (or, for fishing poles, the weapon subclass), and by
// English name as a fallback for old dumps.
type professionTool struct {
	ids      []int64
	namePart string
	skill    int64
}

// professionTools: Skinning Knife (7005), Mining Pick (2901), Fishing Pole (6256, and any item of the Fishing
// Pole weapon subclass).
var professionTools = []professionTool{
	{[]int64{6256}, "Fishing Pole", professions.SkillFishing},
	{[]int64{2901}, "Mining Pick", professions.SkillMining},
	{[]int64{7005}, "Skinning Knife", professions.SkillSkinning},
}

func (t professionTool) matches(item map[string]any, name string) bool {
	id, _ := snapshot.AsInt(item["id"])
	if slices.Contains(t.ids, id) {
		return true
	}
	if sub, ok := itemkind.SubclassOf(item); ok && t.skill == professions.SkillFishing &&
		itemkind.Is(item, itemkind.ClassWeapon) && sub == itemkind.WeaponFishingPole {
		return true
	}
	return strings.Contains(name, t.namePart)
}

// decide picks the action for an item: the first matching rule wins.
func (c *context) decide(st *stack) Suggestion {
	sg := Suggestion{stack: *st}
	keep := func(why string) Suggestion { sg.Action, sg.Reason = Keep, why; return sg }
	bank := func(why string) Suggestion { sg.Action, sg.Reason = Bank, why; return sg }
	sell := func(why string) Suggestion { sg.Action, sg.Reason = Sell, why; return sg }

	quality, _ := snapshot.AsInt(st.item["quality"])

	switch {
	case c.questKeep[st.name] != "":
		return keep("quest item: " + c.questKeep[st.name])
	case c.questOther[st.name]:
		return bank("quest item with no quest in the log (may be a leftover); bank it instead of discarding, because the server cannot be sure")
	case itemkind.IsAmmo(st.item):
		if kind := itemkind.AmmoKind(st.item); c.rangedKind != "" && kind != c.rangedKind {
			return bank(fmt.Sprintf("%s ammo does not match the equipped weapon (it uses %s); may come in handy after a weapon change", ammoLabel(kind), c.rangedKind))
		}
		return keep("ammo for the equipped ranged weapon")
	}
	if why, ok := c.reagents[strings.ToLower(st.name)]; ok {
		return keep("material for a known recipe: " + why)
	}
	for _, tool := range professionTools {
		if tool.matches(st.item, st.name) && professions.HasSkill(c.snap, tool.skill) {
			return keep("profession tool: " + professions.EnglishNames[tool.skill])
		}
	}
	if itemkind.Is(st.item, itemkind.ClassConsumable) {
		if itemkind.IsFood(st.item) && c.hasPet {
			return keep(fmt.Sprintf("food: yours or your pet's (diet: %s; the server has no food table, so it does not decide by name)", dietText(c.petFood)))
		}
		return keep("consumable, used up in game")
	}
	if offer, ok := c.offers[st.name]; ok && offer.WorthAuction {
		sg.Action, sg.AuctionNet = Auction, offer.AuctionNet
		sg.Reason = fmt.Sprintf("the saved auction price after the cut gives %s instead of %s from a vendor", snapshot.Money(offer.AuctionNet), snapshot.Money(offer.VendorTotal))
		if offer.PriceAge != "" {
			sg.Reason += " (price from " + offer.PriceAge + " ago, check it)"
		}
		return sg
	}
	if slot, ok := items.PossibleUpgrade(c.snap, st.item); ok {
		return keep("may be an upgrade for slot " + slot + " (compare: wow_compare_item)")
	}
	switch {
	case quality == 0 && st.sell > 0:
		return sell("grey item, only good for selling")
	case st.sell <= 0:
		return keep("no vendor will buy it (price 0), so selling frees nothing; keep it or move it to the bank")
	case itemkind.Is(st.item, itemkind.ClassTradeGoods):
		return bank("trade material: in the bank it does not take bag space, and selling pays off only if you do not gather it")
	case quality >= 2:
		return bank(fmt.Sprintf("%s quality with no saved auction price: move it to the bank and check the auction price instead of selling blind", qualityLabel(quality)))
	case itemkind.IsGearClass(st.item):
		return sell("ordinary wearable item, not an upgrade and with no auction price")
	case itemkind.Is(st.item, itemkind.ClassRecipe):
		return bank("recipe/pattern: may be useful for an alt's profession or at auction; move it to the bank")
	}
	return sell("ordinary item with no use according to the dump (not a quest item, no recipe or auction price)")
}

// ammoLabel names an ammo kind for the reason text; "" (unknown) stays readable.
func ammoLabel(kind string) string {
	if kind == "" {
		return "This"
	}
	return kind
}

var qualityNames = []string{"Poor", "Common", "Uncommon", "Rare", "Epic", "Legendary"}

func qualityLabel(q int64) string {
	if q >= 0 && int(q) < len(qualityNames) {
		return qualityNames[q]
	}
	return fmt.Sprint(q)
}

func dietText(diet []any) string {
	var out []string
	for _, d := range diet {
		out = append(out, fmt.Sprint(d))
	}
	if len(out) == 0 {
		return "unknown"
	}
	return strings.Join(out, ", ")
}

// View assesses every item in the bags and summarises how many slots and how much gold this yields.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	groups := stacks(s)
	space := items.BagSpace(s)
	if len(groups) == 0 {
		return map[string]any{"note": "The bags are empty or the dump has no bags section.", "bagSlots": space}
	}
	c := newContext(s, fresh)
	list := make([]Suggestion, len(groups))
	for i, st := range groups {
		list[i] = c.decide(st)
	}
	sort.SliceStable(list, func(i, j int) bool { return rank(list[i].Action) < rank(list[j].Action) })

	entries := make([]any, len(list))
	totals := map[string]*total{}
	for i, sg := range list {
		entries[i] = sg.entry()
		t := totals[sg.Action]
		if t == nil {
			t = &total{}
			totals[sg.Action] = t
		}
		t.items++
		t.slots += sg.slots
		t.vendor += sg.vendor
		t.auction += sg.AuctionNet
	}
	free, _ := snapshot.AsInt(space["freeSlots"])
	freed := slotsOf(totals, Sell) + slotsOf(totals, Auction) + slotsOf(totals, Bank)
	out := map[string]any{
		"items": entries,
		"bagSlots": map[string]any{
			"total": space["totalSlots"], "free": free, "freeAfterPlan": free + freed,
		},
		"slotsFreed": freed,
		"summary":    summary(totals, s),
		"howDecided": "Rule order: quest items, ammo for the equipped weapon, materials for known recipes and profession tools, consumables, auction offer (price after the cut with a 20% margin over the vendor), possible upgrades, then selling grey and ordinary items, while trade materials and better quality go to the bank.",
		"limits":     limitsText(s),
	}
	if bank := s.Map("bank"); bank == nil {
		out["bankNote"] = "No saved bank (it was never opened), so it is unknown whether there is room in it for items with the bank action."
	}
	return out
}

// limitsText says what the plan cannot know; binding is known only from addon 0.9.0 dumps (isBound, bindType).
func limitsText(s snapshot.Snapshot) string {
	binding := "The dump does not report binding (Soulbound) of bag items, so auctions are only a suggestion."
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			if itemkind.BindingKnown(snapshot.AsMap(raw)) {
				binding = "Bound items are known from the dump (isBound, bindType) and are never suggested for the auction house."
				break
			}
		}
	}
	return "The server cannot see what your other characters need, nor does it know every recipe (only those from opened profession windows, wow_recipes). " +
		binding + " The plan has no destroy or discard action; nothing is sold or removed, you make the decisions in game."
}

func rank(action string) int {
	for i, a := range actionOrder {
		if a == action {
			return i
		}
	}
	return len(actionOrder)
}

func (sg Suggestion) entry() map[string]any {
	e := map[string]any{"name": sg.name, "count": sg.count, "slots": sg.slots, "action": sg.Action, "reason": sg.Reason}
	if sg.vendor > 0 {
		e["vendorValue"] = snapshot.Money(sg.vendor).String()
	}
	if sg.Action == Auction {
		e["auctionAfterCut"] = snapshot.Money(sg.AuctionNet).String()
	}
	return e
}

type total struct {
	items, slots, vendor, auction int64
}

func slotsOf(totals map[string]*total, action string) int64 {
	if t := totals[action]; t != nil {
		return t.slots
	}
	return 0
}

// summary gives totals for each action; the bank gets the free slots from the last visit.
func summary(totals map[string]*total, s snapshot.Snapshot) map[string]any {
	out := map[string]any{}
	for _, action := range actionOrder {
		t := totals[action]
		if t == nil {
			continue
		}
		e := map[string]any{"items": t.items, "slots": t.slots}
		switch action {
		case Sell:
			e["vendorValue"] = snapshot.Money(t.vendor).String()
		case Auction:
			e["afterCut"] = snapshot.Money(t.auction).String()
			e["vendorInstead"] = snapshot.Money(t.vendor).String()
		case Bank:
			if bank := s.Map("bank"); bank != nil {
				var free int64
				for _, c := range snapshot.AsList(bank["containers"]) {
					n, _ := snapshot.AsInt(snapshot.AsMap(c)["free"])
					free += n
				}
				e["bankFreeSlots"] = free
				e["fits"] = free >= t.slots
			}
		}
		out[action] = e
	}
	return out
}
