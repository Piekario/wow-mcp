// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package zoneprogress says how far the character is with the quests of one zone: how many of the zone's quests are
// done (completedQuests), how many are in the log, which are left and which quest chains stopped half way. The list of
// a zone's quests comes from the zoneQuests section, which the addon reads from the quest database the player has
// installed (QuestieDB); quests the database marks for another faction, race or class are left out and counted.
package zoneprogress

import (
	"sort"
	"strings"
)

// Query is the input of wow_zone_progress.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = most recently dumped character"`
	Zone      string `json:"zone,omitempty" jsonschema:"zone name or a fragment of it; empty = the zone the character is in"`
}

// Starter is the NPC, object or item that starts a quest; X and Y are map percent in the quest's zone.
type Starter struct {
	Kind, Name, Zone string
	X, Y             float64
	HasXY            bool
}

// Quest is one quest of a zone as the database describes it. Masks are 0 when the quest is open to everyone.
type Quest struct {
	ID, Level, RequiredLevel, MaxLevel int64
	Name                               string
	Races, Classes                     int64
	PreAny, PreAll, ExclusiveTo        []int64
	Next, BreadcrumbFor                int64
	Repeatable                         bool
	Start                              *Starter
}

// Zone is a zone's quest list; QuestCount may exceed len(Quests) when the addon cut the list (Truncated).
type Zone struct {
	AreaID     int64
	Name       string
	Current    bool
	QuestCount int
	Truncated  bool
	Quests     []Quest
}

// Player is the character's race and class bits and the race masks of both factions; 0 = unknown.
type Player struct {
	RaceMask, ClassMask, AllianceMask, HordeMask int64
}

// Data is everything the rules read from the dump.
type Data struct {
	Present, NoIndex, HasCompleted bool
	CurrentZone                    string
	Level                          int64
	Player                         Player
	Completed, InLog               map[int64]bool
	Zones                          []Zone
}

// Restriction says why a quest is not for the character.
type Restriction int

const (
	ForCharacter Restriction = iota
	OtherFaction
	OtherRace
	OtherClass
)

// State is where a quest that is left stands.
type State int

const (
	Available State = iota
	NeedsLevel
	NeedsPrerequisite
)

// Left is a quest not done and not in the log. Missing lists prerequisite quests not done yet.
type Left struct {
	Quest   Quest
	State   State
	Missing []int64
}

// ChainStep is the next quest of a chain whose earlier step (After) is done.
type ChainStep struct {
	Left  Left
	After Quest
}

// Progress is the answer for one zone.
type Progress struct {
	Zone                                Zone
	Total, Completed, InLog, Repeatable int
	Closed, TooHighLevel                int
	Filtered                            map[Restriction]int
	CanFilter                           bool
	Left                                []Left
	Chains                              []ChainStep
	names                               map[int64]string
}

// Name of a quest of the zone, or "" when the zone list does not have it.
func (p Progress) Name(id int64) string { return p.names[id] }

// FindZone picks the zone by name (exact, then fragment, ignoring case) or, without a name, the current one.
func (d Data) FindZone(name string) (Zone, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		for _, z := range d.Zones {
			if z.Current {
				return z, true
			}
		}
		return Zone{}, false
	}
	for _, z := range d.Zones {
		if strings.EqualFold(z.Name, name) {
			return z, true
		}
	}
	for _, z := range d.Zones {
		if strings.Contains(strings.ToLower(z.Name), strings.ToLower(name)) {
			return z, true
		}
	}
	return Zone{}, false
}

// Restriction of a quest for this player; ForCharacter when the masks are unknown.
func (p Player) Restriction(q Quest) Restriction {
	if q.Races != 0 && p.RaceMask != 0 && q.Races&p.RaceMask == 0 {
		own := p.AllianceMask
		if p.HordeMask&p.RaceMask != 0 {
			own = p.HordeMask
		}
		if own != 0 && q.Races&own == 0 {
			return OtherFaction
		}
		return OtherRace
	}
	if q.Classes != 0 && p.ClassMask != 0 && q.Classes&p.ClassMask == 0 {
		return OtherClass
	}
	return ForCharacter
}

