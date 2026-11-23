// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package character is the character sheet: gold, play time, durability and average
// gear level, addon version and the overview of all characters on the account.
package character

import (
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// NearlyBrokenPercent: below this durability an item goes on the repair list.
const NearlyBrokenPercent = 25

type PlayTime struct {
	TotalSeconds int64
	LevelSeconds int64
	HasLevel     bool
}

// Gear is one equipped item as far as the character's rules need it.
type Gear struct {
	Slot              string
	Name              any
	ItemLevel         float64
	HasItemLevel      bool
	WholeItemLevel    int64
	HasWholeItemLevel bool
	Durability        int64
	MaxDurability     int64
	HasDurability     bool
}

func (g Gear) countsForAverage() bool {
	return g.Slot != "shirt" && g.Slot != "tabard" && g.Slot != "ammo" && g.HasItemLevel
}

func averageItemLevel(gear []Gear) (float64, bool) {
	var sum, n float64
	for _, g := range gear {
		if g.countsForAverage() {
			sum += g.ItemLevel
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return snapshot.Round1(sum / n), true
}

// AverageItemLevel is the average item level of the equipped gear (without shirt, tabard and ammo).
func AverageItemLevel(s snapshot.Snapshot) (float64, bool) { return averageItemLevel(readGear(s)) }

// PlayTimeOf is the played time from /played; false when the dump has none.
func PlayTimeOf(s snapshot.Snapshot) (PlayTime, bool) { return readPlayTime(s) }

type WornItem struct {
	Name    any
	Percent float64
}

type RepairQuote struct {
	CostCopper int64
	NPC        any
	Captured   snapshot.Captured
}

// DurabilityReport is the state of the gear's durability: the average, the weakest item and the repair cost quoted
// by the last vendor.
type DurabilityReport struct {
	Percent      float64
	Lowest       WornItem
	NearlyBroken []WornItem
	Repair       RepairQuote
	HasRepair    bool
}

// InspectDurability is false when no equipped item has durability.
func InspectDurability(s snapshot.Snapshot) (DurabilityReport, bool) {
	repair, hasRepair := readRepairQuote(s)
	return judgeDurability(readGear(s), repair, hasRepair)
}

func judgeDurability(gear []Gear, repair RepairQuote, hasRepair bool) (DurabilityReport, bool) {
	var cur, max int64
	lowest := WornItem{Percent: 101}
	var report DurabilityReport
	for _, g := range gear {
		if !g.HasDurability {
			continue
		}
		cur += g.Durability
		max += g.MaxDurability
		item := WornItem{g.Name, float64(g.Durability*1000/g.MaxDurability) / 10}
		if item.Percent < lowest.Percent {
			lowest = item
		}
		if item.Percent < NearlyBrokenPercent {
			report.NearlyBroken = append(report.NearlyBroken, item)
		}
	}
	if max == 0 {
		return DurabilityReport{}, false
	}
	report.Percent = float64(cur*1000/max) / 10
	report.Lowest = lowest
	report.Repair, report.HasRepair = repair, hasRepair
	return report, true
}

type PlayerNote struct {
	Text     string
	SetAt    int64
	HasSetAt bool
}

// InspectNote is false when the player left no note or it is blank.
func InspectNote(s snapshot.Snapshot) (PlayerNote, bool) {
	n := readNote(s)
	return n, strings.TrimSpace(n.Text) != ""
}

// NoteText is just the note text ("" when there is none); to be included in prompts as the session goal.
func NoteText(s snapshot.Snapshot) string { return strings.TrimSpace(readNote(s).Text) }

type Death struct {
	Zone, SubZone, Level any
	At                   int64
	HasAt                bool
}

// DeathRecord is the deaths section: the counter and the last death.
type DeathRecord struct {
	Count   int64
	Last    Death
	HasLast bool
}

// InspectDeaths is false without a deaths section (old addon or disabled).
func InspectDeaths(s snapshot.Snapshot) (DeathRecord, bool) { return readDeaths(s) }

// DeathCount is the character's death counter; false when the dump has no deaths section (old addon or disabled).
func DeathCount(s snapshot.Snapshot) (int64, bool) {
	d, ok := readDeaths(s)
	return d.Count, ok
}

// Sheet is what wow_character adds to the section summary.
type Sheet struct {
	Money     int64
	HasMoney  bool
	PlayTime  PlayTime
	HasPlayed bool
	Note      PlayerNote
	HasNote   bool
	Deaths    DeathRecord
	HasDeaths bool
}

func InspectSheet(s snapshot.Snapshot) Sheet {
	var sheet Sheet
	sheet.Money, sheet.HasMoney = readSheetMoney(s)
	sheet.PlayTime, sheet.HasPlayed = readPlayTime(s)
	sheet.Note, sheet.HasNote = InspectNote(s)
	sheet.Deaths, sheet.HasDeaths = readDeaths(s)
	return sheet
}

type Overview struct {
	Name           string
	Level, Money   int64
	Class          any
	Race           any
	Zone           any
	Professions    []string
	XPPercent      float64
	HasXPPercent   bool
	RestedXP       int64
	FreeBagSlots   int64
	HasBagSpace    bool
	PlayedSeconds  int64
	HasPlayed      bool
	MailMessages   int
	MailExpiring   []string
	CooldownsReady []string
}

// AccountReport lists the characters of the account, highest level first.
type AccountReport struct {
	Characters []Overview
	TotalMoney int64
	Last       string
}

func InspectAccount(db *snapshot.Database, fresh snapshot.Freshness) AccountReport {
	names := db.CharacterNames()
	sort.SliceStable(names, func(i, j int) bool {
		li, _ := db.Characters[names[i]].LevelAndMoney()
		lj, _ := db.Characters[names[j]].LevelAndMoney()
		return li > lj
	})
	report := AccountReport{Last: db.Last}
	for _, name := range names {
		o := readOverview(name, db.Characters[name], fresh)
		report.TotalMoney += o.Money
		report.Characters = append(report.Characters, o)
	}
	return report
}
