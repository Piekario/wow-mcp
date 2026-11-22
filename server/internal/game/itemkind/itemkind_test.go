// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package itemkind

import "testing"

// pair is the same item twice: as an old dump from an English client (text only) and as an addon 0.9.0 dump
// from a German client (IDs, German text). Every classification must agree.
type pair struct {
	label           string
	english, german map[string]any
}

func pairs() []pair {
	return []pair{
		{"arrow",
			map[string]any{"type": "Projectile", "subType": "Arrow"},
			map[string]any{"type": "Projektil", "subType": "Pfeil", "classID": int64(6), "subclassID": int64(2), "bindType": int64(0)}},
		{"bullet",
			map[string]any{"type": "Projectile", "subType": "Bullet"},
			map[string]any{"type": "Projektil", "subType": "Kugel", "classID": int64(6), "subclassID": int64(3)}},
		{"bread",
			map[string]any{"type": "Consumable", "subType": "Food & Drink"},
			map[string]any{"type": "Verbrauchbar", "subType": "Essen & Trinken", "classID": int64(0), "subclassID": int64(5)}},
		{"potion",
			map[string]any{"type": "Consumable", "subType": "Potion"},
			map[string]any{"type": "Verbrauchbar", "subType": "Trank", "classID": int64(0), "subclassID": int64(1)}},
		{"bandage",
			map[string]any{"type": "Consumable", "subType": "Bandage"},
			map[string]any{"type": "Verbrauchbar", "subType": "Verband", "classID": int64(0), "subclassID": int64(7)}},
		{"scroll",
			map[string]any{"type": "Consumable", "subType": "Scrolls"},
			map[string]any{"type": "Verbrauchbar", "subType": "Schriftrollen", "classID": int64(0), "subclassID": int64(4)}},
		{"leather",
			map[string]any{"type": "Trade Goods", "subType": "Leather"},
			map[string]any{"type": "Handwerkswaren", "subType": "Leder", "classID": int64(7), "subclassID": int64(6)}},
		{"meat",
			map[string]any{"type": "Trade Goods", "subType": "Meat"},
			map[string]any{"type": "Handwerkswaren", "subType": "Kochzutat", "classID": int64(7), "subclassID": int64(8)}},
		{"quest item",
			map[string]any{"type": "Quest", "subType": "Quest"},
			map[string]any{"type": "Quest", "subType": "Quest", "classID": int64(12), "subclassID": int64(0), "isQuestItem": true}},
		{"quest starter",
			map[string]any{"type": "Miscellaneous", "tooltip": []any{"Old Letter", "This Item Begins a Quest"}},
			map[string]any{"type": "Verschiedenes", "classID": int64(15), "subclassID": int64(0), "questID": int64(123), "tooltip": []any{"Alter Brief", "Dieser Gegenstand startet eine Quest"}}},
		{"soulbound sword",
			map[string]any{"type": "Weapon", "subType": "One-Handed Swords", "tooltip": []any{"Sword", "Soulbound"}},
			map[string]any{"type": "Waffe", "subType": "Einhandschwerter", "classID": int64(2), "subclassID": int64(7), "isBound": true, "tooltip": []any{"Schwert", "Seelengebunden"}}},
		{"bind on pickup, no container flag",
			map[string]any{"type": "Armor", "tooltip": []any{"Helm", "Soulbound"}},
			map[string]any{"type": "Rüstung", "classID": int64(4), "subclassID": int64(1), "bindType": int64(1)}},
		{"bind on equip in bags",
			map[string]any{"type": "Armor", "tooltip": []any{"Helm", "Binds when equipped"}},
			map[string]any{"type": "Rüstung", "classID": int64(4), "subclassID": int64(1), "bindType": int64(2), "isBound": false}},
		{"bow",
			map[string]any{"type": "Weapon", "subType": "Bows"},
			map[string]any{"type": "Waffe", "subType": "Bögen", "classID": int64(2), "subclassID": int64(2)}},
		{"gun",
			map[string]any{"type": "Weapon", "subType": "Guns"},
			map[string]any{"type": "Waffe", "subType": "Schusswaffen", "classID": int64(2), "subclassID": int64(3)}},
		{"recipe",
			map[string]any{"type": "Recipe", "subType": "Leatherworking"},
			map[string]any{"type": "Rezept", "subType": "Lederverarbeitung", "classID": int64(9), "subclassID": int64(1)}},
		{"reagent",
			map[string]any{"type": "Reagent", "subType": "Reagent"},
			map[string]any{"type": "Reagenz", "subType": "Reagenz", "classID": int64(5), "subclassID": int64(0)}},
	}
}