// Analyse counts the zone's quests and lists what is left and which chains can go on.
func Analyse(d Data, z Zone) Progress {
	p := Progress{
		Zone: z, Filtered: map[Restriction]int{}, names: map[int64]string{},
		CanFilter: d.Player.RaceMask != 0 && d.Player.ClassMask != 0,
	}
	for _, q := range z.Quests {
		p.names[q.ID] = q.Name
	}
	for _, q := range z.Quests {
		p.classify(d, q)
	}
	sort.SliceStable(p.Left, func(i, j int) bool {
		a, b := p.Left[i], p.Left[j]
		if a.State != b.State {
			return a.State < b.State
		}
		if a.Quest.Level != b.Quest.Level {
			return a.Quest.Level < b.Quest.Level
		}
		return a.Quest.ID < b.Quest.ID
	})
	p.Chains = chains(d, z, p.Left)
	return p
}

func (p *Progress) classify(d Data, q Quest) {
	if r := d.Player.Restriction(q); r != ForCharacter {
		p.Filtered[r]++
		return
	}
	if q.Repeatable {
		p.Repeatable++
		return
	}
	p.Total++
	switch {
	case d.Completed[q.ID]:
		p.Completed++
	case d.InLog[q.ID]:
		p.InLog++
	case closedBy(d, q):
		p.Closed++
	case q.MaxLevel > 0 && d.Level > q.MaxLevel:
		p.TooHighLevel++
	default:
		p.Left = append(p.Left, leftQuest(d, q))
	}
}

// closedBy: a mutually exclusive quest, or the quest a breadcrumb leads to, is already done or taken.
func closedBy(d Data, q Quest) bool {
	taken := func(id int64) bool { return d.Completed[id] || d.InLog[id] }
	for _, id := range q.ExclusiveTo {
		if taken(id) {
			return true
		}
	}
	return q.BreadcrumbFor != 0 && taken(q.BreadcrumbFor)
}

func leftQuest(d Data, q Quest) Left {
	l := Left{Quest: q, Missing: missingPrerequisites(d, q)}
	switch {
	case len(l.Missing) > 0:
		l.State = NeedsPrerequisite
	case q.RequiredLevel > d.Level:
		l.State = NeedsLevel
	}
	return l
}

// missingPrerequisites: every PreAll quest not done, or all PreAny quests when none of them is done.
func missingPrerequisites(d Data, q Quest) []int64 {
	var missing []int64
	for _, id := range q.PreAll {
		if !d.Completed[id] {
			missing = append(missing, id)
		}
	}
	for _, id := range q.PreAny {
		if d.Completed[id] {
			return missing
		}
	}
	return append(missing, q.PreAny...)
}

// chains: quests left whose earlier step in the chain is done, by the prerequisite fields or by nextQuestInChain.
func chains(d Data, z Zone, left []Left) []ChainStep {
	byID := map[int64]Quest{}
	previous := map[int64]Quest{}
	for _, q := range z.Quests {
		byID[q.ID] = q
		if q.Next != 0 && d.Completed[q.ID] {
			previous[q.Next] = q
		}
	}
	var out []ChainStep
	for _, l := range left {
		if l.State == NeedsPrerequisite {
			continue
		}
		after, ok := previous[l.Quest.ID]
		if !ok {
			after, ok = donePrerequisite(d, l.Quest, byID)
		}
		if ok {
			out = append(out, ChainStep{Left: l, After: after})
		}
	}
	return out
}

func donePrerequisite(d Data, q Quest, byID map[int64]Quest) (Quest, bool) {
	for _, id := range append(append([]int64{}, q.PreAll...), q.PreAny...) {
		if d.Completed[id] {
			if before, ok := byID[id]; ok {
				return before, true
			}
			return Quest{ID: id}, true
		}
	}
	return Quest{}, false
}
