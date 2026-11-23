// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package goals

import (
	"fmt"
	"math"
	"strings"

	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// History is what goals need from the dump history (progress.Tracker satisfies it).
type History interface {
	Rates(name string, current snapshot.Snapshot) (progress.Rates, error)
	WholeHistory(name string, current snapshot.Snapshot) (progress.Stretch, error)
}

// Env is the character whose goals are evaluated: its dump and (optionally) history for the ETA.
type Env struct {
	Character string
	Snap      snapshot.Snapshot
	History   History // nil = no ETA
}

// Evaluation is the saved goals of one character judged against its dump. Goals of other characters are only counted.
type Evaluation struct {
	Goals    []Progress
	Done     int
	Others   map[string]int
	Rates    progress.Rates
	RatesErr error
}

// Progress is one goal judged: how far it is and, depending on the goal type, the facts behind it.
type Progress struct {
	Goal    Goal
	Done    bool
	Percent float64
	ETA     *ETA

	Level      *LevelState
	Gold       *GoldState
	Reputation *ReputationState
	Skill      *SkillState
	Stock      *StockState
	Ability    *AbilityState
}

type LevelState struct {
	Level, XP, XPMax int64
	XPNeeded         int64
	HasXPNeeded      bool
}

func (l LevelState) xpFraction() float64 { return float64(l.XP) / float64(l.XPMax) }

type GoldState struct{ Money int64 }

type ReputationState struct {
	Found             bool
	Faction           Faction
	Value             int64
	Target            int64
	ByStanding        bool
	Threshold         int64
	ThresholdFromGame bool
}

type SkillState struct {
	Found        bool
	Skill        Skill
	Owned        []string
	CapBelowGoal bool
}

type StockState struct {
	Holdings items.Holdings
}

// AbilityState: the ability is known, or what the last trainer offer says about learning it.
type AbilityState struct {
	Known          bool
	Offered        bool
	NPC            any
	Category       any
	Cost           int64
	LevelRequired  int64
	Level, Money   int64
	BlockedByLevel bool
	BlockedByGold  bool
}

func (a AbilityState) canBuyNow() bool { return !a.BlockedByLevel && !a.BlockedByGold }

type BasisKind int

const (
	BasisXPRate BasisKind = iota
	BasisBalance
	BasisTrend
)

// Basis is where a pace used for an ETA comes from.
type Basis struct {
	Kind    BasisKind
	PerHour float64
	Label   string // BasisTrend: what grows
	Unit    string // BasisTrend
	Source  string // BasisTrend: where the history time comes from
}

type Assumption int

const (
	AssumeXPTable Assumption = iota
	AssumeBalance
	AssumeHistory
	AssumeConditions
)

// ETA is the estimated play time to the goal at the pace from history.
type ETA struct {
	Hours      float64
	Bases      []Basis
	Assumption Assumption
}

// Evaluate judges the goals of the given character; goals of other characters are only counted.
func Evaluate(all []Goal, env Env) Evaluation {
	ev := newEvaluator(env)
	out := Evaluation{Others: map[string]int{}, Rates: ev.rates, RatesErr: ev.ratesErr}
	for _, g := range all {
		if g.Character != env.Character {
			out.Others[g.Character]++
			continue
		}
		p := ev.evaluate(g)
		if p.Done {
			out.Done++
		}
		out.Goals = append(out.Goals, p)
	}
	return out
}

// evaluator computes the rate from history once per call, because many goals use it.
type evaluator struct {
	env      Env
	rates    progress.Rates
	ratesErr error
}

func newEvaluator(env Env) *evaluator {
	ev := &evaluator{env: env}
	if env.History == nil {
		ev.ratesErr = fmt.Errorf("dump history is disabled")
		return ev
	}
	ev.rates, ev.ratesErr = env.History.Rates(env.Character, env.Snap)
	return ev
}

func (ev *evaluator) evaluate(g Goal) Progress {
	switch g.Type {
	case Level:
		return ev.level(g)
	case Gold:
		return ev.gold(g)
	case Reputation:
		return ev.reputation(g)
	case Profession:
		return ev.profession(g)
	case Spell:
		return ev.spell(g)
	case Item:
		return ev.item(g)
	}
	return Progress{Goal: g}
}

func (ev *evaluator) xpPace() bool { return ev.rates.Known && ev.rates.XPPerHour > 0 }

func (ev *evaluator) level(g Goal) Progress {
	c := readCharacterState(ev.env.Snap)
	st := &LevelState{Level: c.Level, XP: c.XP, XPMax: c.XPMax}
	p := Progress{Goal: g, Level: st, Done: c.Level >= g.Target}
	if c.XPMax > 0 && g.Target > 1 {
		p.Percent = (float64(c.Level-1) + st.xpFraction()) / float64(g.Target-1) * 100
	}
	if p.Done || c.XPMax <= 0 {
		return p
	}
	st.XPNeeded, st.HasXPNeeded = (c.XPMax-c.XP)+(g.Target-c.Level-1)*c.XPMax, true
	if ev.xpPace() {
		p.ETA = &ETA{
			Hours:      float64(st.XPNeeded) / ev.rates.XPPerHour,
			Bases:      []Basis{{Kind: BasisXPRate, PerHour: ev.rates.XPPerHour}},
			Assumption: AssumeXPTable,
		}
	}
	return p
}

func (ev *evaluator) gold(g Goal) Progress {
	_, money := ev.env.Snap.LevelAndMoney()
	p := Progress{Goal: g, Gold: &GoldState{Money: money}, Done: money >= g.Target, Percent: float64(money) / float64(g.Target) * 100}
	if !p.Done && ev.rates.Known && ev.rates.MoneyPerHour > 0 {
		p.ETA = &ETA{
			Hours:      float64(g.Target-money) / ev.rates.MoneyPerHour,
			Bases:      []Basis{{Kind: BasisBalance, PerHour: ev.rates.MoneyPerHour}},
			Assumption: AssumeBalance,
		}
	}
	return p
}

// Successive reputation standings and their lower thresholds from the classic game (an approximation). When the dump has a faction at a given
// standing, the threshold is taken from its floor field, i.e. from the game.
var standings = []struct {
	name  string
	floor int64
}{
	{"Hated", -42000}, {"Hostile", -6000}, {"Unfriendly", -3000}, {"Neutral", 0},
	{"Friendly", 3000}, {"Honored", 9000}, {"Revered", 21000}, {"Exalted", 42000},
}

func canonicalStanding(name string) (string, bool) {
	for _, s := range standings {
		if strings.EqualFold(s.name, name) {
			return s.name, true
		}
	}
	return "", false
}

func standingNames() []string {
	out := make([]string, len(standings))
	for i, s := range standings {
		out[i] = s.name
	}
	return out
}

// standingFloor returns the reputation threshold for a standing; fromGame = true when it comes from the dump, not the classic table.
func standingFloor(factions []Faction, name string) (floor int64, fromGame bool) {
	for _, f := range factions {
		if strings.EqualFold(f.StandingName, name) && f.HasFloor {
			return f.Floor, true
		}
	}
	for _, st := range standings {
		if st.name == name {
			return st.floor, false
		}
	}
	return 0, false
}

// matchName picks an entry by name: an exact (case-insensitive) match first, then by fragment; -1 when none.
func matchName(names []string, name string) int {
	partial := -1
	for i, n := range names {
		switch {
		case strings.EqualFold(n, name):
			return i
		case partial < 0 && strings.Contains(strings.ToLower(n), strings.ToLower(name)):
			partial = i
		}
	}
	return partial
}

func factionNames(factions []Faction) []string {
	names := make([]string, len(factions))
	for i, f := range factions {
		names[i] = f.Name
	}
	return names
}

func findFaction(factions []Faction, name string) (Faction, bool) {
	i := matchName(factionNames(factions), name)
	if i < 0 {
		return Faction{}, false
	}
	return factions[i], true
}

func (ev *evaluator) reputation(g Goal) Progress {
	factions := readFactions(ev.env.Snap)
	f, found := findFaction(factions, g.Faction)
	if !found {
		return Progress{Goal: g, Reputation: &ReputationState{}}
	}
	st := &ReputationState{Found: true, Faction: f, Value: f.Value, Target: g.Target, ByStanding: g.Standing != ""}
	if st.ByStanding {
		st.Threshold, st.ThresholdFromGame = standingFloor(factions, g.Standing)
		st.Target = st.Threshold
	}
	p := Progress{Goal: g, Reputation: st, Done: st.Value >= st.Target}
	switch {
	case st.Target > 0:
		p.Percent = float64(st.Value) / float64(st.Target) * 100
	case p.Done:
		p.Percent = 100
	}
	if !p.Done {
		p.ETA = ev.trend("reputation rate", "pts", func(s snapshot.Snapshot) (float64, bool) {
			cur, ok := findFaction(readFactions(s), f.Name)
			return cur.Number, ok && cur.HasNumber
		}, float64(st.Target-st.Value))
	}
	return p
}

func skillNames(skills []Skill) []string {
	names := make([]string, len(skills))
	for i, k := range skills {
		names[i] = k.Name
	}
	return names
}

func (ev *evaluator) profession(g Goal) Progress {
	skills := readSkills(ev.env.Snap)
	i := matchName(skillNames(skills), g.Name)
	if i < 0 {
		st := &SkillState{}
		for _, k := range skills {
			st.Owned = append(st.Owned, k.Display)
		}
		return Progress{Goal: g, Skill: st}
	}
	k := skills[i]
	p := Progress{Goal: g, Done: k.Rank >= g.Target, Percent: float64(k.Rank) / float64(g.Target) * 100}
	p.Skill = &SkillState{Found: true, Skill: k, CapBelowGoal: k.MaxRank > 0 && k.MaxRank < g.Target && !p.Done}
	if !p.Done {
		p.ETA = ev.trend("skill rate", "pts", func(s snapshot.Snapshot) (float64, bool) {
			all := readSkills(s)
			j := matchName(skillNames(all), k.Name)
			if j < 0 {
				return 0, false
			}
			return all[j].Number, all[j].HasNumber
		}, float64(g.Target-k.Rank))
	}
	return p
}

func (ev *evaluator) item(g Goal) Progress {
	h := items.Count(ev.env.Snap, g.Item)
	have := h.Total()
	p := Progress{Goal: g, Stock: &StockState{Holdings: h}, Done: have >= g.Target, Percent: float64(have) / float64(g.Target) * 100}
	if !p.Done {
		p.ETA = ev.trend("gain rate", "pcs", func(s snapshot.Snapshot) (float64, bool) {
			return float64(items.Count(s, g.Item).Total()), true
		}, float64(g.Target-have))
	}
	return p
}

// trend computes the rate of increase of a value from the whole history (first to last dump) and the time to cover the shortfall.
// nil when there is no history, the stretch is shorter than 5 minutes of play, or the value is not growing.
func (ev *evaluator) trend(what, unit string, value func(snapshot.Snapshot) (float64, bool), missing float64) *ETA {
	if ev.env.History == nil {
		return nil
	}
	st, err := ev.env.History.WholeHistory(ev.env.Character, ev.env.Snap)
	if err != nil || len(st.Snapshots) < 2 || st.ElapsedSeconds < consumption.MinElapsed {
		return nil
	}
	first, ok1 := value(st.Snapshots[0])
	last, ok2 := value(st.Snapshots[len(st.Snapshots)-1])
	if !ok1 || !ok2 || last <= first {
		return nil
	}
	perHour := (last - first) / st.Hours()
	return &ETA{
		Hours:      missing / perHour,
		Bases:      []Basis{{Kind: BasisTrend, PerHour: perHour, Label: what, Unit: unit, Source: st.Basis}},
		Assumption: AssumeHistory,
	}
}

func (ev *evaluator) spell(g Goal) Progress {
	s := ev.env.Snap
	trainer := readTrainer(s)
	offer, offered := trainer.offerFor(g.Name)
	if spellKnown(readSpells(s), g.Name) || (offered && offer.Used) {
		return Progress{Goal: g, Ability: &AbilityState{Known: true}, Done: true, Percent: 100}
	}
	p := Progress{Goal: g, Ability: &AbilityState{}}
	if !offered {
		return p
	}
	level, money := s.LevelAndMoney()
	a := p.Ability
	*a = AbilityState{
		Offered: true, NPC: trainer.NPC, Category: offer.Category, Cost: offer.CostCopper, LevelRequired: offer.LevelRequired,
		Level: level, Money: money, BlockedByLevel: offer.LevelRequired > level, BlockedByGold: offer.CostCopper > money,
	}
	if a.canBuyNow() {
		return p
	}
	var hours float64
	var bases []Basis
	if a.BlockedByLevel && ev.xpPace() {
		if lv := ev.level(Goal{Type: Level, Target: a.LevelRequired}); lv.ETA != nil {
			hours, bases = math.Max(hours, lv.ETA.Hours), append(bases, lv.ETA.Bases...)
		}
	}
	if a.BlockedByGold {
		if gold := ev.gold(Goal{Type: Gold, Target: a.Cost}); gold.ETA != nil {
			hours, bases = math.Max(hours, gold.ETA.Hours), append(bases, gold.ETA.Bases...)
		}
	}
	if hours > 0 {
		p.ETA = &ETA{Hours: hours, Bases: bases, Assumption: AssumeConditions}
	}
	return p
}

// spellKnown: the ability is in the spellbook ("Name" or "Name Rank").
func spellKnown(spells []SpellEntry, name string) bool {
	for _, sp := range spells {
		if strings.EqualFold(sp.Name, name) || strings.EqualFold(strings.TrimSpace(sp.Name+" "+sp.Rank), name) {
			return true
		}
	}
	return false
}

// offerFor looks for an ability in the last trainer offer (by name, case-insensitive). Several ranks with the same
// name: the first not yet known is the next step.
func (t Trainer) offerFor(name string) (TrainerService, bool) {
	var first TrainerService
	found := false
	for _, svc := range t.Services {
		if !strings.EqualFold(svc.Name, name) && !strings.EqualFold(strings.TrimSpace(svc.Name+" "+svc.Rank), name) {
			continue
		}
		if !svc.Used {
			return svc, true
		}
		if !found {
			first, found = svc, true
		}
	}
	return first, found
}
