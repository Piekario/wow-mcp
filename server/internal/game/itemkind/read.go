// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package itemkind

import "wowmcp/internal/snapshot"

// Read takes from a dump item (bags, bank, vendor, mail, auction price) what classification needs.
func Read(item map[string]any) Item {
	i := Item{QuestItem: item["isQuestItem"] == true}
	i.Name, _ = item["name"].(string)
	i.Type, _ = item["type"].(string)
	i.SubType, _ = item["subType"].(string)
	i.Class, i.HasClass = snapshot.AsInt(item["classID"])
	i.Subclass, i.HasSubclass = snapshot.AsInt(item["subclassID"])
	i.Bind, i.HasBind = snapshot.AsInt(item["bindType"])
	i.Bound, i.BoundKnown = item["isBound"].(bool)
	i.QuestID, _ = snapshot.AsInt(item["questID"])
	for _, line := range snapshot.AsList(item["tooltip"]) {
		if text, ok := line.(string); ok {
			i.Tooltip = append(i.Tooltip, text)
		}
	}
	return i
}

// The functions below judge a dump item directly, for callers that still hold the raw map.

func ClassOf(item map[string]any) (int64, bool)    { return snapshot.AsInt(item["classID"]) }
func SubclassOf(item map[string]any) (int64, bool) { return snapshot.AsInt(item["subclassID"]) }
func HasIDs(item map[string]any) bool              { return Read(item).HasClass }
func Is(item map[string]any, class int64) bool     { return Read(item).Is(class) }
func IsGearClass(item map[string]any) bool         { return Read(item).IsGearClass() }
func AmmoKind(item map[string]any) string          { return Read(item).AmmoKind() }
func IsAmmo(item map[string]any) bool              { return Read(item).IsAmmo() }
func RangedAmmo(weapon map[string]any) string      { return Read(weapon).RangedAmmo() }
func ConsumableKind(item map[string]any) string    { return Read(item).ConsumableKind() }
func IsFood(item map[string]any) bool              { return Read(item).IsFood() }
func TradeGoodsKind(item map[string]any) string    { return Read(item).TradeGoodsKind() }
func IsQuestItem(item map[string]any) bool         { return Read(item).IsQuestItem() }
func StartsQuest(item map[string]any) bool         { return Read(item).StartsQuest() }
func IsBound(item map[string]any) bool             { return Read(item).IsBound() }
func BindingKnown(item map[string]any) bool        { return Read(item).BindingKnown() }
func Tradable(item map[string]any) bool            { return Read(item).Tradable() }
