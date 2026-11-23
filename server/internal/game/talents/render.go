// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package talents

import (
	"fmt"
	"strings"

	"wowmcp/internal/snapshot"
)

// View describes the character's talents: the point split, learned, not maxed, full
// trees and what a point can be spent on right now (canBuyNow).
func View(s snapshot.Snapshot, q Query) map[string]any {
	return Analyse(s, q).Render()
}

// Learned is the learned talents: "Name 3/5 (tree)".
func Learned(talents map[string]any) []string {
	return readSheetMap(talents).learnedLines()
}

// Split returns a point split like "11/0/0" and the name of the tree with the most points.
func Split(talents map[string]any) (split, main string) {
	points, main := readSheetMap(talents).split()
	return joinPoints(points), main
}

// Diff is the new talent ranks between dumps, e.g. "Focused Fire 1 -> 2".
func Diff(before, after any) []any {
	out := []any{}
	for _, g := range NewRanks(readSheetMap(snapshot.AsMap(before)), readSheetMap(snapshot.AsMap(after))) {
		out = append(out, fmt.Sprintf("%s %d -> %d", g.Name, g.From, g.To))
	}
	return out
}

func joinPoints(points []int64) string {
	parts := make([]string, len(points))
	for i, p := range points {
		parts[i] = fmt.Sprint(p)
	}
	return strings.Join(parts, "/")
}

// rankLine is "Name 3/5".
func rankLine(name any, rank, maxRank int64) string {
	return fmt.Sprintf("%v %d/%d", name, rank, maxRank)
}

func (s Spent) line() string { return rankLine(s.Name, s.Rank, s.MaxRank) }

func (sh Sheet) learnedLines() []string {
	var learned []string
	for _, sp := range sh.Points {
		learned = append(learned, fmt.Sprintf("%s (%v)", sp.line(), sp.Tree))
	}
	return learned
}

func (r Report) Render() map[string]any {
	if !r.Present {
		return map[string]any{"note": "No talents in the dump. Run /mcp reload in game."}
	}
	sh := r.Sheet
	out := map[string]any{
		"class":  sh.Class,
		"level":  sh.Level,
		"source": sh.Source,
		"spent":  sh.Spent,
		"split":  joinPoints(r.Split),
	}
	if r.MainTree != "" {
		out["mainTree"] = r.MainTree
	}
	out["unspent"] = sh.Unspent
	if r.ExpectedClassic > 0 {
		out["expectedPointsClassic"] = r.ExpectedClassic
	}
	out["trees"] = sh.renderTrees()
	out["learned"] = append([]string{}, sh.learnedLines()...)
	if len(r.NotMaxed) > 0 {
		partial := []string{}
		for _, sp := range r.NotMaxed {
			partial = append(partial, sp.line())
		}
		out["notMaxed"] = partial
	}
	if r.NodeCount == 0 {
		out["note"] = "The dump has no full tree (addon older than 0.4.0). Update the addon and type /mcp reload, then all talents with descriptions and what can be bought now will appear."
		return out
	}
	r.renderNodes(out)
	return out
}

func (sh Sheet) renderTrees() any {
	if !sh.HasTrees {
		return nil
	}
	trees := []any{}
	for _, t := range sh.Trees {
		trees = append(trees, map[string]any{"name": t.Name, "points": t.Points})
	}
	return trees
}

func (r Report) renderNodes(out map[string]any) {
	trees := []any{}
	for _, t := range r.Trees {
		trees = append(trees, t.render())
	}
	out["fullTrees"] = trees
	canBuy := []any{}
	for _, p := range r.CanBuy {
		line := map[string]any{"tree": p.Tree, "talent": rankLine(p.Node.Name, p.Node.Rank, p.Node.MaxRank)}
		if p.Node.HasDescription {
			line["description"] = p.Node.Description
		}
		canBuy = append(canBuy, line)
	}
	out["canBuyNow"] = canBuy
	switch r.Advice {
	case AdviceNoFreePoints:
		addNote(out, "No free talent points (unspent = 0), so canBuyNow is empty. The canPurchase flag from the game only reflects the tree requirements, not free points.")
	case AdviceFreePointsUnexplained:
		addNote(out, "There are free points, but the dump does not say which talents are available. Check the tree in game.")
	}
	if r.MissingDescriptions > 0 {
		out["missingDescriptions"] = r.MissingDescriptions
		addNote(out, fmt.Sprintf("%d of %d talents have no description. Assumption: the game provides descriptions only after the talent window is opened, so open it in game, type /mcp reload and ask again. Do not judge a talent by its name alone without a description.", r.MissingDescriptions, r.NodeCount))
	}
	out["howToUse"] = "Forever trees differ from the classic ones, so base advice on the descriptions (description) from this dump, not on builds from the classic version of the game. row and column are the row and column numbers in the tree (from 1); posY and posX are the raw position from the dump (pixels) when the addon saved it that way."
}

func (t TreeNodes) render() map[string]any {
	label := t.Name
	if label == "" {
		label = "(no tab)"
	}
	talents := make([]any, len(t.Nodes))
	for i, n := range t.Nodes {
		talents[i] = n.render()
	}
	return map[string]any{"tree": label, "points": t.Points, "talents": talents}
}

func (n NodeStatus) render() map[string]any {
	e := map[string]any{}
	putIf(e, "name", n.Name)
	if n.HasRank {
		e["rank"] = n.Rank
	}
	if n.HasMaxRank {
		e["maxRank"] = n.MaxRank
	}
	putIf(e, "spellID", n.SpellID)
	putIf(e, "posX", n.PosX)
	putIf(e, "posY", n.PosY)
	putIf(e, "choices", n.Choices)
	putIf(e, "descriptionSource", n.DescriptionSource)
	putIf(e, "row", n.Row.Raw)
	putIf(e, "column", n.Column.Raw)
	putBool(e, "canPurchase", n.CanPurchase)
	putBool(e, "available", n.Available)
	putBool(e, "prereqMet", n.PrereqMet)
	if n.HasDescription {
		e["description"] = n.Description
	}
	if n.CanBuyNow {
		e["canBuyNow"] = true
	}
	return e
}

func putIf(out map[string]any, key string, v any) {
	if v != nil {
		out[key] = v
	}
}

func putBool(out map[string]any, key string, b *bool) {
	if b != nil {
		out[key] = *b
	}
}

func addNote(out map[string]any, text string) {
	if prev, ok := out["note"].(string); ok && prev != "" {
		text = prev + " " + text
	}
	out["note"] = text
}
