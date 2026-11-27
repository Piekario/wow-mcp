// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package legacy covers the Forever Legacy system from the dump's legacy section: Legacy Points
// earned account-wide from challenges and spent per character on perks in three trees (Professions,
// Adventure, Resourcefulness). The client documents no API for it, so the addon reads every
// non-class trait config and the server picks the Legacy one. The server has no built-in perk list:
// advice comes from the names and descriptions in the dump. Nothing here spends points; only the
// player learns perks and clicks Apply Changes.
package legacy

import (
	"fmt"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_legacy.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Tree      string `json:"tree,omitempty" jsonschema:"Legacy tree (tab) name, e.g. Adventure; empty = all"`
	Discovery bool   `json:"discovery,omitempty" jsonschema:"true = also return what the addon found about the client's Legacy API (names of namespaces, functions, frames and their readings); shown anyway when no perk tree was found. Since addon 0.19.1 the dump has it only then or after /mcp probe"`
}

type Status string

const (
	StatusMaxed     Status = "maxed"
	StatusAvailable Status = "available"
	StatusLocked    Status = "locked"
)

// Advice is what the points say about the perks a point can go to now.
type Advice int

const (
	AdviceNone Advice = iota
	AdviceNoPoints
	AdviceNothingAvailable
)

const maxOpenChallenges = 40

// unrevealedName is what the real client calls the "?" perks of each tree (spell 1317256, one rank): not yet revealed.
const unrevealedName = "Unknown"

// knownTabs are the English tab names of the Legacy trees (patch 1.60.1); used only to recognise the config.
var knownTabs = []string{"professions", "adventure", "resourcefulness"}

// treeNames names the Legacy trees by trait tree ID. The real client gives these trees no group names, so the
// addon writes "tree <ID>". IDs verified in the first real dump (Forever 1.60.1 build 71037, 2026-11-22) by their
// perks: 1187 holds Working Overtime and Bountiful Harvest, 1188 Well Rested and Thrill of Adventure, 1189 Gourmand
// and Reinforce. Tree 1118 in the same config has no perks.
var treeNames = map[int64]string{1187: "Professions", 1188: "Adventure", 1189: "Resourcefulness"}

// Perk keeps Name, Description, Requires, RequiresSpent, Row and Column as the dump gave them.
type Perk struct {
	Name, Description, Requires, RequiresSpent, Row, Column any
	Tree                                                    string
	Rank, MaxRank                                           int64
	CanPurchase                                             bool
}

type Tree struct {
	Name    string
	HasName bool
	ID      any
	Points  any
}

// Currency is one trait currency; the client repeats the same one for every tree.
type Currency struct {
	ID, TreeID, Available, Spent, Max, MaxAvailable, Name, TotalEarned, Quantity, MaxQuantity any
}

type Config struct {
	ID, TypeName, Name  any
	Trees               []Tree
	Currencies          []Currency
	Perks               []Perk
	MissingDescriptions int64
}

type Challenge struct {
	Name, Category, Description, Reward, Points any
	Completed                                   bool
	CompletedOn                                 string
	Missing                                     []string // criteria still to do (incomplete challenges)
	MoreMissing                                 int64
}

type Section struct {
	Configs            []Config
	Challenges         *[]Challenge
	MatchedBy          string // how the addon matched the challenges
	ChallengesError    string
	HasChallengesError bool
	Discovery          any
}

// treeName is the display name of a tree: the one in the dump, or the known Legacy name for an unnamed tree.
func treeName(name string, treeID any) string {
	if id, ok := snapshot.AsInt(treeID); ok && (name == "" || name == fmt.Sprintf("tree %d", id)) {
		if known, ok := treeNames[id]; ok {
			return known
		}
	}
	return name
}

// Label is the perk's tree mapped like treeName ("tree 1188" -> Adventure).
func (p Perk) Label() string {
	var id int64
	if _, err := fmt.Sscanf(p.Tree, "tree %d", &id); err == nil {
		return treeName(p.Tree, id)
	}
	return p.Tree
}

func (p Perk) Status() Status {
	switch {
	case p.MaxRank > 0 && p.Rank >= p.MaxRank:
		return StatusMaxed
	case p.CanPurchase:
		return StatusAvailable
	}
	return StatusLocked
}

