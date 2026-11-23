// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package talents covers the talent trees from the dump. The trees in Forever differ from
// the classic ones (there are talents the classic game did not have), so the server has no
// built-in "popular builds": it relies solely on the tree saved
// by the addon (the talents.nodes section).
package talents

import (
	"slices"
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_talents.
type Query struct {
	Character string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	Tree      string `json:"tree,omitempty" jsonschema:"tree (tab) name, e.g. Beast Mastery; empty = all"`
}

const sourceClassic = "classic"

// Tree is a talent tab with the points spent in it.
type Tree struct {
	Name   string
	Points int64
}

// Spent is a learned talent. Tree and Name stay as the dump gave them.
type Spent struct {
	Tree, Name    any
	Rank, MaxRank int64
}

func (s Spent) Partial() bool { return s.MaxRank > 0 && s.Rank < s.MaxRank }

// Coord is a row or column: Raw is what the dump (or the grid normalisation) holds, Number its value when numeric.
type Coord struct {
	Raw     any
	Number  float64
	Numeric bool
}

// Node is a talent in the tree. Fields typed any are passed through to the answer unchanged;
// a nil one was not in the dump.
type Node struct {
	Tree                                            string
	Name                                            any
	Rank, MaxRank                                   int64
	HasRank, HasMaxRank                             bool
	Row, Column                                     Coord
	SpellID, PosX, PosY, Choices, DescriptionSource any
	CanPurchase, Available, PrereqMet               *bool
	Description                                     string
	HasDescription                                  bool
}

// Sheet is the talents section plus the character facts the rules need.
type Sheet struct {
	Class         any
	Level         int64
	Source, Spent any
	Unspent       int64
	UnspentKnown  bool
	HasTrees      bool
	Trees         []Tree
	Points        []Spent
	Nodes         []Node
}

// NodeStatus is a talent of a tree and whether a point can go to it now.
type NodeStatus struct {
	Node
	CanBuyNow bool
}

// TreeNodes is one tab with its talents in row and column order.
type TreeNodes struct {
	Name   string
	Points int64
	Nodes  []NodeStatus
}

type Purchase struct {
	Tree string
	Node Node
}

// Advice says why the list of purchasable talents is what it is.
type Advice int

const (
	AdviceNone Advice = iota
	AdviceNoFreePoints
	AdviceFreePointsUnexplained
)

// Report is the character's talents judged by the rules. Present is false when the dump has no talents section.
type Report struct {
	Present             bool
	Sheet               Sheet
	Split               []int64
	MainTree            string
	ExpectedClassic     int64
	NotMaxed            []Spent
	NodeCount           int
	MissingDescriptions int
	Trees               []TreeNodes
	CanBuy              []Purchase
	Advice              Advice
}

// Split returns the points per tree and the name of the tree with the most points ("" when nothing is spent).
func (sh Sheet) split() ([]int64, string) {
	var parts []int64
	var best int64 = -1
	main := ""
	for _, tree := range sh.Trees {
		parts = append(parts, tree.Points)
		if tree.Points > best {
			best = tree.Points
			main = tree.Name
		}
	}
	if best <= 0 {
		main = ""
	}
	return parts, main
}

// classicRowUnlocked: in classic trees row N requires 5*(N-1) points in that tree.
func classicRowUnlocked(row, pointsInTree int64) bool {
	return row <= 1 || pointsInTree >= 5*(row-1)
}

// Analyse judges the character's talents: the point split, learned, not maxed, full
// trees and what a point can be spent on right now.
func Analyse(s snapshot.Snapshot, q Query) Report {
	sheet, ok := readSheet(s)
	if !ok {
		return Report{}
	}
	r := Report{Present: true, Sheet: sheet}
	r.Split, r.MainTree = sheet.split()
	if sheet.Level >= 10 {
		// Classically the first point is at level 10, then one per level.
		r.ExpectedClassic = sheet.Level - 9
	}
	for _, sp := range sheet.Points {
		if sp.Partial() {
			r.NotMaxed = append(r.NotMaxed, sp)
		}
	}
	nodes := normalizePositions(sheet.Nodes)
	r.NodeCount = len(nodes)
	if len(nodes) == 0 {
		return r
	}
	noPoints := sheet.UnspentKnown && sheet.Unspent == 0 // the canPurchase flag from the game does not account for free points
	byTree, canBuy := sheet.classify(nodes, q.Tree, noPoints)
	r.Trees = sheet.fullTrees(byTree)
	r.CanBuy = canBuy
	switch {
	case noPoints:
		r.Advice = AdviceNoFreePoints
	case sheet.Unspent > 0 && len(canBuy) == 0:
		r.Advice = AdviceFreePointsUnexplained
	}
	for _, n := range nodes {
		if !n.HasDescription || n.Description == "" {
			r.MissingDescriptions++
		}
	}
	return r
}

func (sh Sheet) pointsInTree() map[string]int64 {
	points := map[string]int64{}
	for _, t := range sh.Trees {
		points[t.Name] = t.Points
	}
	return points
}

// buyable says whether a point can be spent on the talent now: from the game (canPurchase) or,
// for the classic API, from the row and prerequisite rule.
func (sh Sheet) buyable(n Node, pointsInTree int64) bool {
	if n.CanPurchase != nil {
		return *n.CanPurchase
	}
	if sh.Source != sourceClassic {
		return false
	}
	row, _ := snapshot.AsInt(n.Row.Raw)
	return n.Rank < n.MaxRank && classicRowUnlocked(row, pointsInTree) && (n.PrereqMet == nil || *n.PrereqMet)
}

// classify groups the nodes by tab (with the tree filter) and collects those that can be bought now.
func (sh Sheet) classify(nodes []Node, treeFilter string, noPoints bool) (map[string][]NodeStatus, []Purchase) {
	byTree := map[string][]NodeStatus{}
	var canBuy []Purchase
	inTree := sh.pointsInTree()
	tq := strings.ToLower(treeFilter)
	for _, n := range nodes {
		if tq != "" && !strings.Contains(strings.ToLower(n.Tree), tq) {
			continue
		}
		status := NodeStatus{Node: n}
		if !noPoints && n.Rank < n.MaxRank && sh.buyable(n, inTree[n.Tree]) {
			status.CanBuyNow = true
			canBuy = append(canBuy, Purchase{Tree: n.Tree, Node: n})
		}
		byTree[n.Tree] = append(byTree[n.Tree], status)
	}
	return byTree, canBuy
}

// fullTrees arranges the trees in the order of the section's trees (then the rest),
// and the talents within a tree by row and column.
func (sh Sheet) fullTrees(byTree map[string][]NodeStatus) []TreeNodes {
	var order []string
	for _, t := range sh.Trees {
		order = append(order, t.Name)
	}
	for name := range byTree {
		if !slices.Contains(order, name) {
			order = append(order, name)
		}
	}
	inTree := sh.pointsInTree()
	var trees []TreeNodes
	for _, name := range order {
		list := byTree[name]
		if len(list) == 0 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool {
			if ri, rj := list[i].Row.Number, list[j].Row.Number; ri != rj {
				return ri < rj
			}
			return list[i].Column.Number < list[j].Column.Number
		})
		trees = append(trees, TreeNodes{Name: name, Points: inTree[name], Nodes: list})
	}
	return trees
}

// RankGain is a talent whose rank rose between two dumps.
type RankGain struct {
	Name     string
	From, To int64
}

// NewRanks lists the talents that have a higher rank in after than in before.
func NewRanks(before, after Sheet) []RankGain {
	ranks := map[string]int64{}
	for _, sp := range before.Points {
		if name, ok := sp.Name.(string); ok {
			ranks[name] = sp.Rank
		}
	}
	var gains []RankGain
	for _, sp := range after.Points {
		name, _ := sp.Name.(string)
		if prev := ranks[name]; sp.Rank > prev {
			gains = append(gains, RankGain{Name: name, From: prev, To: sp.Rank})
		}
	}
	return gains
}
