// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package group covers the party and raid: the composition at dump time (the group section) plus
// the gear of players the player inspected with the Inspect window (the inspect section, key = name).
package group

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"wowmcp/internal/game/character"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_group.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Player    string `json:"player,omitempty" jsonschema:"player name: full gear from their latest Inspect"`
}

// bigLevelGap: from this level difference on the group gets a warning.
const bigLevelGap = 6

// Role is the role a member chose in the group.
type Role string

const (
	Tank   Role = "tank"
	Healer Role = "healer"
	Damage Role = "dps"
)

type Member struct {
	Name                      string
	Role                      Role // empty when the member has none
	Class                     string
	HasClass                  bool
	Level, Subgroup           int64
	Offline, Dead, OutOfRange bool
	Raw                       any
}

// Party is the group section. Type and the loot settings stay as the dump has them.
type Party struct {
	Type                            any
	Members                         []Member
	LootMethod, LootThreshold       any
	HasLootMethod, HasLootThreshold bool
}

func (p Party) IsParty() bool { return p.Type == "party" }

// Inspected is a player's Inspect record. Raw is the whole record for the full-gear answer.
type Inspected struct {
	Name             string
	Level, Class     any
	AverageItemLevel any
	HasAverage       bool
	CapturedAt       int64
	HasCapturedAt    bool
	Guild            any
	HasGuild         bool
	Record           bool // the dump has a table for the player, not a stray value
	Raw              any
}

// Inspection is an Inspect record with how long ago it was taken (Age is valid when HasCapturedAt).
type Inspection struct {
	Inspected
	Age time.Duration
}

type ProblemKind int

const (
	MemberOffline ProblemKind = iota
	MemberDead
	MemberOutOfRange
	LevelGap
	NoTank
	NoHealer
)

// Problem is a warning about the group; Member names the member for the member kinds, Min and Max the levels for LevelGap.
type Problem struct {
	Kind     ProblemKind
	Member   string
	Min, Max int64
}

type LevelSpread struct {
	Min, Max int64
	Average  float64
}

type MemberStatus struct {
	Member
	Inspection *Inspection
}

type Composition struct {
	Party    Party
	Members  []MemberStatus // ordered by subgroup
	Classes  map[string]int
	Roles    map[Role]int
	Levels   *LevelSpread // nil for an empty group
	Problems []Problem
}

// Report is the group at dump time with the players the character inspected.
type Report struct {
	InGroup      bool
	Composition  Composition
	ItemLevel    float64
	HasItemLevel bool
	Inspected    []Inspection // ordered by name
}

// Analyse judges the group: roles, classes, level spread and what is wrong with it.
func Analyse(s snapshot.Snapshot, fresh snapshot.Freshness) Report {
	inspected := readInspected(s)
	r := Report{}
	if party, ok := readParty(s); ok {
		r.InGroup = true
		r.Composition = compose(party, inspected, fresh)
	}
	r.ItemLevel, r.HasItemLevel = character.AverageItemLevel(s)
	for _, i := range inspected {
		r.Inspected = append(r.Inspected, inspect(i, fresh))
	}
	return r
}

func inspect(i Inspected, fresh snapshot.Freshness) Inspection {
	in := Inspection{Inspected: i}
	if i.HasCapturedAt {
		in.Age = fresh.Age(i.CapturedAt)
	}
	return in
}

func compose(party Party, inspected []Inspected, fresh snapshot.Freshness) Composition {
	c := Composition{Party: party, Classes: map[string]int{}, Roles: map[Role]int{}}
	var minLevel, maxLevel, sumLevel int64
	for _, m := range party.Members {
		status := MemberStatus{Member: m}
		if i := findInspected(inspected, m.Name); i != nil && i.Record {
			in := inspect(*i, fresh)
			status.Inspection = &in
		}
		c.Members = append(c.Members, status)
		if m.Role != "" {
			c.Roles[m.Role]++
		}
		if m.HasClass {
			c.Classes[m.Class]++
		}
		sumLevel += m.Level
		if minLevel == 0 || m.Level < minLevel {
			minLevel = m.Level
		}
		maxLevel = max(maxLevel, m.Level)
		c.Problems = append(c.Problems, memberProblems(m)...)
	}
	sort.SliceStable(c.Members, func(i, j int) bool { return c.Members[i].Subgroup < c.Members[j].Subgroup })
	if n := int64(len(party.Members)); n > 0 {
		c.Levels = &LevelSpread{Min: minLevel, Max: maxLevel, Average: float64(sumLevel*10/n) / 10}
		if maxLevel-minLevel >= bigLevelGap {
			c.Problems = append(c.Problems, Problem{Kind: LevelGap, Min: minLevel, Max: maxLevel})
		}
	}
	if party.IsParty() && len(c.Roles) > 0 {
		if c.Roles[Tank] == 0 {
			c.Problems = append(c.Problems, Problem{Kind: NoTank})
		}
		if c.Roles[Healer] == 0 {
			c.Problems = append(c.Problems, Problem{Kind: NoHealer})
		}
	}
	return c
}

func findInspected(all []Inspected, name string) *Inspected {
	for i := range all {
		if all[i].Name == name {
			return &all[i]
		}
	}
	return nil
}

func memberProblems(m Member) []Problem {
	var problems []Problem
	if m.Offline {
		problems = append(problems, Problem{Kind: MemberOffline, Member: m.Name})
	}
	if m.Dead {
		problems = append(problems, Problem{Kind: MemberDead, Member: m.Name})
	}
	if m.OutOfRange {
		problems = append(problems, Problem{Kind: MemberOutOfRange, Member: m.Name})
	}
	return problems
}

// InspectionOf returns a player's Inspect record (a name without the realm matches too).
func InspectionOf(s snapshot.Snapshot, player string, fresh snapshot.Freshness) (Inspection, error) {
	all := readInspected(s)
	for _, i := range all {
		if strings.EqualFold(i.Name, player) || strings.EqualFold(strings.SplitN(i.Name, "-", 2)[0], player) {
			return inspect(i, fresh), nil
		}
	}
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = p.Name
	}
	slices.Sort(names)
	return Inspection{}, fmt.Errorf("no Inspect data for %q. Inspected players: %s. In game right-click a player, choose Inspect, then /mcp reload",
		player, snapshot.OrEmpty(strings.Join(names, ", ")))
}
