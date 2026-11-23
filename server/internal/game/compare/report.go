// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package compare

import (
	"wowmcp/internal/game/professions"
	"wowmcp/internal/snapshot"
)

// skewSeconds: dumps further apart than this make the comparison unfair.
const skewSeconds = 600

// Piece is an equipped item. Name stays as the dump has it.
type Piece struct {
	Slot         string
	HasSlot      bool
	Name         any
	ItemLevel    int64
	HasItemLevel bool
	Quality      int64
	HasQuality   bool
}

// Card is one character as the comparison shows it.
type Card struct {
	Name                                string
	Level                               int64
	Class, Race, Faction, Zone, SubZone any
	Money                               int64
	Professions                         []professions.Profession
	DumpedAtText                        string
	HasDumpedAtText                     bool
	AgeMinutes                          int64
	HasAge                              bool
	Stale                               bool
	RestedXP                            int64
	HasRestedXP                         bool
	PlayedSeconds                       int64
	HasPlayed                           bool
	AverageItemLevel                    float64
	GearPieces                          int
	Gear                                []Piece
	DumpedAt                            int64
	HasDumpedAt                         bool
}

func (c *Card) judgeAge(fresh snapshot.Freshness) {
	if c.HasDumpedAt {
		c.AgeMinutes, c.HasAge = int64(fresh.Age(c.DumpedAt).Minutes()), true
		c.Stale = fresh.IsStale(c.DumpedAt)
	}
}

// Better says which side has the higher item level in a slot.
type Better string

const (
	BetterA     Better = "a"
	BetterB     Better = "b"
	BetterEqual Better = "="
)

// SlotComparison is one equipment slot of both characters. A or B is nil for an empty slot; Better is
// meaningful only when Compared (gear slots, not shirt, tabard or ammo).
type SlotComparison struct {
	Slot     string
	A, B     *Piece
	Compared bool
	Better   Better
}

type EdgeKind int

const (
	HigherLevel EdgeKind = iota
	MoreGold
	HigherItemLevel
)

// Edge is one way the two characters differ. Gap is the difference in levels or copper; High and Low are the item levels.
type Edge struct {
	Kind          EdgeKind
	Leader, Other string
	Gap           int64
	High, Low     float64
}

// Skew is how far apart in time the two dumps are. Known is false when either dump has no save time.
type Skew struct {
	Known      bool
	GapSeconds int64
	Older      string
}

// Unfair: the dumps are too far apart, or the gap is unknown.
func (k Skew) Unfair() bool { return !k.Known || k.GapSeconds >= skewSeconds }

type Report struct {
	A, B  Card
	Slots []SlotComparison
	Edges []Edge
	Skew  Skew
}

// Compare puts two characters of the account side by side.
func Compare(db *snapshot.Database, q Query, fresh snapshot.Freshness) (Report, error) {
	an, bn, err := Resolve(db, q)
	if err != nil {
		return Report{}, err
	}
	a, b := readCard(an, db.Characters[an]), readCard(bn, db.Characters[bn])
	a.judgeAge(fresh)
	b.judgeAge(fresh)
	return Report{A: a, B: b, Slots: compareSlots(a, b), Edges: edges(a, b), Skew: skewOf(a, b)}, nil
}

// slotOrder is the slot order in the response; slots outside the list go last.
var slotOrder = []string{"head", "neck", "shoulder", "back", "chest", "shirt", "tabard", "wrist", "hands", "waist", "legs", "feet",
	"finger1", "finger2", "trinket1", "trinket2", "mainhand", "offhand", "ranged", "ammo"}

// notGear are the slots not counted toward the average item level.
var notGear = map[string]bool{"shirt": true, "tabard": true, "ammo": true}

// averageItemLevel is the average ilvl of equipped armor and weapon pieces (rounded to 0.1).
func averageItemLevel(gear []Piece) (avg float64, pieces int) {
	var sum int64
	for _, p := range gear {
		if !p.HasItemLevel || notGear[p.Slot] {
			continue
		}
		sum += p.ItemLevel
		pieces++
	}
	if pieces == 0 {
		return 0, 0
	}
	return float64(sum*10/int64(pieces)) / 10, pieces
}

func bySlot(gear []Piece) map[string]*Piece {
	out := map[string]*Piece{}
	for i := range gear {
		if gear[i].HasSlot {
			out[gear[i].Slot] = &gear[i]
		}
	}
	return out
}

func compareSlots(a, b Card) []SlotComparison {
	ea, eb := bySlot(a.Gear), bySlot(b.Gear)
	seen := map[string]bool{}
	var order []string
	for _, s := range slotOrder {
		if ea[s] != nil || eb[s] != nil {
			order = append(order, s)
			seen[s] = true
		}
	}
	for _, m := range []map[string]*Piece{ea, eb} {
		for s := range m {
			if !seen[s] {
				seen[s] = true
				order = append(order, s)
			}
		}
	}
	out := make([]SlotComparison, 0, len(order))
	for _, slot := range order {
		c := SlotComparison{Slot: slot, A: ea[slot], B: eb[slot], Compared: !notGear[slot]}
		if c.Compared {
			c.Better = better(ea[slot], eb[slot])
		}
		out = append(out, c)
	}
	return out
}

// better compares ilvl: A, B or equal (equal values or no data on either side).
func better(a, b *Piece) Better {
	var la, lb int64
	if a != nil {
		la = a.ItemLevel
	}
	if b != nil {
		lb = b.ItemLevel
	}
	switch {
	case la > lb:
		return BetterA
	case lb > la:
		return BetterB
	}
	return BetterEqual
}

func edges(a, b Card) []Edge {
	var out []Edge
	if a.Level != b.Level {
		out = append(out, edgeBetween(HigherLevel, a, b, a.Level > b.Level, max(a.Level-b.Level, b.Level-a.Level)))
	}
	if a.Money != b.Money {
		out = append(out, edgeBetween(MoreGold, a, b, a.Money > b.Money, max(a.Money-b.Money, b.Money-a.Money)))
	}
	if a.GearPieces > 0 && b.GearPieces > 0 && a.AverageItemLevel != b.AverageItemLevel {
		e := edgeBetween(HigherItemLevel, a, b, a.AverageItemLevel > b.AverageItemLevel, 0)
		e.High, e.Low = max(a.AverageItemLevel, b.AverageItemLevel), min(a.AverageItemLevel, b.AverageItemLevel)
		out = append(out, e)
	}
	return out
}

func edgeBetween(kind EdgeKind, a, b Card, aLeads bool, gap int64) Edge {
	if aLeads {
		return Edge{Kind: kind, Leader: a.Name, Other: b.Name, Gap: gap}
	}
	return Edge{Kind: kind, Leader: b.Name, Other: a.Name, Gap: gap}
}

func skewOf(a, b Card) Skew {
	if !a.HasDumpedAt || !b.HasDumpedAt {
		return Skew{}
	}
	skew := Skew{Known: true, GapSeconds: max(a.DumpedAt-b.DumpedAt, b.DumpedAt-a.DumpedAt), Older: a.Name}
	if b.DumpedAt < a.DumpedAt {
		skew.Older = b.Name
	}
	return skew
}
