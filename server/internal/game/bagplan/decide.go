// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package bagplan

import (
	"fmt"
	"slices"
	"strings"

	"wowmcp/internal/game/auctions"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

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

func (t professionTool) matches(it items.Item, name string) bool {
	if slices.Contains(t.ids, it.ID) {
		return true
	}
	if it.HasSubclass && t.skill == professions.SkillFishing && it.Is(itemkind.ClassWeapon) && it.Subclass == itemkind.WeaponFishingPole {
		return true
	}
	return strings.Contains(name, t.namePart)
}

// context holds what is known about the character to assess many items at once.
type context struct {
	buyer      items.Buyer
	questKeep  map[string]string // name -> reason (quest items from the quest log)
	questOther map[string]bool   // quest items without a quest in the log
	reagents   map[string]string // known recipes: material -> recipe
	offers     map[string]auctions.Offer
	skills     []professions.Profession
	rangedKind string // "Arrow", "Bullet" or "" when unknown
	hasPet     bool
	diet       []string
	petEats    map[string]bool // item name -> the game client's verdict whether the pet can eat it (addon 0.22.0)
}

// decide picks the action for an item: the first matching rule wins.
func (c *context) decide(st Stack) Suggestion {
	sg := Suggestion{Stack: st}
	keep := func(why string) Suggestion { sg.Action, sg.Reason = Keep, why; return sg }
	bank := func(why string) Suggestion { sg.Action, sg.Reason = Bank, why; return sg }
	it := st.Item

	switch {
	case c.questKeep[st.Name] != "":
		return keep("quest item: " + c.questKeep[st.Name])
	case c.questOther[st.Name]:
		return bank("quest item with no quest in the log (may be a leftover); bank it instead of discarding, because the server cannot be sure")
	case it.IsAmmo():
		if kind := it.AmmoKind(); c.rangedKind != "" && kind != c.rangedKind {
			return bank(fmt.Sprintf("%s ammo does not match the equipped weapon (it uses %s); may come in handy after a weapon change", ammoLabel(kind), c.rangedKind))
		}
		return keep("ammo for the equipped ranged weapon")
	}
	if why, ok := c.reagents[strings.ToLower(st.Name)]; ok {
		return keep("material for a known recipe: " + why)
	}
	if c.petEats[st.Name] {
		return keep("food your pet can eat (the game client says so)")
	}
	for _, tool := range professionTools {
		if tool.matches(it, st.Name) && c.hasSkill(tool.skill) {
			return keep("profession tool: " + professions.EnglishNames[tool.skill])
		}
	}
	if it.Is(itemkind.ClassConsumable) {
		if eats, checked := c.petEats[st.Name]; checked && !eats && it.IsFood() {
			return keep("food for you; the game client says your pet does not eat it")
		}
		if it.IsFood() && c.hasPet {
			return keep(fmt.Sprintf("food: yours or your pet's (diet: %s; the server has no food table, so it does not decide by name)", dietText(c.diet)))
		}
		return keep("consumable, used up in game")
	}
	if offer, ok := c.offers[st.Name]; ok && offer.WorthAuction {
		return auctionSuggestion(sg, offer)
	}
	if slot, ok := c.buyer.UpgradeSlot(it); ok {
		return keep("may be an upgrade for slot " + slot + " (compare: wow_compare_item)")
	}
	return sellOrBank(sg)
}

func (c *context) hasSkill(skill int64) bool {
	return slices.ContainsFunc(c.skills, func(p professions.Profession) bool {
		id, ok := p.SkillLine()
		return ok && id == skill
	})
}

func auctionSuggestion(sg Suggestion, offer auctions.Offer) Suggestion {
	sg.Action, sg.AuctionNet = Auction, offer.AuctionNet
	sg.Reason = fmt.Sprintf("the saved auction price after the cut gives %s instead of %s from a vendor", snapshot.Money(offer.AuctionNet), snapshot.Money(offer.VendorTotal))
	if offer.PriceAge != "" {
		sg.Reason += " (price from " + offer.PriceAge + " ago, check it)"
	}
	return sg
}

// sellOrBank is the last rules, for an item that is no quest item, ammo, material, tool, consumable,
// auction offer or upgrade.
func sellOrBank(sg Suggestion) Suggestion {
	it := sg.Item
	sell := func(why string) Suggestion { sg.Action, sg.Reason = Sell, why; return sg }
	bank := func(why string) Suggestion { sg.Action, sg.Reason = Bank, why; return sg }
	switch {
	case it.Quality == 0 && sg.Sell > 0:
		return sell("grey item, only good for selling")
	case sg.Sell <= 0:
		sg.Action, sg.Reason = Keep, "no vendor will buy it (price 0), so selling frees nothing; keep it or move it to the bank"
		return sg
	case it.Is(itemkind.ClassTradeGoods):
		return bank("trade material: in the bank it does not take bag space, and selling pays off only if you do not gather it")
	case it.Quality >= 2:
		return bank(fmt.Sprintf("%s quality with no saved auction price: move it to the bank and check the auction price instead of selling blind", qualityLabel(it.Quality)))
	case it.IsGearClass():
		return sell("ordinary wearable item, not an upgrade and with no auction price")
	case it.Is(itemkind.ClassRecipe):
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

func qualityLabel(q int64) string {
	if name, ok := itemkind.QualityName(q); ok {
		return name
	}
	return fmt.Sprint(q)
}

func dietText(diet []string) string {
	if len(diet) == 0 {
		return "unknown"
	}
	return strings.Join(diet, ", ")
}
