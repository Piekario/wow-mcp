// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package questrewards rates the choosable quest rewards: for every quest in the log and for the quest
// an NPC is currently offering (questOffer) or accepting (questTurnIn), it compares each reward
// with the equipped gear and points to the best choice with a justification. It only reads the dump; it chooses nothing in game.
package questrewards

import (
	"strconv"
	"strings"

	"wowmcp/internal/game/items"
	"wowmcp/internal/snapshot"
)

// Query is the input of wow_quest_rewards.
type Query struct {
	Character string             `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Quest     string             `json:"quest,omitempty" jsonschema:"quest name (or fragment) or ID; empty = all quests with rewards"`
	Weights   map[string]float64 `json:"weights,omitempty" jsonschema:"optional stat weights, e.g. {\"agility\":1,\"attackPower\":0.5}; without them the server uses weights.json (if present), and without that item level and DPS"`
}

// Where the quest reward rating comes from.
const (
	fromTurnIn = "turnIn" // quest turn-in window at the NPC
	fromOffer  = "offer"  // quest offer window at the NPC
	fromLog    = "log"    // quest log
)

// Choice is a choosable reward. Name and ID are the dump's values (ID is nil when the dump has none); Raw is the
// dump item the rating reads.
type Choice struct {
	Name, ID  any
	ItemID    int64
	HasItemID bool
	Count     int64
	Raw       any
}

type FixedItem struct {
	Name  any
	Count int64
}

// Rewards are what a quest gives: fixed items, gold, XP and currencies, and the choosable items.
type Rewards struct {
	Fixed      []FixedItem
	Money, XP  int64
	Currencies []any
	Choices    []Choice
}

func (r Rewards) HasFixed() bool {
	return len(r.Fixed) > 0 || r.Money > 0 || r.XP > 0 || len(r.Currencies) > 0
}

// Quest is a quest with rewards to rate: from an NPC window or the quest log.
type Quest struct {
	Source   string
	ID       int64
	Title    string
	NPC      string
	Captured snapshot.Captured
	Done     bool
	Rewards  Rewards
}

type RatedChoice struct {
	Choice
	Eval items.Evaluation
}

// Verdict says what the rating of one quest's choices came to.
type Verdict int

const (
	NoChoices Verdict = iota // only fixed rewards
	Suspect                  // the same choices repeat on several log quests: false data is likely
	Stale                    // the NPC window was captured long ago
	Rated
)

// Pick is the recommended reward: the best upgrade, or when there is none, the highest vendor value.
type Pick struct {
	RatedChoice
	Upgrade bool
}

// Alternative is what to take now when the best reward needs a higher level.
type Alternative struct{ RatedChoice }

type Judgement struct {
	Quest        Quest
	AlsoInWindow bool
	Verdict      Verdict
	Choices      []RatedChoice
	Best         *Pick
	ForNow       *Alternative
}

// Method is how the rating was computed: with stat weights, or by item level and DPS.
type Method struct {
	Weights items.Weights
	Source  string
	Note    string
}

// Report is the quests with their ratings. Filter is the quest argument, kept for the empty answer.
type Report struct {
	Method         Method
	Judged         []Judgement
	WithoutChoices []Quest
	AnySuspect     bool
	HasAnyRewards  bool
	Filter         string
}

func analyse(s snapshot.Snapshot, q Query, sc items.Scoring, fresh snapshot.Freshness) (Report, error) {
	weights, source, note, err := items.ChooseWeights(q.Weights, sc, s)
	if err != nil {
		return Report{}, err
	}
	src := readQuests(s)
	inWindow := map[int64]bool{}
	for _, e := range src.window {
		inWindow[e.ID] = true
	}
	suspect := suspectQuests(src.logged)
	quests := append([]Quest(nil), src.window...)
	for _, e := range src.logged {
		if !inWindow[e.ID] || e.ID == 0 {
			quests = append(quests, e)
		}
	}

	rep := Report{Method: Method{weights, source, note}, AnySuspect: len(suspect) > 0, HasAnyRewards: src.anyRewards, Filter: q.Quest}
	for _, e := range quests {
		if !q.matches(e) {
			continue
		}
		if len(e.Rewards.Choices) == 0 && e.Source == fromLog {
			rep.WithoutChoices = append(rep.WithoutChoices, e)
			continue
		}
		rep.Judged = append(rep.Judged, judge(s, weights, e, suspect[e.ID] && e.Source == fromLog, inWindow, fresh))
	}
	return rep, nil
}

// matches applies the quest filter: ID or a case-insensitive title fragment.
func (q Query) matches(e Quest) bool {
	needle := strings.TrimSpace(q.Quest)
	if needle == "" {
		return true
	}
	if id, err := strconv.ParseInt(needle, 10, 64); err == nil {
		return e.ID == id
	}
	return strings.Contains(strings.ToLower(e.Title), strings.ToLower(needle))
}

// choiceSignature is the IDs of the choosable rewards in order; "" when a reward has no ID.
func choiceSignature(choices []Choice) string {
	ids := make([]string, 0, len(choices))
	for _, c := range choices {
		if !c.HasItemID {
			return ""
		}
		ids = append(ids, strconv.FormatInt(c.ItemID, 10))
	}
	return strings.Join(ids, ",")
}

// suspectQuests returns the IDs of log quests whose non-empty choosable reward list (the same items in the same
// order) repeats on at least two quests. This is the pattern a client would give if it ignored the quest ID and returned
// the selected entry's rewards for all of them.
func suspectQuests(logged []Quest) map[int64]bool {
	groups := map[string][]int64{}
	for _, e := range logged {
		if sig := choiceSignature(e.Rewards.Choices); sig != "" {
			groups[sig] = append(groups[sig], e.ID)
		}
	}
	out := map[int64]bool{}
	for _, ids := range groups {
		if len(ids) >= 2 {
			for _, id := range ids {
				out[id] = true
			}
		}
	}
	return out
}
