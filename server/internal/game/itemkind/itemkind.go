// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package itemkind classifies items from the dump without depending on the client language. Addon 0.9.0
// saves the numeric item class and subclass (classID, subclassID), the binding type (bindType) and, for bag and
// bank items, isBound, isQuestItem and questID. Those are the same on every client (enUS, deDE, ruRU...), so they
// come first. The English type, subtype and tooltip text is only a fallback for dumps from older addons, which
// always came from English clients.
package itemkind

import (
	"strings"

	"wowmcp/internal/snapshot"
)

// Item classes: Blizzard's Enum.ItemClass (the same values in classic and modern clients).
const (
	ClassConsumable    int64 = 0
	ClassContainer     int64 = 1
	ClassWeapon        int64 = 2
	ClassArmor         int64 = 4
	ClassReagent       int64 = 5
	ClassProjectile    int64 = 6
	ClassTradeGoods    int64 = 7
	ClassRecipe        int64 = 9
	ClassQuiver        int64 = 11
	ClassQuest         int64 = 12
	ClassMiscellaneous int64 = 15
)

// Consumable subclasses: Enum.ItemConsumableSubclass.
const (
	consumablePotion    int64 = 1
	consumableElixir    int64 = 2
	consumableFlask     int64 = 3 // Enum.ItemConsumableSubclass.Flasksphials
	consumableFoodDrink int64 = 5 // Enum.ItemConsumableSubclass.Fooddrink
	consumableBandage   int64 = 7
)

// Projectile subclasses (item class 6): 2 Arrow, 3 Bullet (the classic ItemSubClass table).
const (
	projectileArrow  int64 = 2
	projectileBullet int64 = 3
)

// Weapon subclasses: Enum.ItemWeaponSubclass.
const (
	weaponBows        int64 = 2
	weaponGuns        int64 = 3
	weaponCrossbow    int64 = 18
	WeaponFishingPole int64 = 20
)

// Trade Goods subclasses: Enum.ItemTradeGoodsSubclass.
const (
	tradeCloth      int64 = 5
	tradeLeather    int64 = 6
	tradeMetalStone int64 = 7
	tradeCooking    int64 = 8
	tradeHerb       int64 = 9
	tradeEnchanting int64 = 12
)

// Binding types: Enum.ItemBind.
const (
	BindNone      int64 = 0
	BindOnAcquire int64 = 1 // binds when picked up
	BindOnEquip   int64 = 2
	BindOnUse     int64 = 3
	BindQuest     int64 = 4
)

// Ammo kinds, in the server's own (English) words.
const (
	Arrow  = "Arrow"
	Bullet = "Bullet"
)

// englishType is the item type (GetItemInfo's itemType) an English client gives for a class: the fallback for old dumps.
var englishType = map[int64]string{
	ClassConsumable: "Consumable", ClassContainer: "Container", ClassWeapon: "Weapon", ClassArmor: "Armor",
	ClassReagent: "Reagent", ClassProjectile: "Projectile", ClassTradeGoods: "Trade Goods", ClassRecipe: "Recipe",
	ClassQuiver: "Quiver", ClassQuest: "Quest", ClassMiscellaneous: "Miscellaneous",
}

// tradeGoodsKinds names the Trade Goods subclasses the server reasons about (the enUS subtype names).
var tradeGoodsKinds = map[int64]string{
	tradeCloth: "Cloth", tradeLeather: "Leather", tradeMetalStone: "Metal & Stone", tradeCooking: "Cooking",
	tradeHerb: "Herb", tradeEnchanting: "Enchanting",
}

// ClassOf returns the numeric item class; false when the dump has none (addon older than 0.9.0, or no item data).
func ClassOf(item map[string]any) (int64, bool) { return snapshot.AsInt(item["classID"]) }

// SubclassOf returns the numeric item subclass; false when the dump has none.
func SubclassOf(item map[string]any) (int64, bool) { return snapshot.AsInt(item["subclassID"]) }

// HasIDs says whether the item carries the locale-independent class (addon 0.9.0+).
func HasIDs(item map[string]any) bool {
	_, ok := ClassOf(item)
	return ok
}

func text(item map[string]any, key string) string {
	s, _ := item[key].(string)
	return s
}

// Is says whether the item belongs to a class: by classID, or for an old dump by the English type name.
func Is(item map[string]any, class int64) bool {
	if c, ok := ClassOf(item); ok {
		return c == class
	}
	return text(item, "type") == englishType[class]
}

// IsGearClass says whether the item is a weapon or armor.
func IsGearClass(item map[string]any) bool { return Is(item, ClassWeapon) || Is(item, ClassArmor) }

// AmmoKind returns Arrow or Bullet for ammunition, "" for anything else (or ammo of an unknown kind).
func AmmoKind(item map[string]any) string {
	if c, ok := ClassOf(item); ok {
		if c != ClassProjectile {
			return ""
		}
		switch sub, _ := SubclassOf(item); sub {
		case projectileArrow:
			return Arrow
		case projectileBullet:
			return Bullet
		}
		return ""
	}
	switch text(item, "subType") {
	case "Arrow":
		return Arrow
	case "Bullet":
		return Bullet
	}
	return ""
}

func IsAmmo(item map[string]any) bool {
	if c, ok := ClassOf(item); ok {
		return c == ClassProjectile
	}
	return text(item, "type") == "Projectile" || AmmoKind(item) != ""
}

