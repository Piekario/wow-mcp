// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"wowmcp/internal/game/itemkind"
)

// The names of the numbers the comparison treats specially.
const (
	StatItemLevel = "Item Level"
	StatDPS       = "DPS"
	StatSpeed     = "Speed"
	StatArmor     = "Armor"
)

// tokenNames maps the client's stat tokens (the keys of GetItemStats, e.g. ITEM_MOD_STAMINA_SHORT) to the names
// the server uses: the labels an English client shows for them, so that comparisons from a localized client
// (statTokens, addon 0.9.0) use the same words as old dumps from English clients. Tokens not listed here
// get a name built from the token itself (statName).
var tokenNames = map[string]string{
	"ITEM_MOD_STRENGTH_SHORT": "Strength", "ITEM_MOD_AGILITY_SHORT": "Agility", "ITEM_MOD_STAMINA_SHORT": "Stamina",
	"ITEM_MOD_INTELLECT_SHORT": "Intellect", "ITEM_MOD_SPIRIT_SHORT": "Spirit",
	"ITEM_MOD_ATTACK_POWER_SHORT": "Attack Power", "ITEM_MOD_RANGED_ATTACK_POWER_SHORT": "Ranged Attack Power",
	"ITEM_MOD_SPELL_POWER_SHORT": "Spell Power", "ITEM_MOD_DAMAGE_PER_SECOND_SHORT": StatDPS,
	"RESISTANCE0_NAME": StatArmor, "RESISTANCE1_NAME": "Holy Resistance", "RESISTANCE2_NAME": "Fire Resistance",
	"RESISTANCE3_NAME": "Nature Resistance", "RESISTANCE4_NAME": "Frost Resistance", "RESISTANCE5_NAME": "Shadow Resistance",
	"RESISTANCE6_NAME": "Arcane Resistance",
}

// statName turns a stat key into a readable name: ITEM_MOD_STAMINA_SHORT -> Stamina.
func statName(key string) string {
	if name, ok := tokenNames[key]; ok {
		return name
	}
	if strings.EqualFold(key, "damage per second") { // real dump: DPS sits in stats under this name
		return StatDPS
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

// Numbers are the item's numbers for comparison: stats, DPS, speed, armor and item level.
func (i Item) Numbers() map[string]float64 {
	nums := map[string]float64{}
	for k, v := range i.Stats {
		nums[statName(k)] += v
	}
	w := i.Weapon
	lo, hi, speed := w.DamageMin, w.DamageMax, w.Speed
	if w.HasDPS {
		nums[StatDPS] = w.DPS
	}
	// The English tooltip lines are only the fallback for dumps from addons before 0.10.0.
	for _, line := range i.Tooltip {
		if m := reDamage.FindStringSubmatch(line); m != nil && lo == 0 {
			lo, _ = strconv.ParseFloat(m[1], 64)
			hi, _ = strconv.ParseFloat(m[2], 64)
		}
		if m := reSpeed.FindStringSubmatch(line); m != nil && speed == 0 {
			speed, _ = strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		}
		if m := reDPS.FindStringSubmatch(line); m != nil && !w.HasDPS {
			nums[StatDPS], _ = strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
		}
		if m := reArmor.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			nums[StatArmor], _ = strconv.ParseFloat(m[1], 64)
		}
	}
	if _, ok := nums[StatDPS]; !ok && hi > 0 && speed > 0 {
		nums[StatDPS] = math.Round((lo+hi)/2/speed*10) / 10
	}
	if speed > 0 {
		nums[StatSpeed] = speed
	}
	if i.HasItemLevel {
		nums[StatItemLevel] = i.ItemLevel
	}
	return nums
}

// ReadsEnglishTooltip says whether Numbers has to read English tooltip text for this item: a weapon without the
// numeric speed of addon 0.10.0, or an item with a tooltip but without statTokens (damage and armor then come from text).
func (i Item) ReadsEnglishTooltip() bool {
	if i.tooltipLines == 0 {
		return false
	}
	if !i.Weapon.HasSpeed && i.Is(itemkind.ClassWeapon) {
		return true
	}
	return !i.HasStatTokens
}
