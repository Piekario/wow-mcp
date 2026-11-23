// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package quests is the quest log: difficulty colors as in the game, objective progress
// and a quest plan grouped by zone.
package quests

import (
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Difficulty colors of the quest log.
const (
	Red    = "red"
	Orange = "orange"
	Yellow = "yellow"
	Green  = "green"
	Gray   = "gray"
)

// Objective is one line of a quest's objectives. HasCount: the client gave have and need.
type Objective struct {
	Text       string
	HasText    bool
	Have, Need int64
	HasCount   bool
	Done       bool
}

// Quest is one quest of the log. ID is passed through as the dump has it.
type Quest struct {
	ID             any
	Title, Zone    string
	Level          int64
	HasLevel       bool
	ZoneMapID      int64
	Complete       bool
	Failed         bool
	SuggestedGroup int64
	Objectives     []Objective
}

// Log is the quest log with what the quest rules need to know about the character.
type Log struct {
	Present      bool
	Quests       []Quest
	Level        int64
	GameGreen    int64
	HasGameGreen bool
	MaxQuests    int64
	HasMaxQuests bool
}

// EffectiveGreen is the green quest range: from the game when it gave a positive value, else the classic table.
func (l Log) EffectiveGreen() (green int64, fromGame bool) {
	if l.HasGameGreen && l.GameGreen > 0 {
		return l.GameGreen, true
	}
	return GreenRange(l.Level), false
}

// ObjectiveTexts returns the texts of the objectives.
func (q Quest) ObjectiveTexts() []string {
	var texts []string
	for _, o := range q.Objectives {
		if o.HasText {
			texts = append(texts, o.Text)
		}
	}
	return texts
}

// Progress is the objective progress in percent (a quest without numbers: 0 or 100) and the texts of the open objectives.
func (q Quest) Progress() (percent int64, left []string) {
	var have, need int64
	for _, o := range q.Objectives {
		if o.HasCount && o.Need > 0 {
			have += min(o.Have, o.Need)
			need += o.Need
		} else {
			need++
			if o.Done {
				have++
			}
		}
		if !o.Done && o.HasText {
			left = append(left, o.Text)
		}
	}
	if need == 0 {
		if q.Complete {
			return 100, nil
		}
		return 0, left
	}
	return have * 100 / need, left
}

// GreenRange is the classic table: how many levels lower a quest is still green.
// Used when the addon did not save the value from the game (character.questGreenRange).
func GreenRange(level int64) int64 {
	switch {
	case level <= 7:
		return 5
	case level <= 9:
		return 6
	case level <= 11:
		return 7
	case level <= 15:
		return 8
	case level <= 19:
		return 9
	case level <= 29:
		return 11
	case level <= 39:
		return 12
	case level <= 44:
		return 13
	case level <= 49:
		return 14
	case level <= 54:
		return 15
	case level <= 59:
		return 16
	}
	return 17
}

// Difficulty mirrors the quest log colors (GetQuestDifficultyColor).
func Difficulty(questLevel, playerLevel, green int64) string {
	diff := questLevel - playerLevel
	switch {
	case diff >= 5:
		return Red
	case diff >= 3:
		return Orange
	case diff >= -2:
		return Yellow
	case -diff <= green:
		return Green
	}
	return Gray
}

// Summary counts the log.
type Summary struct{ Count, Ready int }

func Summarize(s snapshot.Snapshot) Summary {
	log := ReadLog(s)
	sum := Summary{Count: len(log.Quests)}
	for _, q := range log.Quests {
		if q.Complete {
			sum.Ready++
		}
	}
	return sum
}

// PlanQuery is the input of wow_quest_plan.
type PlanQuery struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Zone      string `json:"zone,omitempty" jsonschema:"only quests from this zone (name fragment)"`
}

// Status is where a quest stands in the plan.
type Status int

const (
	InProgress Status = iota
	ReadyToTurnIn
	FailedQuest
)

// PlanEntry is one quest of the plan with its rating.
type PlanEntry struct {
	Quest      Quest
	Difficulty string
	Percent    int64
	Left       []string
	Status     Status
}

