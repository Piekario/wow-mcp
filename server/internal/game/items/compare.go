// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/snapshot"
)

// slotsFor maps an item's equipLoc to the addon's equipment slots (names from EQUIPMENT_SLOTS).
func slotsFor(equipLoc string) []string {
	switch equipLoc {
	case "INVTYPE_HEAD":
		return []string{"head"}
	case "INVTYPE_NECK":
		return []string{"neck"}
	case "INVTYPE_SHOULDER":
		return []string{"shoulder"}
	case "INVTYPE_BODY":
		return []string{"shirt"}
	case "INVTYPE_CHEST", "INVTYPE_ROBE":
		return []string{"chest"}
	case "INVTYPE_WAIST":
		return []string{"waist"}
	case "INVTYPE_LEGS":
		return []string{"legs"}
	case "INVTYPE_FEET":
		return []string{"feet"}
	case "INVTYPE_WRIST":
		return []string{"wrist"}
	case "INVTYPE_HAND":
		return []string{"hands"}
	case "INVTYPE_FINGER":
		return []string{"finger1", "finger2"}
	case "INVTYPE_TRINKET":
		return []string{"trinket1", "trinket2"}
	case "INVTYPE_CLOAK":
		return []string{"back"}
	case "INVTYPE_WEAPON":
		return []string{"mainHand", "offHand"}
	case "INVTYPE_2HWEAPON", "INVTYPE_WEAPONMAINHAND":
		return []string{"mainHand"}
	case "INVTYPE_WEAPONOFFHAND", "INVTYPE_SHIELD", "INVTYPE_HOLDABLE":
		return []string{"offHand"}
	case "INVTYPE_RANGED", "INVTYPE_RANGEDRIGHT", "INVTYPE_THROWN", "INVTYPE_RELIC":
		return []string{"ranged"}
	case "INVTYPE_TABARD":
		return []string{"tabard"}
	}
	return nil
}

// Equippable says whether the item can be equipped (equipLoc matches an equipment slot).
func Equippable(item map[string]any) bool {
	equipLoc, _ := item["equipLoc"].(string)
	return len(slotsFor(equipLoc)) > 0
}

// tokenNames maps the client's stat tokens (the keys of GetItemStats, e.g. ITEM_MOD_STAMINA_SHORT) to the names
// the server uses: the labels an English client shows for them, so that comparisons from a localized client
// (statTokens, addon 0.9.0) use the same words as old dumps from English clients. Tokens not listed here
// get a name built from the token itself (statName).
var tokenNames = map[string]string{
	"ITEM_MOD_STRENGTH_SHORT": "Strength", "ITEM_MOD_AGILITY_SHORT": "Agility", "ITEM_MOD_STAMINA_SHORT": "Stamina",
	"ITEM_MOD_INTELLECT_SHORT": "Intellect", "ITEM_MOD_SPIRIT_SHORT": "Spirit",
	"ITEM_MOD_ATTACK_POWER_SHORT": "Attack Power", "ITEM_MOD_RANGED_ATTACK_POWER_SHORT": "Ranged Attack Power",
	"ITEM_MOD_SPELL_POWER_SHORT": "Spell Power", "ITEM_MOD_DAMAGE_PER_SECOND_SHORT": "DPS",
	"RESISTANCE0_NAME": "Armor", "RESISTANCE1_NAME": "Holy Resistance", "RESISTANCE2_NAME": "Fire Resistance",
	"RESISTANCE3_NAME": "Nature Resistance", "RESISTANCE4_NAME": "Frost Resistance", "RESISTANCE5_NAME": "Shadow Resistance",
	"RESISTANCE6_NAME": "Arcane Resistance",
}

