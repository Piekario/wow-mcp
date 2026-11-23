// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package dungeons covers dungeons and raids. First what the client itself reports (the dungeons section: the LFG tool with
// level ranges), and when the client reports nothing, a built-in table of dungeons from the
// classic version of the game. The table's ranges are approximate (different sources give
// them to within a few levels), and Forever may change them, so the
// response always says where they come from. Dungeons from the client carry numeric IDs (the activity or dungeon ID,
// the instance's map ID) and need no English table for their level ranges; the table stays English-only, and only
// adds zone and notes to dungeons whose English name it knows.
package dungeons

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"wowmcp/internal/game/quests"
	"wowmcp/internal/snapshot"
)

type Dungeon struct {
	Name     string
	Min, Max int64
	Zone     string
	Note     string
	Raid     bool
	Players  int64
	Aliases  []string
	ID       int64 // activity ID (C_LFGList) or dungeon ID (GetLFGDungeonInfo) from the client, 0 for the table
	MapID    int64 // the instance's map ID (lfgMapID), 0 when the client did not say
}

var classicDungeons = []Dungeon{
	{Name: "Ragefire Chasm", Min: 13, Max: 18, Zone: "Orgrimmar", Note: "entrance in the Horde capital"},
	{Name: "Wailing Caverns", Min: 17, Max: 24, Zone: "The Barrens"},
	{Name: "The Deadmines", Min: 17, Max: 26, Zone: "Westfall", Aliases: []string{"Deadmines"}},
	{Name: "Shadowfang Keep", Min: 22, Max: 30, Zone: "Silverpine Forest"},
	{Name: "Blackfathom Deeps", Min: 24, Max: 32, Zone: "Ashenvale"},
	{Name: "The Stockade", Min: 24, Max: 32, Zone: "Stormwind City", Note: "entrance in the Alliance capital", Aliases: []string{"Stormwind Stockade", "Stockade"}},
	{Name: "Gnomeregan", Min: 29, Max: 38, Zone: "Dun Morogh"},
	{Name: "Razorfen Kraul", Min: 29, Max: 38, Zone: "The Barrens"},
	{Name: "Scarlet Monastery: Graveyard", Min: 28, Max: 38, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Scarlet Monastery: Library", Min: 29, Max: 39, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Scarlet Monastery: Armory", Min: 32, Max: 42, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Scarlet Monastery: Cathedral", Min: 35, Max: 45, Zone: "Tirisfal Glades", Aliases: []string{"Scarlet Monastery"}},
	{Name: "Razorfen Downs", Min: 37, Max: 46, Zone: "The Barrens"},
	{Name: "Uldaman", Min: 41, Max: 51, Zone: "Badlands"},
	{Name: "Zul'Farrak", Min: 44, Max: 54, Zone: "Tanaris"},
	{Name: "Maraudon", Min: 46, Max: 55, Zone: "Desolace"},
	{Name: "Sunken Temple", Min: 50, Max: 56, Zone: "Swamp of Sorrows", Aliases: []string{"The Temple of Atal'Hakkar", "Temple of Atal'Hakkar"}},
	{Name: "Blackrock Depths", Min: 52, Max: 60, Zone: "Burning Steppes"},
	{Name: "Lower Blackrock Spire", Min: 55, Max: 60, Zone: "Burning Steppes", Aliases: []string{"Blackrock Spire"}},
	{Name: "Dire Maul", Min: 55, Max: 60, Zone: "Feralas"},
	{Name: "Scholomance", Min: 58, Max: 60, Zone: "Western Plaguelands"},
	{Name: "Stratholme", Min: 58, Max: 60, Zone: "Eastern Plaguelands"},
	{Name: "Upper Blackrock Spire", Min: 58, Max: 60, Zone: "Burning Steppes", Players: 10, Raid: true, Aliases: []string{"Blackrock Spire"}},
	{Name: "Zul'Gurub", Min: 60, Max: 60, Zone: "Stranglethorn Vale", Players: 20, Raid: true},
	{Name: "Ruins of Ahn'Qiraj", Min: 60, Max: 60, Zone: "Silithus", Players: 20, Raid: true},
	{Name: "Molten Core", Min: 60, Max: 60, Zone: "Burning Steppes", Players: 40, Raid: true},
	{Name: "Onyxia's Lair", Min: 60, Max: 60, Zone: "Dustwallow Marsh", Players: 40, Raid: true},
	{Name: "Blackwing Lair", Min: 60, Max: 60, Zone: "Burning Steppes", Players: 40, Raid: true},
	{Name: "Temple of Ahn'Qiraj", Min: 60, Max: 60, Zone: "Silithus", Players: 40, Raid: true},
	{Name: "Naxxramas", Min: 60, Max: 60, Zone: "Eastern Plaguelands", Players: 40, Raid: true},
}

// Query is the input of wow_dungeons.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	All       bool   `json:"all,omitempty" jsonschema:"true = all dungeons and raids, not just those matching the level"`
	Name      string `json:"name,omitempty" jsonschema:"fragment of a dungeon name: only that dungeon, with its quests"`
}

func findClassicDungeon(name string) (Dungeon, bool) {
	n := strings.ToLower(strings.TrimPrefix(name, "The "))
	for _, d := range classicDungeons {
		if strings.ToLower(strings.TrimPrefix(d.Name, "The ")) == n {
			return d, true
		}
	}
	return Dungeon{}, false
}

// Source says where the dungeon list comes from.
type Source int

const (
	FromGame Source = iota
	FromTable
)

