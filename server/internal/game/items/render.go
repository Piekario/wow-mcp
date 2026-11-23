// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

const buybackAssumption = "The buyback list is an assumption, not a certainty: the addon saves it from the vendor window, and the client keeps it only until logout, " +
	"so after a relog the items on the list may no longer be buyable (the server cannot see this; see capturedAge). " +
	"priceCopper is the price for the whole batch (count units), as in the vendor offer."

// Summary is a short item description with the numbers for comparison.
func Summary(item map[string]any) map[string]any {
	if item == nil {
		return nil
	}
	return readItem(item).summary()
}

func (i Item) summary() map[string]any {
	out := map[string]any{"name": i.raw["name"], "id": i.raw["id"], "numbers": i.Numbers()}
	i.passThrough(out, "itemLevel", "requiredLevel", "subType", "equipLoc")
	if q := i.QualityName(); q != "" {
		out["quality"] = q
	}
	return out
}

// passThrough copies the dump fields that are present into the answer, unchanged.
func (i Item) passThrough(out map[string]any, keys ...string) {
	for _, k := range keys {
		if v, ok := i.raw[k]; ok {
			out[k] = v
		}
	}
}

// copyFields is a shallow copy of the dump item, to which the answer adds computed fields.
func (i Item) copyFields() map[string]any {
	entry := map[string]any{}
	for k, v := range i.raw {
		entry[k] = v
	}
	return entry
}

// Compare compares an item with what is equipped in each slot it fits; nil when it cannot be equipped.
func Compare(item map[string]any, equipment []any) []any {
	return renderComparisons(GearOf(readItems(equipment)).Compare(readItem(item)))
}

func renderComparisons(cmp []SlotComparison) []any {
	if cmp == nil {
		return nil
	}
	out := make([]any, 0, len(cmp))
	for _, c := range cmp {
		out = append(out, c.render())
	}
	return out
}

func (c SlotComparison) render() map[string]any {
	entry := map[string]any{"slot": c.Slot, "delta": c.Delta}
	if c.Empty() {
		entry["equipped"] = snapshot.Empty
		return entry
	}
	names := make([]string, 0, len(c.Replaced))
	for _, w := range c.Replaced {
		names = append(names, w.nameLabel())
	}
	entry["equipped"] = strings.Join(names, " + ")
	entry["equippedNumbers"] = c.Equipped
	return entry
}

// CompareView compares an item without file weights (only weights from the query).
func CompareView(s snapshot.Snapshot, q CompareQuery) (map[string]any, error) {
	return CompareViewScored(s, q, Scoring{})
}

// CompareViewScored is the answer of wow_compare_item.
func CompareViewScored(s snapshot.Snapshot, q CompareQuery, sc Scoring) (map[string]any, error) {
	r, err := CompareItem(s, q, sc)
	if err != nil {
		return nil, err
	}
	return r.Render(s), nil
}

func (r ItemComparison) Render(s snapshot.Snapshot) map[string]any {
	if len(r.Choices) > 0 {
		return renderChoices(r.Choices)
	}
	it := r.Candidate.Item
	cand := it.summary()
	cand["source"] = r.Candidate.Source
	cand["levelOk"] = r.LevelOk
	if it.HasPrice {
		cand["price"] = snapshot.Money(it.Price).String()
		cand["canAfford"] = r.CanAfford
	}
	comparisons := renderComparisons(r.Slots)
	out := map[string]any{
		"item":        cand,
		"comparisons": comparisons,
		"note":        "Delta = item minus equipped (positive = gain). The numbers come from the stats and tooltip in the dump; Equip/Use effects appear only in the tooltip.",
	}
	r.Scores.addTo(out, comparisons)
	if ws := r.WeaponSkill; ws != nil {
		out["weaponSkill"] = map[string]any{"name": ws.Name, "rank": ws.Rank, "maxRank": ws.MaxRank}
	}
	it.passThrough(cand, "tooltip")
	if r.EnglishTooltip {
		s.AddEnglishOnlyNote(out, "This item's weapon speed, damage or armor are read from English tooltip lines, because the dump has no numeric speed and stat tokens for it")
	}
	return out
}

func renderChoices(choices []FoundItem) map[string]any {
	names := make([]string, 0, len(choices))
	for _, f := range choices {
		names = append(names, fmt.Sprintf("%s (%s)", f.Item.nameLabel(), f.Source))
	}
	sort.Strings(names)
	return map[string]any{"ambiguous": true, "matches": names,
		"note": "Several items match. Call the tool again with the full name or ID."}
}

