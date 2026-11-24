// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package dungeonloot says which items from a dungeon would be upgrades for the character. The drop lists come only
// from the dungeonLoot section, which the addon reads from the game's Encounter Journal when the player types
// /mcp loot; the server has no drop tables of its own. Each item is compared with the equipped gear by the same rule
// as wow_compare_item and quest rewards (items.Buyer.Evaluate), after a check that the class can use it.
package dungeonloot

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_dungeon_loot.
type Query struct {
	Character    string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Dungeon      string `json:"dungeon,omitempty" jsonschema:"fragment of a dungeon name; empty = every dungeon read with /mcp loot"`
	Slot         string `json:"slot,omitempty" jsonschema:"only items for this equipment slot, e.g. head, chest, finger, trinket, mainHand, offHand, ranged; empty = all"`
	UpgradesOnly *bool  `json:"upgradesOnly,omitempty" jsonschema:"true (default) = only items that would be upgrades; false = every item with its verdict"`
}

func (q Query) upgradesOnly() bool { return q.UpgradesOnly == nil || *q.UpgradesOnly }

// Loot is one item of a dungeon as the addon saved it.
type Loot struct {
	Item      items.Item
	Name      string
	ID        int64
	Encounter string // the boss that drops it; "" when the journal did not say
	Pending   bool   // the client had not loaded the item's data yet
}

// Dungeon is one dungeon read from the Encounter Journal.
type Dungeon struct {
	Name       string
	JournalID  int64
	Min, Max   int64 // level range from the group finder; 0 when unknown
	Raid       bool
	Captured   snapshot.Captured
	Encounters []string // bosses in journal order
	Items      []Loot
	Truncated  bool // the addon kept only the first items
}

// Section is the dungeonLoot section. Unavailable is the addon's reason when the client has no journal API.
type Section struct {
	Present     bool
	Unavailable string
	Dungeons    []Dungeon
}

type Character struct {
	ClassToken, Class string
	Level             int64
}

// Verdict is what an item means for the character.
type Verdict int

const (
	Upgrade       Verdict = iota
	NoGain                // usable, but not better than what is equipped
	NotForClass           // the class cannot use this armor or weapon type
	LaterLevel            // the class learns this armor type at a higher level
	Loading               // the client had not loaded the item when the journal was read
	NotEquippable         // not gear
)

// Rated is a dungeon item judged against the character.
type Rated struct {
	Loot        Loot
	Verdict     Verdict
	Eval        items.Evaluation
	Proficiency itemkind.Proficiency
	FromLevel   int64
}

// Boss groups the items one boss drops.
type Boss struct {
	Name  string
	Items []Rated
}

// DungeonReport is one dungeon: the items shown, grouped by boss, and how many were left out for each reason.
type DungeonReport struct {
	Dungeon Dungeon
	Bosses  []Boss
	Skipped map[Verdict]int
	Loading []string
}

type State int

const (
	Absent      State = iota // no /mcp loot yet
	Unavailable              // the client has no journal API
	Ready
)

// Report is the answer of wow_dungeon_loot before rendering.
type Report struct {
	State        State
	Reason       string
	Character    Character
	UpgradesOnly bool
	Query        Query
	Saved        []string // names of all saved dungeons
	Dungeons     []DungeonReport
	Weights      items.Weights
	WeightsFrom  string
	WeightsNote  string
}

// Analyse rates the saved dungeon loot for the character.
func Analyse(s snapshot.Snapshot, q Query, sc items.Scoring) (Report, error) {
	slot, err := slotFilter(q.Slot)
	if err != nil {
		return Report{}, err
	}
	sec := readSection(s)
	r := Report{Character: readCharacter(s), UpgradesOnly: q.upgradesOnly(), Query: q}
	switch {
	case len(sec.Dungeons) > 0:
		r.State = Ready
	case sec.Present && sec.Unavailable != "":
		r.State, r.Reason = Unavailable, sec.Unavailable
		return r, nil
	default:
		return r, nil
	}
	r.Weights, r.WeightsFrom, r.WeightsNote, err = items.ChooseWeights(nil, sc, s)
	if err != nil {
		return Report{}, err
	}
	buyer := items.NewBuyer(s)
	want := strings.ToLower(strings.TrimSpace(q.Dungeon))
	for _, d := range sec.Dungeons {
		r.Saved = append(r.Saved, d.Name)
		if want != "" && !strings.Contains(strings.ToLower(d.Name), want) {
			continue
		}
		r.Dungeons = append(r.Dungeons, r.judgeDungeon(d, buyer, slot))
	}
	return r, nil
}