// statName turns a stat key into a readable name: ITEM_MOD_STAMINA_SHORT -> Stamina.
func statName(key string) string {
	if name, ok := tokenNames[key]; ok {
		return name
	}
	if strings.EqualFold(key, "damage per second") { // real dump: DPS sits in stats under this name
		return "DPS"
	}
	if !strings.HasPrefix(key, "ITEM_MOD_") {
		return key
	}
	k := strings.TrimSuffix(strings.TrimPrefix(key, "ITEM_MOD_"), "_SHORT")
	words := strings.Split(strings.ToLower(k), "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

var (
	reDamage = regexp.MustCompile(`(\d+)\s*-\s*(\d+)\s+Damage`)
	reSpeed  = regexp.MustCompile(`Speed\s+(\d+(?:[.,]\d+)?)`)
	reDPS    = regexp.MustCompile(`\((\d+(?:[.,]\d+)?)\s+damage per second\)`)
	reArmor  = regexp.MustCompile(`^(\d+)\s+Armor`)
)

// itemNumbers collects an item's numbers for comparison: stats, DPS and armor from the tooltip.
func itemNumbers(item map[string]any) map[string]float64 {
	nums := map[string]float64{}
	// statTokens (addon 0.9.0) has the client's stat tokens, the same in every language; stats has the labels the
	// client shows (English only on an English client), so it is the fallback for older dumps.
	stats := snapshot.AsMap(item["statTokens"])
	if len(stats) == 0 {
		stats = snapshot.AsMap(item["stats"])
	}
	for k, v := range stats {
		if f, ok := snapshot.AsFloat(v); ok {
			nums[statName(k)] += f
		}
	}
	// Weapon numbers from the addon (0.10.0, read with the client's own format strings): numeric speed, damage range and
	// DPS. The English tooltip lines below are only the fallback for dumps from older addons.
	lo, _ := snapshot.AsFloat(item["damageMin"])
	hi, _ := snapshot.AsFloat(item["damageMax"])
	speed, _ := snapshot.AsFloat(item["speed"])
	dps, hasDPS := snapshot.AsFloat(item["dps"])
	if hasDPS {
		nums["DPS"] = dps
	}
	for _, raw := range snapshot.AsList(item["tooltip"]) {
		line, _ := raw.(string)
		if m := reDamage.FindStringSubmatch(line); m != nil && lo == 0 {
			lo, _ = strconv.ParseFloat(m[1], 64)
			hi, _ = strconv.ParseFloat(m[2], 64)
		}
		if m := reSpeed.FindStringSubmatch(line); m != nil && speed == 0 {
			speed, _ = strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		}
		if m := reDPS.FindStringSubmatch(line); m != nil && !hasDPS {
			nums["DPS"], _ = strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		}
		if m := reArmor.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			nums["Armor"], _ = strconv.ParseFloat(m[1], 64)
		}
	}
	if _, ok := nums["DPS"]; !ok && hi > 0 && speed > 0 {
		nums["DPS"] = math.Round((lo+hi)/2/speed*10) / 10
	}
	if speed > 0 {
		nums["Speed"] = speed
	}
	if il, ok := snapshot.AsFloat(item["itemLevel"]); ok {
		nums["Item Level"] = il
	}
	return nums
}

// Summary is a short item description with the numbers for comparison.
func Summary(item map[string]any) map[string]any {
	if item == nil {
		return nil
	}
	out := map[string]any{"name": item["name"], "id": item["id"], "numbers": itemNumbers(item)}
	for _, k := range []string{"itemLevel", "requiredLevel", "subType", "equipLoc"} {
		if v, ok := item[k]; ok {
			out[k] = v
		}
	}
	if q := qualityName(item); q != "" {
		out["quality"] = q
	}
	return out
}

// Compare compares an item with what is equipped in each slot it fits.
// A two-handed weapon is compared with the sum of mainHand + offHand, because it removes both.
func Compare(item map[string]any, equipment []any) []any {
	equipLoc, _ := item["equipLoc"].(string)
	slots := slotsFor(equipLoc)
	if len(slots) == 0 {
		return nil
	}
	worn := snapshot.EquippedBySlot(equipment)
	cand := itemNumbers(item)
	_, hasTooltip := item["tooltip"]
	var out []any
	for _, slot := range slots {
		replaced := []map[string]any{}
		if w := worn[slot]; w != nil {
			replaced = append(replaced, w)
		}
		if equipLoc == "INVTYPE_2HWEAPON" {
			if off := worn["offHand"]; off != nil {
				replaced = append(replaced, off)
			}
		}
		current, names := wornNumbers(replaced)
		entry := map[string]any{"slot": slot, "delta": numbersDelta(cand, current, hasTooltip)}
		if len(replaced) == 0 {
			entry["equipped"] = snapshot.Empty
		} else {
			entry["equipped"] = strings.Join(names, " + ")
			entry["equippedNumbers"] = current
		}
		out = append(out, entry)
	}
	return out
}

// wornNumbers sums the numbers of the items being removed; speed and item level
// make sense only for a single item, so they are skipped for two.
func wornNumbers(replaced []map[string]any) (map[string]float64, []string) {
	current := map[string]float64{}
	var names []string
	for _, w := range replaced {
		names = append(names, fmt.Sprint(w["name"]))
		for k, v := range itemNumbers(w) {
			if k == "Speed" || k == "Item Level" {
				if len(replaced) == 1 {
					current[k] = v
				}
				continue
			}
			current[k] += v
		}
	}
	return current, names
}

// numbersDelta is the candidate minus the equipped item, rounded to 0.1, without zeros.
func numbersDelta(cand, current map[string]float64, hasTooltip bool) map[string]float64 {
	keys := map[string]bool{}
	for k := range cand {
		keys[k] = true
	}
	for k := range current {
		keys[k] = true
	}
	delta := map[string]float64{}
	for k := range keys {
		if k == "Speed" {
			continue // speed is neither gain nor loss; it is in both items' numbers
		}
		if _, known := cand[k]; (k == "DPS" || k == "Armor") && !hasTooltip && !known {
			continue // without a tooltip or numbers from the addon we do not know the candidate's damage or armor
		}
		if d := math.Round((cand[k]-current[k])*10) / 10; d != 0 {
			delta[k] = d
		}
	}
	return delta
}

// bestComparison reduces the comparison to the best slot: change in item level, DPS and stats.
func bestComparison(cmp []any) map[string]any {
	var best map[string]any
	bestScore := math.Inf(-1)
	for _, raw := range cmp {
		c := snapshot.AsMap(raw)
		if score := comparisonScore(c); score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

// weaponSkills maps a weapon subclass (Enum.ItemWeaponSubclass) to its weapon skill line ID (the same IDs the addon
// uses for the weapon skills in the professions section).
var weaponSkills = map[int64]int64{
	0: 44, 1: 172, 2: 45, 3: 46, 4: 54, 5: 160, 6: 229, 7: 43, 8: 55, 10: 136, 13: 473, 15: 173, 16: 176, 18: 226, 19: 228,
}

// weaponSkillFor looks for the character's weapon skill for a weapon: by the weapon subclass and skill line ID
// (addon 0.9.0), otherwise by the English subtype name, e.g. Dagger -> Daggers.
func weaponSkillFor(s snapshot.Snapshot, item map[string]any) map[string]any {
	if sub, ok := itemkind.SubclassOf(item); ok && itemkind.HasIDs(item) {
		want, known := weaponSkills[sub]
		for _, raw := range s.List("professions") {
			p := snapshot.AsMap(raw)
			if id, ok := snapshot.AsInt(p["skillID"]); ok && known && id == want {
				return map[string]any{"name": p["name"], "rank": p["rank"], "maxRank": p["maxRank"]}
			}
		}
	}
	subType, _ := item["subType"].(string)
	if subType == "" {
		return nil
	}
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.TrimPrefix(s, "one-handed ")
		return strings.TrimSuffix(s, "s")
	}
	want := norm(subType)
	for _, raw := range s.List("professions") {
		p := snapshot.AsMap(raw)
		group, _ := p["group"].(string)
		if !strings.EqualFold(group, "weapon") && !strings.EqualFold(group, "Weapon Skills") {
			continue
		}
		if name, _ := p["name"].(string); norm(name) == want {
			return map[string]any{"name": name, "rank": p["rank"], "maxRank": p["maxRank"]}
		}
	}
	return nil
}

// Found is an item found on a character and the place it comes from (bags, bank, vendor).
type Found struct {
	Item   map[string]any
	Source string
}

// Find looks for an item in bags, bank and at a vendor; source narrows it to one place.
// An empty name matches everything. An exact name beats a fragment.
func Find(s snapshot.Snapshot, query, source string) []Found {
	match := matcher(query)
	var found []Found
	add := func(src string, items []any) {
		if source != "" && !strings.EqualFold(source, src) {
			return
		}
		for _, raw := range items {
			if item := snapshot.AsMap(raw); match(item) {
				found = append(found, Found{item, src})
			}
		}
	}
	for _, bag := range s.Bags() {
		add("bags", snapshot.AsList(snapshot.AsMap(bag)["items"]))
	}
	for _, c := range s.BankContainers() {
		add("bank", snapshot.AsList(snapshot.AsMap(c)["items"]))
	}
	add("vendor", snapshot.AsList(s.Map("vendor")["items"]))

	var exact []Found
	for _, f := range found {
		if name, _ := f.Item["name"].(string); strings.EqualFold(name, query) {
			exact = append(exact, f)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return found
}

// CompareQuery is the input of wow_compare_item.
type CompareQuery struct {
	Item      string `json:"item" jsonschema:"item name (or a fragment of it) or its ID"`
	Source    string `json:"source,omitempty" jsonschema:"where to look: bags, bank, vendor; empty = everywhere"`
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	// Weights are the player's stat weights for scoring the difference; empty = weights from weights.json (for the character or class), and without them just the difference.
	Weights map[string]float64 `json:"weights,omitempty" jsonschema:"optional stat weights, e.g. {\"agility\":1,\"attackPower\":0.5}; each comparison then gets weightedDelta (sum of difference x weight). Without it the server uses weights.json if the player has one"`
}

// Scoring says where to take stat weights from when the query gives none.
type Scoring struct {
	Store WeightsStore // nil = no weights file
	Key   string       // the character key in the dump (to choose weights from the file)
}

// CompareView compares an item without file weights (only weights from the query).
func CompareView(s snapshot.Snapshot, q CompareQuery) (map[string]any, error) {
	return CompareViewScored(s, q, Scoring{})
}

// CompareViewScored compares one item from bags, bank or a vendor with the equipped
// gear; when several different items match, it returns a list to choose from. When stat weights exist
// (from the query or the file), each comparison gets weightedDelta.
func CompareViewScored(s snapshot.Snapshot, q CompareQuery, sc Scoring) (map[string]any, error) {
	if strings.TrimSpace(q.Item) == "" {
		return nil, fmt.Errorf("give an item name or ID in the item parameter")
	}
	found := Find(s, q.Item, q.Source)
	if len(found) == 0 {
		return nil, fmt.Errorf("%q not found in bags, bank or at a vendor. The bank and vendor appear in the dump only after their window is opened and /mcp reload is run", q.Item)
	}
	if choices := ambiguous(found); choices != nil {
		return choices, nil
	}
	f := found[0]
	if !Equippable(f.Item) {
		return nil, fmt.Errorf("%v cannot be equipped (no equipLoc in the dump), so there is nothing to compare with", f.Item["name"])
	}
	level, money := s.LevelAndMoney()
	cand := Summary(f.Item)
	cand["source"] = f.Source
	req, _ := snapshot.AsInt(f.Item["requiredLevel"])
	cand["levelOk"] = req <= level
	if price, ok := snapshot.AsInt(f.Item["priceCopper"]); ok {
		cand["price"] = snapshot.Money(price).String()
		cand["canAfford"] = price <= money
	}
	comparisons := Compare(f.Item, s.Equipment())
	out := map[string]any{
		"item":        cand,
		"comparisons": comparisons,
		"note":        "Delta = item minus equipped (positive = gain). The numbers come from the stats and tooltip in the dump; Equip/Use effects appear only in the tooltip.",
	}
	if err := scoreComparisons(out, comparisons, s, q, sc); err != nil {
		return nil, err
	}
	if itemkind.Is(f.Item, itemkind.ClassWeapon) {
		if ws := weaponSkillFor(s, f.Item); ws != nil {
			out["weaponSkill"] = ws
		}
	}
	if tt, ok := f.Item["tooltip"]; ok {
		cand["tooltip"] = tt
	}
	if readsEnglishTooltip(f.Item) {
		s.AddEnglishOnlyNote(out, "This item's weapon speed, damage or armor are read from English tooltip lines, because the dump has no numeric speed and stat tokens for it")
	}
	return out, nil
}

// readsEnglishTooltip says whether itemNumbers has to read English tooltip text for this item: a weapon without the
// numeric speed of addon 0.10.0, or an item with a tooltip but without statTokens (damage and armor then come from text).
func readsEnglishTooltip(item map[string]any) bool {
	if len(snapshot.AsList(item["tooltip"])) == 0 {
		return false
	}
	if _, ok := snapshot.AsFloat(item["speed"]); !ok && itemkind.Is(item, itemkind.ClassWeapon) {
		return true
	}
	return len(snapshot.AsMap(item["statTokens"])) == 0
}

// ambiguous returns a list to choose from when different items match the query; nil when there is one.
func ambiguous(found []Found) map[string]any {
	distinct := map[string]Found{}
	for _, f := range found {
		distinct[fmt.Sprint(f.Item["name"])] = f
	}
	if len(distinct) <= 1 {
		return nil
	}
	names := make([]string, 0, len(distinct))
	for n, f := range distinct {
		names = append(names, fmt.Sprintf("%s (%s)", n, f.Source))
	}
	sort.Strings(names)
	return map[string]any{"ambiguous": true, "matches": names,
		"note": "Several items match. Call the tool again with the full name or ID."}
}

// PossibleUpgrade says whether an equippable item looks like an upgrade over what is equipped in the matching slot
// (higher item level or DPS) and for which slot; an empty slot is also an upgrade. false when it cannot be
// equipped or the character's level is too low for it. This is only a hint: the player decides about stats.
func PossibleUpgrade(s snapshot.Snapshot, item map[string]any) (slot string, ok bool) {
	if !Equippable(item) {
		return "", false
	}
	level, _ := s.LevelAndMoney()
	if req, _ := snapshot.AsInt(item["requiredLevel"]); req > level {
		return "", false
	}
	for _, raw := range Compare(item, s.Equipment()) {
		c := snapshot.AsMap(raw)
		if c["equipped"] == snapshot.Empty {
			return fmt.Sprint(c["slot"]), true
		}
		if delta, _ := c["delta"].(map[string]float64); delta["Item Level"] > 0 || delta["DPS"] > 0 {
			return fmt.Sprint(c["slot"]), true
		}
	}
	return "", false
}

// scoreComparisons adds weightedDelta to the comparisons when weights exist: from the query, then from the file.
// A corrupt weights file does not break the comparison, it only adds weightsNote.
func scoreComparisons(out map[string]any, comparisons []any, s snapshot.Snapshot, q CompareQuery, sc Scoring) error {
	weights, source, note, err := ChooseWeights(q.Weights, sc, s)
	if err != nil {
		return err
	}
	if note != "" {
		out["weightsNote"] = note
		return nil
	}
	if len(weights) == 0 {
		return nil
	}
	out["weights"] = map[string]any{"source": source, "values": weights}
	var bestSlot any
	best := math.Inf(-1)
	ignored := map[string]bool{}
	for _, raw := range comparisons {
		c := snapshot.AsMap(raw)
		delta, _ := c["delta"].(map[string]float64)
		score, unweighted := weights.score(delta)
		c["weightedDelta"] = score
		for _, stat := range unweighted {
			ignored[stat] = true
		}
		if score > best {
			best, bestSlot = score, c["slot"]
		}
	}
	if bestSlot != nil {
		out["bestWeighted"] = map[string]any{"slot": bestSlot, "weightedDelta": best}
	}
	if len(ignored) > 0 {
		names := make([]string, 0, len(ignored))
		for n := range ignored {
			names = append(names, n)
		}
		sort.Strings(names)
		out["weightsIgnored"] = names
		out["weightsIgnoredNote"] = "stats without a weight are not included in weightedDelta"
	}
	return nil
}
