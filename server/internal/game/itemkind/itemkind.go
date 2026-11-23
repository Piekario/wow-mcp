// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package itemkind classifies items from the dump without depending on the client language. Addon 0.9.0
// saves the numeric item class and subclass (classID, subclassID), the binding type (bindType) and, for bag and
// bank items, isBound, isQuestItem and questID. Those are the same on every client (enUS, deDE, ruRU...), so they
// come first. The English type, subtype and tooltip text is only a fallback for dumps from older addons, which
// always came from English clients.
package itemkind

import "strings"

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

// Item is what the dump says about one item, as far as classification needs it. Class, Subclass and Bind are the
// client's numeric IDs (addon 0.9.0+); Type, SubType and Tooltip are client text, read only when the IDs are missing.
type Item struct {
	Name          string
	Type, SubType string
	Class         int64
	HasClass      bool
	Subclass      int64
	HasSubclass   bool
	Bind          int64
	HasBind       bool
	Bound         bool
	BoundKnown    bool
	QuestItem     bool
	QuestID       int64
	Tooltip       []string
}

// Is says whether the item belongs to a class: by classID, or for an old dump by the English type name.
func (i Item) Is(class int64) bool {
	if i.HasClass {
		return i.Class == class
	}
	return i.Type == englishType[class]
}

func (i Item) IsGearClass() bool { return i.Is(ClassWeapon) || i.Is(ClassArmor) }

// AmmoKind returns Arrow or Bullet for ammunition, "" for anything else (or ammo of an unknown kind).
func (i Item) AmmoKind() string {
	if i.HasClass {
		if i.Class != ClassProjectile {
			return ""
		}
		switch i.Subclass {
		case projectileArrow:
			return Arrow
		case projectileBullet:
			return Bullet
		}
		return ""
	}
	switch i.SubType {
	case "Arrow":
		return Arrow
	case "Bullet":
		return Bullet
	}
	return ""
}

func (i Item) IsAmmo() bool {
	if i.HasClass {
		return i.Class == ClassProjectile
	}
	return i.Type == "Projectile" || i.AmmoKind() != ""
}

// RangedAmmo says which ammo a ranged weapon fires: Arrow (bows, crossbows), Bullet (guns), "" when unknown.
func (i Item) RangedAmmo() string {
	if i.HasClass {
		if i.Class != ClassWeapon {
			return ""
		}
		switch i.Subclass {
		case weaponBows, weaponCrossbow:
			return Arrow
		case weaponGuns:
			return Bullet
		}
		return ""
	}
	switch strings.ToLower(i.SubType) {
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
func (i Item) ConsumableKind() string {
	if !i.Is(ClassConsumable) {
		return ""
	}
	if i.HasSubclass && i.HasClass {
		switch i.Subclass {
		case consumableFoodDrink:
			return Food
		case consumableBandage:
			return Bandage
		case consumablePotion, consumableElixir, consumableFlask:
			return Potion
		}
		// An item name is client text: only a last resort for bandages filed as generic consumables.
		if i.nameMentionsBandage() {
			return Bandage
		}
		return Other
	}
	lower := strings.ToLower(i.SubType)
	switch {
	case strings.Contains(lower, "food") || strings.Contains(lower, "drink"):
		return Food
	case strings.Contains(lower, "bandage") || i.nameMentionsBandage():
		return Bandage
	case strings.Contains(lower, "potion") || strings.Contains(lower, "elixir") || strings.Contains(lower, "flask"):
		return Potion
	}
	return Other
}

func (i Item) nameMentionsBandage() bool {
	return strings.Contains(strings.ToLower(i.Name), "bandage")
}

// IsFood says whether the item is food or drink (the Food & Drink subclass). Without IDs: a subtype containing "food".
func (i Item) IsFood() bool {
	if i.HasClass {
		return i.ConsumableKind() == Food
	}
	return strings.Contains(strings.ToLower(i.SubType), "food")
}

// TradeGoodsKind returns the kind of a Trade Goods material in English (Cloth, Leather, Metal & Stone, Cooking,
// Herb, Enchanting), or "" when it is not Trade Goods or not one of those kinds.
func (i Item) TradeGoodsKind() string {
	if !i.Is(ClassTradeGoods) {
		return ""
	}
	if i.HasClass {
		return tradeGoodsKinds[i.Subclass]
	}
	if i.SubType == "Meat" { // the classic name of the Cooking subclass
		return "Cooking"
	}
	for _, kind := range tradeGoodsKinds {
		if kind == i.SubType {
			return kind
		}
	}
	return ""
}

// IsQuestItem: the container's isQuestItem, the Quest class, a quest binding, or for an old dump the English type Quest.
func (i Item) IsQuestItem() bool {
	return i.QuestItem || i.Is(ClassQuest) || i.HasBind && i.Bind == BindQuest
}

// StartsQuest: questID from the container (addon 0.9.0), or for an old dump the English tooltip line
// "This Item Begins a Quest".
func (i Item) StartsQuest() bool {
	if i.QuestID > 0 {
		return true
	}
	if i.HasClass {
		return false
	}
	for _, line := range i.Tooltip {
		if strings.Contains(strings.ToLower(line), "begins a quest") {
			return true
		}
	}
	return false
}

// IsBound says whether an owned item (bags, bank) is bound to the character: isBound from the container
// (addon 0.9.0), then a binding that applies on pickup (Binds when picked up, quest items), and for old dumps
// the English tooltip lines Soulbound and Quest Item.
func (i Item) IsBound() bool {
	if i.BoundKnown {
		return i.Bound
	}
	if i.HasBind {
		return i.Bind == BindOnAcquire || i.Bind == BindQuest
	}
	if i.HasClass {
		return false
	}
	for _, line := range i.Tooltip {
		if strings.HasPrefix(line, "Soulbound") || strings.HasPrefix(line, "Quest Item") {
			return true
		}
	}
	return false
}

// BindingKnown says whether the dump states the binding of the item without reading tooltip text.
func (i Item) BindingKnown() bool { return i.BoundKnown || i.HasBind }

// Tradable says whether an owned item can be mailed or sold at the auction house: neither bound nor a quest item.
func (i Item) Tradable() bool { return !i.IsQuestItem() && !i.IsBound() }

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
