// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"fmt"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// record is a dump item as the client saved it; the answers of wow_bank and wow_vendor pass it through whole.
type record = map[string]any

func readItem(m record) Item {
	it := Item{Item: itemkind.Read(m), Count: snapshot.ItemCount(m), present: m != nil, raw: m}
	it.ID, it.HasID = snapshot.AsInt(m["id"])
	it.Quality, it.HasQuality = snapshot.AsInt(m["quality"])
	it.SellPrice, _ = snapshot.AsInt(m["sellPrice"])
	it.Price, it.HasPrice = snapshot.AsInt(m["priceCopper"])
	it.Stack, _ = snapshot.AsInt(m["stack"])
	it.EquipLoc, _ = m["equipLoc"].(string)
	it.RequiredLevel, _ = snapshot.AsInt(m["requiredLevel"])
	it.Unusable = m["usable"] == false
	it.Slot, it.HasSlot = m["slot"].(string)
	it.ItemLevel, it.HasItemLevel = snapshot.AsFloat(m["itemLevel"])
	_, it.HasTooltip = m["tooltip"]
	it.tooltipLines = len(snapshot.AsList(m["tooltip"]))
	it.Weapon = readWeaponNumbers(m)
	// statTokens (addon 0.9.0) are the same in every language; stats has the labels the client shows, so it is
	// the fallback for older dumps.
	stats := snapshot.AsMap(m["statTokens"])
	it.HasStatTokens = len(stats) > 0
	if !it.HasStatTokens {
		stats = snapshot.AsMap(m["stats"])
	}
	for k, v := range stats {
		if f, ok := snapshot.AsFloat(v); ok {
			if it.Stats == nil {
				it.Stats = map[string]float64{}
			}
			it.Stats[k] = f
		}
	}
	return it
}

func readWeaponNumbers(m record) WeaponNumbers {
	var w WeaponNumbers
	w.DamageMin, _ = snapshot.AsFloat(m["damageMin"])
	w.DamageMax, _ = snapshot.AsFloat(m["damageMax"])
	w.Speed, w.HasSpeed = snapshot.AsFloat(m["speed"])
	w.DPS, w.HasDPS = snapshot.AsFloat(m["dps"])
	return w
}

func readItems(list []any) []Item {
	out := make([]Item, 0, len(list))
	for _, raw := range list {
		out = append(out, readItem(snapshot.AsMap(raw)))
	}
	return out
}

func readContainer(m record) Container {
	c := Container{Name: m["name"], Items: readItems(snapshot.AsList(m["items"]))}
	c.Size, _ = snapshot.AsInt(m["size"])
	c.Free, _ = snapshot.AsInt(m["free"])
	return c
}

func readContainers(list []any) []Container {
	out := make([]Container, 0, len(list))
	for _, raw := range list {
		out = append(out, readContainer(snapshot.AsMap(raw)))
	}
	return out
}

func readBags(s snapshot.Snapshot) []Container { return readContainers(s.Bags()) }

func readEquipment(s snapshot.Snapshot) []Item { return readItems(s.Equipment()) }

func readBank(s snapshot.Snapshot) (BankSection, bool) {
	section := s.Map("bank")
	if section == nil {
		return BankSection{}, false
	}
	return BankSection{Containers: readContainers(snapshot.AsList(section["containers"])), Captured: snapshot.CapturedOf(section)}, true
}

func readVendorSection(section record) (VendorSection, bool) {
	if section == nil {
		return VendorSection{}, false
	}
	return VendorSection{Npc: section["npc"], CanRepair: section["canRepair"],
		Items: readItems(snapshot.AsList(section["items"])), Captured: snapshot.CapturedOf(section)}, true
}

func readLastVendor(s snapshot.Snapshot) (VendorSection, bool) {
	return readVendorSection(s.Map("vendor"))
}

func readBuyback(s snapshot.Snapshot) (BuybackSection, bool) {
	section := s.Map("buyback")
	if section == nil {
		return BuybackSection{}, false
	}
	b := BuybackSection{Npc: section["npc"], Items: readItems(snapshot.AsList(section["items"])), Captured: snapshot.CapturedOf(section)}
	b.At, b.HasAt = snapshot.AsInt(section["capturedAt"])
	return b, true
}