func TestEnglishAndGermanItemsClassifyTheSame(t *testing.T) {
	type verdict struct {
		ammo                                bool
		ammoKind, ranged, consumable, trade string
		food, quest, starts, bound, gear    bool
		tradable, recipe, reagent, trades   bool
	}
	judge := func(item map[string]any) verdict {
		return verdict{
			ammo: IsAmmo(item), ammoKind: AmmoKind(item), ranged: RangedAmmo(item), consumable: ConsumableKind(item),
			trade: TradeGoodsKind(item), food: IsFood(item), quest: IsQuestItem(item), starts: StartsQuest(item),
			bound: IsBound(item), gear: IsGearClass(item), tradable: Tradable(item), recipe: Is(item, ClassRecipe),
			reagent: Is(item, ClassReagent), trades: Is(item, ClassTradeGoods),
		}
	}
	for _, p := range pairs() {
		en, de := judge(p.english), judge(p.german)
		if en != de {
			t.Errorf("%s: English %+v, German %+v", p.label, en, de)
		}
	}
}

func TestClassificationValues(t *testing.T) {
	byLabel := map[string]pair{}
	for _, p := range pairs() {
		byLabel[p.label] = p
	}
	de := func(label string) map[string]any { return byLabel[label].german }
	switch {
	case AmmoKind(de("arrow")) != Arrow || AmmoKind(de("bullet")) != Bullet:
		t.Error("ammo kinds")
	case RangedAmmo(de("bow")) != Arrow || RangedAmmo(de("gun")) != Bullet:
		t.Error("ranged ammo")
	case ConsumableKind(de("bread")) != Food || ConsumableKind(de("potion")) != Potion || ConsumableKind(de("bandage")) != Bandage || ConsumableKind(de("scroll")) != Other:
		t.Error("consumable kinds")
	case TradeGoodsKind(de("leather")) != "Leather" || TradeGoodsKind(de("meat")) != "Cooking":
		t.Error("trade goods kinds")
	case !IsQuestItem(de("quest item")) || !StartsQuest(de("quest starter")) || IsQuestItem(de("quest starter")):
		t.Error("quest items")
	case !IsBound(de("soulbound sword")) || !IsBound(de("bind on pickup, no container flag")) || IsBound(de("bind on equip in bags")):
		t.Error("binding")
	case Tradable(de("soulbound sword")) || !Tradable(de("leather")) || Tradable(de("quest item")):
		t.Error("tradable")
	}
}

func TestGermanTooltipIsNotReadAsEnglish(t *testing.T) {
	// With IDs, an English-looking tooltip line no longer decides: the container flags do.
	item := map[string]any{"classID": int64(4), "isBound": false, "tooltip": []any{"Soulbound"}}
	if IsBound(item) {
		t.Error("isBound=false must win over tooltip text")
	}
	if !BindingKnown(item) || BindingKnown(map[string]any{"type": "Armor"}) {
		t.Error("BindingKnown")
	}
}

func TestClassByEnglishName(t *testing.T) {
	if c, ok := ClassByEnglishName("trade goods"); !ok || c != ClassTradeGoods {
		t.Errorf("trade goods: %d %v", c, ok)
	}
	if _, ok := ClassByEnglishName("Handwerkswaren"); ok {
		t.Error("only English names are known")
	}
}