type PerkView struct {
	Perk
	Status     Status
	Unrevealed bool
	Learned    bool
}

type Learnable struct {
	Tree string
	Perk Perk
}

type TreeView struct {
	Name        string
	PointsSpent any
	Perks       []PerkView
}

// ConfigSummary describes a trait config that was not picked as the Legacy one.
type ConfigSummary struct {
	ID, TypeName, Name any
	Tabs               []string
}

// Points is the Legacy point currency. Main is the first entry (the whole picture); Others are further
// currency IDs. Known says whether the dump states the unspent points at all.
type Points struct {
	Present bool
	Main    Currency
	Others  []Currency
	Unspent int64
	Known   bool
}

type ChallengeReport struct {
	Total          int
	Source         string // ChallengesFromAddon or ChallengesFromRewards
	MatchedBy      string
	CompletedNames []any
	Done           []Challenge
	Open           []Challenge
	NotListed      int
}

// Report is the character's Legacy state judged by the rules. Found is false when the dump has no legacy section.
type Report struct {
	Found               bool
	Config              *Config
	Others              []ConfigSummary
	Points              Points
	Trees               []TreeView
	CanLearn            []Learnable
	Advice              Advice
	MissingDescriptions int64
	Challenges          *ChallengeReport
	ChallengesError     string
	HasChallengesError  bool
	Discovery           bool
	DiscoveryData       any
	LocaleNote          string
}

func Analyse(s snapshot.Snapshot, q Query) Report {
	section, ok := read(s)
	if !ok {
		return Report{}
	}
	r := Report{Found: true, Discovery: q.Discovery, DiscoveryData: section.Discovery,
		ChallengesError: section.ChallengesError, HasChallengesError: section.HasChallengesError}
	var notes []string
	r.Config, r.Others = pickConfig(section.Configs)
	if r.Config == nil {
		r.Discovery = true
	} else {
		r.analyseConfig(*r.Config, q.Tree)
		notes = append(notes, s.EnglishOnlyNote("recognising the Legacy tree among several trait trees by its tab names"))
	}
	if section.Challenges != nil && len(*section.Challenges) > 0 {
		r.Challenges = summariseChallenges(*section.Challenges)
		r.Challenges.Source, r.Challenges.MatchedBy = ChallengesFromAddon, section.MatchedBy
		notes = append(notes, s.EnglishOnlyNote("finding Legacy challenges by the achievement category title or reward text"))
	} else if derived := deriveChallenges(s); derived != nil {
		r.Challenges = derived
		notes = append(notes, s.EnglishOnlyNote("finding Legacy challenges by the reward text \""+rewardFragment+"\""))
	} else if section.Challenges != nil {
		r.Challenges = summariseChallenges(nil)
		r.Challenges.Source, r.Challenges.MatchedBy = ChallengesFromAddon, section.MatchedBy
	}
	r.LocaleNote = joinNonEmpty(notes)
	return r
}

func joinNonEmpty(notes []string) string {
	var kept []string
	for _, n := range notes {
		if n != "" {
			kept = append(kept, n)
		}
	}
	return strings.Join(kept, " ")
}