// ZoneGroup is the quests of one zone in the plan.
type ZoneGroup struct {
	Name            string
	Quests          []PlanEntry
	Ready, Playable int
}

// Flag is a quest the plan warns about.
type Flag struct {
	Quest Quest
	Zone  string
}

// PlanResult is the log grouped by zone with the quests that need attention.
type PlanResult struct {
	PlayerLevel                         int64
	Green                               int64
	GreenFromGame                       bool
	Zones                               []ZoneGroup
	Count                               int
	ByDifficulty                        map[string]int
	NoLog                               bool
	Ready, TooHard, Gray, Failed, Group []Flag
	ShowLogSize                         bool
	LogMax                              int64
	AlmostFull                          bool
}

// Analyse groups the log by zone (zones with the most things to do first) and rates each quest.
func Analyse(s snapshot.Snapshot, q PlanQuery) PlanResult {
	log := ReadLog(s)
	green, fromGame := log.EffectiveGreen()
	r := PlanResult{PlayerLevel: log.Level, Green: green, GreenFromGame: fromGame, ByDifficulty: map[string]int{}, NoLog: !log.Present}
	zones := map[string]*ZoneGroup{}
	for _, quest := range log.Quests {
		zone := quest.Zone
		if zone == "" {
			zone = "(no zone)"
		}
		if q.Zone != "" && !strings.Contains(strings.ToLower(zone), strings.ToLower(q.Zone)) {
			continue
		}
		r.Count++
		z := zones[zone]
		if z == nil {
			z = &ZoneGroup{Name: zone}
			zones[zone] = z
		}
		entry := r.rate(quest, zone, z)
		r.ByDifficulty[entry.Difficulty]++
		z.Quests = append(z.Quests, entry)
	}
	r.Zones = sortedZones(zones)
	if log.HasMaxQuests && log.MaxQuests > 0 && q.Zone == "" {
		r.ShowLogSize, r.LogMax = true, log.MaxQuests
		r.AlmostFull = int64(r.Count) >= log.MaxQuests-2
	}
	return r
}

// rate rates one quest and adds it to the zone counters and the warning lists.
func (r *PlanResult) rate(quest Quest, zone string, z *ZoneGroup) PlanEntry {
	diff := Difficulty(quest.Level, r.PlayerLevel, r.Green)
	percent, left := quest.Progress()
	entry := PlanEntry{Quest: quest, Difficulty: diff, Percent: percent, Left: left}
	flag := Flag{quest, zone}
	if quest.SuggestedGroup > 1 {
		r.Group = append(r.Group, flag)
	}
	switch {
	case quest.Failed:
		entry.Status = FailedQuest
		r.Failed = append(r.Failed, flag)
	case quest.Complete:
		entry.Status = ReadyToTurnIn
		r.Ready = append(r.Ready, flag)
		z.Ready++
	default:
		entry.Status = InProgress
		if diff == Red {
			r.TooHard = append(r.TooHard, flag)
		} else {
			z.Playable++
		}
	}
	if diff == Gray && !quest.Complete {
		r.Gray = append(r.Gray, flag)
	}
	return entry
}

// sortedZones puts the zones with the most things to do and turn in on top,
// and the quests within a zone from the lowest level.
func sortedZones(zones map[string]*ZoneGroup) []ZoneGroup {
	list := make([]*ZoneGroup, 0, len(zones))
	for _, z := range zones {
		list = append(list, z)
	}
	sort.Slice(list, func(i, j int) bool {
		si, sj := list[i].Ready+list[i].Playable, list[j].Ready+list[j].Playable
		if si != sj {
			return si > sj
		}
		return list[i].Name < list[j].Name
	})
	out := make([]ZoneGroup, 0, len(list))
	for _, z := range list {
		sort.SliceStable(z.Quests, func(i, j int) bool { return z.Quests[i].Quest.Level < z.Quests[j].Quest.Level })
		out = append(out, *z)
	}
	return out
}