func (sc SlotScores) addTo(out map[string]any, comparisons []any) {
	if sc.Note != "" {
		out["weightsNote"] = sc.Note
		return
	}
	if !sc.Weighted() {
		return
	}
	out["weights"] = map[string]any{"source": sc.Source, "values": sc.Weights}
	for i, score := range sc.Scores {
		snapshot.AsMap(comparisons[i])["weightedDelta"] = score
	}
	if sc.HasBest {
		out["bestWeighted"] = map[string]any{"slot": sc.BestSlot, "weightedDelta": sc.Best}
	}
	if len(sc.Ignored) > 0 {
		out["weightsIgnored"] = sc.Ignored
		out["weightsIgnoredNote"] = "stats without a weight are not included in weightedDelta"
	}
}

// BagSpace returns the total of bag slots.
func BagSpace(s snapshot.Snapshot) map[string]any { return Space(s).render() }

func (b BagSlots) render() map[string]any {
	return map[string]any{"totalSlots": b.Total, "freeSlots": b.Free}
}

// SummarizeBags adds the slot totals, junk and ammo to the wow_bags response.
func SummarizeBags(s snapshot.Snapshot, out map[string]any) {
	r := InspectBags(s)
	snapshot.Merge(out, r.BagSlots.render())
	snapshot.Merge(out, r.renderExtras())
}

func bagExtras(s snapshot.Snapshot) map[string]any { return InspectBags(s).renderExtras() }

func (r BagContents) renderExtras() map[string]any {
	out := map[string]any{}
	if len(r.Junk) > 0 {
		names := make([]string, 0, len(r.Junk))
		for _, it := range r.Junk {
			names = append(names, it.nameLabel())
		}
		out["junk"] = map[string]any{"items": names, "vendorValue": snapshot.Money(r.JunkValue).String()}
	}
	if r.Ammo != nil {
		out["ammo"] = map[string]any{"name": r.Ammo.raw["name"], "count": r.Ammo.raw["count"]}
	}
	return out
}

// Bank describes the bank from the last visit: items with their vendor value and free slots.
func Bank(s snapshot.Snapshot, q BankQuery, fresh snapshot.Freshness) map[string]any {
	return InspectBank(s, q).Render(fresh)
}

func (r BankReport) Render(fresh snapshot.Freshness) map[string]any {
	if !r.Saved {
		return map[string]any{"note": "No saved bank. Open the bank in game, then type /mcp reload."}
	}
	items := []any{}
	for _, e := range r.Entries {
		entry := e.copyFields()
		entry["container"] = e.Container
		if name := e.QualityName(); name != "" {
			entry["qualityName"] = name
		}
		if e.SellPrice > 0 {
			entry["vendorValue"] = snapshot.Money(e.SellValue()).String()
		}
		items = append(items, entry)
	}
	out := map[string]any{
		"items":            items,
		"count":            len(items),
		"totalSlots":       r.TotalSlots,
		"freeSlots":        r.FreeSlots,
		"totalVendorValue": snapshot.Money(r.TotalValue).String(),
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(r.Captured))
	return out
}

// Buyback describes the buyback list from the last vendor window: what can still be bought back, for how much,
// whether the character can afford it and how long ago the list was saved. nil when the dump has no section.
func Buyback(s snapshot.Snapshot, query string, fresh snapshot.Freshness) map[string]any {
	r, ok := InspectBuyback(s, query)
	if !ok {
		return nil
	}
	return r.Render(fresh)
}

func (r BuybackReport) Render(fresh snapshot.Freshness) map[string]any {
	list := []any{}
	for _, l := range r.Lines {
		e := map[string]any{"name": l.Name}
		l.passThrough(e, "index", "id", "count")
		if l.HasPrice {
			e["priceCopper"] = l.Price
			e["price"] = snapshot.Money(l.Price).String()
			e["canAfford"] = l.CanAfford
		}
		list = append(list, e)
	}
	out := map[string]any{"npc": r.Npc, "items": list, "count": len(list), "assumption": buybackAssumption}
	if len(list) > 0 {
		out["totalPrice"] = snapshot.Money(r.Total).String()
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(r.Captured))
	if r.HasAt && fresh.Now != nil {
		out["capturedAge"] = snapshot.FormatAge(fresh.Age(r.At))
	}
	return out
}

