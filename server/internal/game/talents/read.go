// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package talents

import "wowmcp/internal/snapshot"

func readSheet(s snapshot.Snapshot) (Sheet, bool) {
	m := s.Map("talents")
	if m == nil {
		return Sheet{}, false
	}
	sheet := readSheetMap(m)
	sheet.Class = s.Character()["class"]
	sheet.Level, _ = s.LevelAndMoney()
	return sheet, true
}

func readSheetMap(m map[string]any) Sheet {
	sheet := Sheet{Source: m["source"], Spent: m["spent"], HasTrees: m["trees"] != nil}
	sheet.Unspent, sheet.UnspentKnown = snapshot.AsInt(m["unspent"])
	for _, raw := range snapshot.AsList(m["trees"]) {
		t := snapshot.AsMap(raw)
		tree := Tree{}
		tree.Name, _ = t["name"].(string)
		tree.Points, _ = snapshot.AsInt(t["points"])
		sheet.Trees = append(sheet.Trees, tree)
	}
	for _, raw := range snapshot.AsList(m["points"]) {
		t := snapshot.AsMap(raw)
		sp := Spent{Tree: t["tree"], Name: t["name"]}
		sp.Rank, _ = snapshot.AsInt(t["rank"])
		sp.MaxRank, _ = snapshot.AsInt(t["maxRank"])
		sheet.Points = append(sheet.Points, sp)
	}
	for _, raw := range snapshot.AsList(m["nodes"]) {
		if n := snapshot.AsMap(raw); n != nil {
			sheet.Nodes = append(sheet.Nodes, readNode(n))
		}
	}
	return sheet
}

func readNode(m map[string]any) Node {
	n := Node{
		Name: m["name"], SpellID: m["spellID"], PosX: m["posX"], PosY: m["posY"], Choices: m["choices"],
		DescriptionSource: m["descriptionSource"],
		Row:               readCoord(m["row"]), Column: readCoord(m["column"]),
		CanPurchase: boolOf(m["canPurchase"]), Available: boolOf(m["available"]), PrereqMet: boolOf(m["prereqMet"]),
	}
	n.Tree, _ = m["tree"].(string)
	n.Rank, n.HasRank = snapshot.AsInt(m["rank"])
	n.MaxRank, n.HasMaxRank = snapshot.AsInt(m["maxRank"])
	n.Description, n.HasDescription = m["description"].(string)
	return n
}

func readCoord(v any) Coord {
	number, ok := snapshot.AsFloat(v)
	return Coord{Raw: v, Number: number, Numeric: ok}
}

func boolOf(v any) *bool {
	if b, ok := v.(bool); ok {
		return &b
	}
	return nil
}