func (r Report) judgeDungeon(d Dungeon, buyer items.Buyer, slot string) DungeonReport {
	out := DungeonReport{Dungeon: d, Skipped: map[Verdict]int{}}
	byBoss := map[string][]Rated{}
	for _, l := range d.Items {
		if slot != "" && !l.Pending && !fitsSlot(l.Item, slot) {
			continue
		}
		rated := rate(l, buyer, r.Weights, r.Character)
		if rated.Verdict == Loading {
			out.Loading = append(out.Loading, l.Name)
		}
		if r.UpgradesOnly && rated.Verdict != Upgrade {
			out.Skipped[rated.Verdict]++
			continue
		}
		byBoss[l.Encounter] = append(byBoss[l.Encounter], rated)
	}
	for _, name := range bossOrder(d, byBoss) {
		list := byBoss[name]
		sort.SliceStable(list, func(i, j int) bool { return list[i].Eval.Score > list[j].Eval.Score })
		out.Bosses = append(out.Bosses, Boss{Name: name, Items: list})
	}
	return out
}

// bossOrder is the journal's boss order, then bosses the journal did not list, then items without a boss.
func bossOrder(d Dungeon, byBoss map[string][]Rated) []string {
	var order []string
	seen := map[string]bool{}
	for _, name := range d.Encounters {
		if len(byBoss[name]) > 0 && !seen[name] {
			order = append(order, name)
			seen[name] = true
		}
	}
	var rest []string
	for name := range byBoss {
		if !seen[name] && name != "" {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)
	if len(byBoss[""]) > 0 {
		order = append(order, "")
	}
	return order
}

func rate(l Loot, buyer items.Buyer, w items.Weights, c Character) Rated {
	r := Rated{Loot: l, Eval: buyer.Evaluate(l.Item, w)}
	r.Proficiency, r.FromLevel = l.Item.ProficiencyFor(c.ClassToken, c.Level)
	switch {
	case l.Pending:
		r.Verdict = Loading
	case !r.Eval.Equippable:
		r.Verdict = NotEquippable
	case r.Proficiency == itemkind.NotProficient:
		r.Verdict = NotForClass
	case r.Proficiency == itemkind.ProficientLater:
		r.Verdict = LaterLevel
	case r.Eval.Upgrade():
		r.Verdict = Upgrade
	default:
		r.Verdict = NoGain
	}
	return r
}

// slotFilter turns the slot argument into a prefix of the addon's slot names (finger matches finger1 and finger2).
func slotFilter(arg string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(arg))
	if alias, ok := slotAliases[want]; ok {
		want = alias
	}
	if want == "" {
		return "", nil
	}
	for _, s := range items.GearSlots {
		if strings.HasPrefix(strings.ToLower(s), want) {
			return want, nil
		}
	}
	return "", fmt.Errorf("unknown slot %q; use one of %s (finger and trinket match both slots)", arg, strings.Join(items.GearSlots, ", "))
}

var slotAliases = map[string]string{
	"ring": "finger", "cloak": "back", "shoulders": "shoulder", "gloves": "hands", "belt": "waist", "boots": "feet",
	"bracers": "wrist", "weapon": "mainhand", "main hand": "mainhand", "off hand": "offhand", "shield": "offhand",
}

func fitsSlot(it items.Item, want string) bool {
	for _, s := range it.Slots() {
		if strings.HasPrefix(strings.ToLower(s), want) {
			return true
		}
	}
	return false
}