// Lockout is a saved instance. The fields that are only passed through stay as the dump has them.
type Lockout struct {
	Name                                                   any
	Difficulty, DifficultyID, InstanceID, Raid, MaxPlayers any
	InstanceMapID                                          int64
	Released                                               bool // the dump says locked = false
	Killed, Encounters                                     int64
	HasKilled                                              bool
	ResetAt                                                int64
	HasReset                                               bool
}

// LockoutStatus is a lockout judged against the clock.
type LockoutStatus struct {
	Lockout
	Passed bool
	Left   time.Duration
}

// RelatedQuest is a quest of the log tied to a dungeon.
type RelatedQuest struct {
	Quest         quests.Quest
	Difficulty    string
	HasDifficulty bool
}

// Entry is a dungeon with what the character has to do with it.
type Entry struct {
	Dungeon
	Locked     bool
	Quests     []RelatedQuest
	LevelsToGo int64
}

// Report splits the dungeons into those matching the level, those coming soon and the rest.
type Report struct {
	Level        int64
	Source       Source
	All          bool
	Fit, Soon    []Entry
	Other        []Entry
	Lockouts     []LockoutStatus
	WithoutTable int
}

// Analyse splits dungeons into those matching the level (Fit), those available in a few levels
// (Soon) and the rest, with quest log quests and instance lockouts.
func Analyse(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) Report {
	log := quests.ReadLog(s)
	// Unlike in the quest plan: a value from the game <= 0 does not switch to the table.
	green := quests.GreenRange(log.Level)
	if log.HasGameGreen {
		green = log.GameGreen
	}
	r := Report{Level: log.Level, All: q.All}
	list := withTableData(readClientDungeons(s))
	if len(list) == 0 {
		list, r.Source = classicDungeons, FromTable
	} else {
		r.WithoutTable = withoutTableData(list)
	}
	lockouts := readLockouts(s)
	locked := lockedInstanceIDs(lockouts)
	nq := strings.ToLower(q.Name)
	for _, d := range list {
		if nq != "" && !strings.Contains(strings.ToLower(d.Name), nq) {
			continue
		}
		e := Entry{Dungeon: d, Locked: d.MapID > 0 && locked[d.MapID], Quests: dungeonQuests(log.Quests, d, log.Level, green)}
		r.place(e, nq != "", q.All)
	}
	r.Lockouts = judgeLockouts(lockouts, fresh)
	return r
}

func (r *Report) place(e Entry, named, all bool) {
	switch {
	case named:
		r.Fit = append(r.Fit, e)
	case e.Min > 0 && r.Level >= e.Min-1 && (e.Max == 0 || r.Level <= e.Max):
		r.Fit = append(r.Fit, e)
	case e.Min > 0 && r.Level < e.Min-1 && r.Level >= e.Min-6:
		e.LevelsToGo = e.Min - 1 - r.Level
		r.Soon = append(r.Soon, e)
	case all || len(e.Quests) > 0:
		r.Other = append(r.Other, e)
	}
}

// withTableData adds zone, notes and other names from the table to the client's dungeons it knows.
func withTableData(list []Dungeon) []Dungeon {
	for i := range list {
		if ref, ok := findClassicDungeon(list[i].Name); ok {
			list[i].Zone, list[i].Note, list[i].Aliases = ref.Zone, ref.Note, ref.Aliases
		}
	}
	return list
}

// withoutTableData counts the client's dungeons the English table did not enrich (no zone from it).
func withoutTableData(list []Dungeon) int {
	n := 0
	for _, d := range list {
		if d.Zone == "" {
			n++
		}
	}
	return n
}

// dungeonQuests returns the quest log quests tied to a dungeon: the log header
// is the dungeon name, or the dungeon name appears in the quest title or objectives.
func dungeonQuests(log []quests.Quest, d Dungeon, playerLevel, green int64) []RelatedQuest {
	names := []string{strings.ToLower(d.Name), strings.ToLower(strings.TrimPrefix(d.Name, "The "))}
	for _, a := range d.Aliases {
		names = append(names, strings.ToLower(a))
	}
	mentions := func(text string) bool {
		t := strings.ToLower(text)
		for _, n := range names {
			if n != "" && strings.Contains(t, n) {
				return true
			}
		}
		return false
	}
	var out []RelatedQuest
	for _, q := range log {
		if !mentions(q.Zone) && !mentions(q.Title) && !mentions(strings.Join(q.ObjectiveTexts(), " ")) {
			continue
		}
		related := RelatedQuest{Quest: q}
		if q.HasLevel && playerLevel > 0 {
			related.Difficulty, related.HasDifficulty = quests.Difficulty(q.Level, playerLevel, green), true
		}
		out = append(out, related)
	}
	return out
}

// lockedInstanceIDs is the set of instance map IDs the character is saved to (the lockouts' instanceID, addon 0.10.0).
func lockedInstanceIDs(lockouts []Lockout) map[int64]bool {
	ids := map[int64]bool{}
	for _, l := range lockouts {
		if l.InstanceMapID > 0 && !l.Released {
			ids[l.InstanceMapID] = true
		}
	}
	return ids
}

func judgeLockouts(lockouts []Lockout, fresh snapshot.Freshness) []LockoutStatus {
	var out []LockoutStatus
	for _, l := range lockouts {
		st := LockoutStatus{Lockout: l}
		if l.HasReset {
			st.Left, st.Passed = fresh.Until(l.ResetAt)
		}
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(out[i].Name) < fmt.Sprint(out[j].Name)
	})
	return out
}