// pickConfig chooses the Legacy config: named Legacy, else with the known tab names, else the only one.
// The rest are summarised as other trait configs.
func pickConfig(configs []Config) (*Config, []ConfigSummary) {
	best, bestScore := -1, 0
	for i, c := range configs {
		if score := c.legacyScore(); score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 && len(configs) == 1 {
		best = 0
	}
	var picked *Config
	var others []ConfigSummary
	for i := range configs {
		c := &configs[i]
		if i == best {
			picked = c
			continue
		}
		others = append(others, ConfigSummary{ID: c.ID, TypeName: c.TypeName, Name: c.Name, Tabs: c.tabNames()})
	}
	return picked, others
}

func (c Config) legacyScore() int {
	score := 0
	if name, _ := c.Name.(string); strings.Contains(strings.ToLower(name), "legacy") {
		score += 10
	}
	for _, t := range c.Trees {
		if id, ok := snapshot.AsInt(t.ID); ok && treeNames[id] != "" {
			score++
		}
	}
	for _, tab := range c.tabNames() {
		for _, known := range knownTabs {
			if strings.EqualFold(tab, known) {
				score++
			}
		}
	}
	return score
}

func (c Config) tabNames() []string {
	names := []string{}
	for _, t := range c.Trees {
		if t.HasName {
			names = append(names, treeName(t.Name, t.ID))
		}
	}
	return names
}

func (r *Report) analyseConfig(c Config, treeFilter string) {
	r.Points = pointsOf(c)
	noPoints := r.Points.Known && r.Points.Unspent == 0
	byTree, canLearn := c.perksByTree(treeFilter, noPoints)
	r.Trees = c.treeViews(byTree)
	r.CanLearn = canLearn
	switch {
	case noPoints:
		r.Advice = AdviceNoPoints
	case len(canLearn) == 0:
		r.Advice = AdviceNothingAvailable
	}
	r.MissingDescriptions = c.MissingDescriptions
}

// pointsOf reads the Legacy point currency. The real client reports one trait currency shared by all three trees,
// repeated per tree with the same numbers, so the first entry is the whole picture; a second currency ID is listed separately.
func pointsOf(c Config) Points {
	if len(c.Currencies) == 0 {
		return Points{}
	}
	main := c.Currencies[0]
	p := Points{Present: true, Main: main}
	seen := map[any]bool{main.ID: true}
	for _, cur := range c.Currencies[1:] {
		if !seen[cur.ID] {
			seen[cur.ID] = true
			p.Others = append(p.Others, cur)
		}
	}
	p.Unspent, p.Known = snapshot.AsInt(main.Available)
	return p
}

func (p Perk) view() PerkView {
	return PerkView{Perk: p, Status: p.Status(), Unrevealed: p.Name == unrevealedName, Learned: p.Rank > 0}
}

// perksByTree groups perks by tab (with the filter) and collects the ones a point can go to now.
func (c Config) perksByTree(treeFilter string, noPoints bool) (map[string][]PerkView, []Learnable) {
	byTree := map[string][]PerkView{}
	var canLearn []Learnable
	tq := strings.ToLower(treeFilter)
	for _, p := range c.Perks {
		tree := p.Label()
		if tq != "" && !strings.Contains(strings.ToLower(tree), tq) {
			continue
		}
		v := p.view()
		byTree[tree] = append(byTree[tree], v)
		if v.Status == StatusAvailable && !noPoints {
			canLearn = append(canLearn, Learnable{Tree: tree, Perk: p})
		}
	}
	return byTree, canLearn
}

func sortByPosition(list []PerkView) {
	sort.SliceStable(list, func(i, j int) bool {
		ri, _ := snapshot.AsFloat(list[i].Row)
		rj, _ := snapshot.AsFloat(list[j].Row)
		if ri != rj {
			return ri < rj
		}
		ci, _ := snapshot.AsFloat(list[i].Column)
		cj, _ := snapshot.AsFloat(list[j].Column)
		return ci < cj
	})
}

// treeViews lists the tabs in the dump's order, perks by row and column, then the tabs only the perks name.
func (c Config) treeViews(byTree map[string][]PerkView) []TreeView {
	var out []TreeView
	seen := map[string]bool{}
	add := func(name string, points any) {
		list := byTree[name]
		if seen[name] || len(list) == 0 {
			return
		}
		seen[name] = true
		sortByPosition(list)
		out = append(out, TreeView{Name: name, PointsSpent: points, Perks: list})
	}
	for _, t := range c.Trees {
		add(treeName(t.Name, t.ID), t.Points)
	}
	var rest []string
	for name := range byTree {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		add(name, nil)
	}
	return out
}

// summariseChallenges counts the Legacy challenges and keeps the completed names and the first open ones.
func summariseChallenges(list []Challenge) *ChallengeReport {
	r := &ChallengeReport{Total: len(list), CompletedNames: []any{}}
	for _, ch := range list {
		if ch.Completed {
			r.CompletedNames = append(r.CompletedNames, ch.Name)
			r.Done = append(r.Done, ch)
		} else if len(r.Open) < maxOpenChallenges {
			r.Open = append(r.Open, ch)
		}
	}
	if left := len(list) - len(r.CompletedNames) - len(r.Open); left > 0 {
		r.NotListed = left
	}
	return r
}