// RangedAmmo says which ammo a ranged weapon fires: Arrow (bows, crossbows), Bullet (guns), "" when unknown.
func RangedAmmo(weapon map[string]any) string {
	if weapon == nil {
		return ""
	}
	if c, ok := ClassOf(weapon); ok {
		if c != ClassWeapon {
			return ""
		}
		switch sub, _ := SubclassOf(weapon); sub {
		case weaponBows, weaponCrossbow:
			return Arrow
		case weaponGuns:
			return Bullet
		}
		return ""
	}
	switch strings.ToLower(text(weapon, "subType")) {
	case "bows", "bow", "crossbows", "crossbow":
		return Arrow
	case "guns", "gun":
		return Bullet
	}
	return ""
}

const (
	Food    = "food"    // food and drink
	Bandage = "bandage" // bandages
	Potion  = "potion"  // potions, elixirs, flasks
	Other   = "other"   // any other consumable (scrolls, devices...)
)

// ConsumableKind returns the kind of a consumable item (Food, Bandage, Potion, Other) or "" when it is not one.
func ConsumableKind(item map[string]any) string {
	if !Is(item, ClassConsumable) {
		return ""
	}
	if sub, ok := SubclassOf(item); ok && HasIDs(item) {
		switch sub {
		case consumableFoodDrink:
			return Food
		case consumableBandage:
			return Bandage
		case consumablePotion, consumableElixir, consumableFlask:
			return Potion
		}
		// An item name is client text: only a last resort for bandages filed as generic consumables.
		if nameMentionsBandage(item) {
			return Bandage
		}
		return Other
	}
	lower := strings.ToLower(text(item, "subType"))
	switch {
	case strings.Contains(lower, "food") || strings.Contains(lower, "drink"):
		return Food
	case strings.Contains(lower, "bandage") || nameMentionsBandage(item):
		return Bandage
	case strings.Contains(lower, "potion") || strings.Contains(lower, "elixir") || strings.Contains(lower, "flask"):
		return Potion
	}
	return Other
}

func nameMentionsBandage(item map[string]any) bool {
	return strings.Contains(strings.ToLower(text(item, "name")), "bandage")
}

// IsFood says whether the item is food or drink (the Food & Drink subclass). Without IDs: a subtype containing "food".
func IsFood(item map[string]any) bool {
	if HasIDs(item) {
		return ConsumableKind(item) == Food
	}
	return strings.Contains(strings.ToLower(text(item, "subType")), "food")
}

// TradeGoodsKind returns the kind of a Trade Goods material in English (Cloth, Leather, Metal & Stone, Cooking,
// Herb, Enchanting), or "" when it is not Trade Goods or not one of those kinds.
func TradeGoodsKind(item map[string]any) string {
	if !Is(item, ClassTradeGoods) {
		return ""
	}
	if HasIDs(item) {
		sub, _ := SubclassOf(item)
		return tradeGoodsKinds[sub]
	}
	sub := text(item, "subType")
	if sub == "Meat" { // the classic name of the Cooking subclass
		return "Cooking"
	}
	for _, kind := range tradeGoodsKinds {
		if kind == sub {
			return kind
		}
	}
	return ""
}

// IsQuestItem says whether the item is a quest item: the container's isQuestItem, the Quest class,
// a quest binding, or for an old dump the English type Quest.
func IsQuestItem(item map[string]any) bool {
	if item["isQuestItem"] == true || Is(item, ClassQuest) {
		return true
	}
	bind, ok := snapshot.AsInt(item["bindType"])
	return ok && bind == BindQuest
}

// StartsQuest says whether the item begins a quest: questID from the container (addon 0.9.0), or for an old dump
// the English tooltip line "This Item Begins a Quest".
func StartsQuest(item map[string]any) bool {
	if id, ok := snapshot.AsInt(item["questID"]); ok && id > 0 {
		return true
	}
	if HasIDs(item) {
		return false
	}
	for _, line := range snapshot.AsList(item["tooltip"]) {
		if l, _ := line.(string); strings.Contains(strings.ToLower(l), "begins a quest") {
			return true
		}
	}
	return false
}

// IsBound says whether an owned item (bags, bank) is bound to the character: isBound from the container
// (addon 0.9.0), then a binding that applies on pickup (Binds when picked up, quest items), and for old dumps
// the English tooltip lines Soulbound and Quest Item.
func IsBound(item map[string]any) bool {
	if b, ok := item["isBound"].(bool); ok {
		return b
	}
	if bind, ok := snapshot.AsInt(item["bindType"]); ok {
		return bind == BindOnAcquire || bind == BindQuest
	}
	if HasIDs(item) {
		return false
	}
	for _, line := range snapshot.AsList(item["tooltip"]) {
		if l, _ := line.(string); strings.HasPrefix(l, "Soulbound") || strings.HasPrefix(l, "Quest Item") {
			return true
		}
	}
	return false
}

// BindingKnown says whether the dump states the binding of the item without reading tooltip text.
func BindingKnown(item map[string]any) bool {
	if _, ok := item["isBound"].(bool); ok {
		return true
	}
	_, ok := snapshot.AsInt(item["bindType"])
	return ok
}

// Tradable says whether an owned item can be mailed or sold at the auction house: neither bound nor a quest item.
func Tradable(item map[string]any) bool { return !IsQuestItem(item) && !IsBound(item) }

// ClassByEnglishName finds the item class for an English type name (e.g. "Trade Goods"), case-insensitive.
// Used to filter items from a localized client by the English words a player or model types.
func ClassByEnglishName(name string) (int64, bool) {
	for class, english := range englishType {
		if strings.EqualFold(english, strings.TrimSpace(name)) {
			return class, true
		}
	}
	return 0, false
}
