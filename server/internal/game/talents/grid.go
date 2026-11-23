// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package talents

import (
	"slices"
	"sort"
)

// Node positions in dumps from addon 0.5.0 are raw pixels of the talent window
// (a grid about every 600, with occasional outliers), not row and column numbers.
// Here we turn them into numbers from 1 so wow_talents reads the same regardless
// of the addon version. The raw values stay in posY (row) and posX (column).

const (
	// gridMax: values up to this are already row or column numbers, not pixels.
	gridMax = 50
	// clusterGap: values closer than this (pixels) lie in the same row or column.
	// The grid step is about 600 and outliers are tens, so 200 separates one from the other.
	clusterGap = 200
	// stretchFactor: a value exactly 10x another is a dump artifact (e.g. 39300 instead of 3930).
	stretchFactor = 10
)

// normalizePositions returns a copy of the nodes in which row and column (if they are pixels)
// are turned into numbers from 1 within the tree, and the raw values are saved in posY and posX.
// Nodes without pixels (an addon with numbers or the classic API) stay unchanged.
func normalizePositions(nodes []Node) []Node {
	out := slices.Clone(nodes)
	byTree := map[string][]*Node{}
	for i := range out {
		byTree[out[i].Tree] = append(byTree[out[i].Tree], &out[i])
	}
	for _, group := range byTree {
		reindex(group, func(n *Node) *Coord { return &n.Row }, func(n *Node) *any { return &n.PosY })
		reindex(group, func(n *Node) *Coord { return &n.Column }, func(n *Node) *any { return &n.PosX })
	}
	return out
}

// reindex turns the axis of one tree's nodes into numbers from 1; the raw value goes to the raw field.
func reindex(group []*Node, axis func(*Node) *Coord, raw func(*Node) *any) {
	var values []float64
	for _, n := range group {
		if c := axis(n); c.Numeric {
			values = append(values, c.Number)
		}
	}
	index := gridIndexes(values)
	if index == nil {
		return
	}
	for _, n := range group {
		if c := axis(n); c.Numeric {
			*raw(n) = c.Raw
			*c = Coord{Raw: index[c.Number], Number: float64(index[c.Number]), Numeric: true}
		}
	}
}

// gridIndexes maps a raw value to a number from 1; nil when the values do not look like pixels.
// A value exactly 10x another value gets that one's number, and the rest are grouped
// with the clusterGap tolerance and numbered ascending.
func gridIndexes(values []float64) map[float64]int64 {
	if len(values) == 0 || slices.Max(values) <= gridMax {
		return nil
	}
	distinct := slices.Clone(values)
	sort.Float64s(distinct)
	distinct = slices.Compact(distinct)

	base := map[float64]float64{} // artifact value -> the value it is 10x of
	for _, v := range distinct {
		if u := v / stretchFactor; u != v && slices.Contains(distinct, u) {
			base[v] = u
		}
	}

	index := map[float64]int64{}
	var next int64
	prev := 0.0
	for _, v := range distinct {
		if _, artifact := base[v]; artifact {
			continue
		}
		if next == 0 || v-prev > clusterGap {
			next++
		}
		index[v], prev = next, v
	}
	for _, v := range distinct {
		root := v
		for {
			u, artifact := base[root]
			if !artifact {
				break
			}
			root = u
		}
		if _, ok := index[v]; !ok {
			index[v] = index[root]
		}
	}
	return index
}