// Vendor describes the offer of the most recently opened vendor (or the one named in the query).
func Vendor(s snapshot.Snapshot, q VendorQuery, fresh snapshot.Freshness) map[string]any {
	return InspectVendor(s, q).Render(fresh)
}

func (r VendorReport) Render(fresh snapshot.Freshness) map[string]any {
	if r.UnknownNpc != "" {
		return map[string]any{"note": "I do not remember a vendor " + r.UnknownNpc + ". The remembered ones are in the known field; open that vendor's window in game and type /mcp reload.",
			"known": r.Known}
	}
	if !r.Saved {
		return map[string]any{"note": "No saved vendor offer. Open a vendor window in game, then type /mcp reload."}
	}
	items := []any{}
	for _, o := range r.Offers {
		entry := o.copyFields()
		if o.HasPrice {
			entry["price"] = snapshot.Money(o.Price).String()
		}
		o.Offer.addTo(entry)
		items = append(items, entry)
	}
	out := map[string]any{
		"npc":       r.Vendor.Npc,
		"canRepair": r.Vendor.CanRepair,
		"money":     snapshot.Money(r.Money).String(),
		"items":     items,
		"count":     len(items),
	}
	if r.Known != nil {
		out["known"] = r.Known
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(r.Vendor.Captured))
	if r.Buyback != nil {
		out["buyback"] = r.Buyback.Render(fresh)
	}
	return out
}

// Judge adds to the entry what the buyer wants to know about the offer: canAfford (when there is a price),
// levelOk, the quality name and, for equippable items, vsEquipped: the difference to what is worn.
func (b Buyer) Judge(entry, item map[string]any, price int64, hasPrice bool) {
	b.Assess(readItem(item), price, hasPrice).addTo(entry)
}

func (o Offer) addTo(entry map[string]any) {
	if o.HasPrice {
		entry["canAfford"] = o.CanAfford
	}
	entry["levelOk"] = o.LevelOk
	if o.Quality != "" {
		entry["qualityName"] = o.Quality
	}
	if o.VsEquipped != nil {
		entry["vsEquipped"] = o.VsEquipped.render()
	}
}

// UpgradeScore ranks the vsEquipped comparison of Judge (Offer.UpgradeScore on an answer entry).
func UpgradeScore(entry map[string]any) float64 {
	vs := snapshot.AsMap(entry["vsEquipped"])
	if vs == nil {
		return Offer{}.UpgradeScore()
	}
	delta, _ := vs["delta"].(map[string]float64)
	return SlotComparison{Delta: delta}.UpgradeScore()
}

// Describe describes the vendor for the response: who, the price of a pack and of a unit.
func (sl Seller) Describe() map[string]any {
	return map[string]any{"npc": sl.Npc, "pack": snapshot.Money(sl.PriceCopper).String(), "stack": sl.Stack,
		"unitPrice": fmt.Sprintf("%.1f c", sl.UnitCopper)}
}

// FindOnAccount searches for an item across all characters on the account at once:
// bags, equipped gear, bank and mail, with a total for each item.
func FindOnAccount(db *snapshot.Database, q FindQuery, fresh snapshot.Freshness) (map[string]any, error) {
	r, err := SearchAccount(db, q)
	if err != nil {
		return nil, err
	}
	return r.Render(), nil
}

func (r AccountSearch) Render() map[string]any {
	results := []any{}
	for _, loc := range r.Locations {
		results = append(results, loc.render())
	}
	out := map[string]any{
		"query":   r.Query,
		"matches": results,
		"count":   len(results),
		"totals":  r.Totals,
		"total":   r.Total,
		"searched": map[string]any{
			"characters": r.Characters,
			"where":      "bags, equipment, bank (state from the last visit) and mail (state from the last time the mailbox was opened)",
		},
	}
	if len(results) == 0 {
		out["note"] = "Nothing found. The bank and mail appear in the dump only after they are opened in game and /mcp reload is run. " + snapshot.AltsHint
	}
	return out
}

func (l Location) render() map[string]any {
	e := map[string]any{"where": l.Where, "name": l.Item.raw["name"], "id": l.Item.raw["id"], "count": l.Item.Count, "character": l.Character}
	switch l.Where {
	case InBags:
		e["container"] = l.Container
	case InBank:
		e["container"] = l.Container
		if l.SeenAt.Known {
			e["seenAt"] = snapshot.FormatTime(l.SeenAt.At)
		}
	case IsEquipped:
		e["slot"] = l.Item.raw["slot"]
	case InMail:
		e["from"], e["subject"] = l.From, l.Subject
	}
	return e
}