func readMailItems(s snapshot.Snapshot) []Letter {
	var out []Letter
	for _, raw := range snapshot.AsList(s.Map("mail")["messages"]) {
		m := snapshot.AsMap(raw)
		out = append(out, Letter{Sender: m["sender"], Subject: m["subject"], Items: readItems(snapshot.AsList(m["items"]))})
	}
	return out
}

func readWeaponSkills(s snapshot.Snapshot) []SkillLine {
	var out []SkillLine
	for _, raw := range s.List("professions") {
		p := snapshot.AsMap(raw)
		l := SkillLine{Name: p["name"], Rank: p["rank"], MaxRank: p["maxRank"]}
		l.ID, l.HasID = snapshot.AsInt(p["skillID"])
		l.NameText, _ = p["name"].(string)
		l.Group, _ = p["group"].(string)
		out = append(out, l)
	}
	return out
}

type identity struct{ Name, Class, ClassToken string }

func readIdentity(s snapshot.Snapshot) identity {
	c := s.Character()
	var who identity
	who.Name, _ = c["name"].(string)
	who.Class, _ = c["class"].(string)
	who.ClassToken, _ = c["classToken"].(string)
	return who
}

// nameLabel is the item name as text, the way answers and errors print it (a missing name prints as <nil>).
func (i Item) nameLabel() string { return fmt.Sprint(i.raw["name"]) }

// The functions and types below take or give raw dump items, for callers that still hold them.

// Found is an item found on a character and the place it comes from (bags, bank, vendor).
type Found struct {
	Item   map[string]any
	Source string
}

// Find is FindItems for callers that need the dump items.
func Find(s snapshot.Snapshot, query, source string) []Found {
	var out []Found
	for _, f := range FindItems(s, query, source) {
		out = append(out, Found{Item: f.Item.raw, Source: f.Source})
	}
	return out
}

// StockLine is ItemStock with the first instance as a dump item.
type StockLine struct {
	Name, Type, SubType string
	Item                map[string]any
	Bags, Bank          int64
}

func (l StockLine) Total() int64 { return l.Bags + l.Bank }

// BagBankStock is StockOf for callers that judge dump items.
func BagBankStock(s snapshot.Snapshot, keep func(item map[string]any) bool) map[string]*StockLine {
	var keepItem func(Item) bool
	if keep != nil {
		keepItem = func(it Item) bool { return keep(it.raw) }
	}
	out := map[string]*StockLine{}
	for key, st := range StockOf(s, keepItem) {
		out[key] = &StockLine{Name: st.Item.Name, Type: st.Item.Type, SubType: st.Item.SubType, Item: st.Item.raw, Bags: st.Bags, Bank: st.Bank}
	}
	return out
}

// Equippable says whether the item can be equipped (equipLoc matches an equipment slot).
func Equippable(item map[string]any) bool { return readItem(item).Equippable() }

// MatchesType is Item.IsOfType for a dump item.
func MatchesType(item map[string]any, kind string) bool { return readItem(item).IsOfType(kind) }

// Admits is Buyer.Allows for a dump item.
func (b Buyer) Admits(item map[string]any, price int64, hasPrice bool, f OfferFilter) bool {
	return b.Allows(readItem(item), price, hasPrice, f)
}

// PossibleUpgrade is Buyer.UpgradeSlot for a dump item and the character of the snapshot.
func PossibleUpgrade(s snapshot.Snapshot, item map[string]any) (slot string, ok bool) {
	return NewBuyer(s).UpgradeSlot(readItem(item))
}

// Read turns a dump item into an Item, for packages that keep their own sections of items (e.g. dungeon loot).
func Read(item map[string]any) Item { return readItem(item) }

// Evaluate is Buyer.Evaluate for a dump item and the character of the snapshot.
func Evaluate(s snapshot.Snapshot, item map[string]any, w Weights) Evaluation {
	return NewBuyer(s).Evaluate(readItem(item), w)
}
