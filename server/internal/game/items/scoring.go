// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package items

import (
	"math"
	"sort"

	"wowmcp/internal/snapshot"
)

// SlotScores is the weighted score of each slot comparison (Scores[i] belongs to the i-th comparison).
// A weights file that cannot be used leaves only Note; no weights at all leaves everything empty.
type SlotScores struct {
	Weights  Weights
	Source   string
	Note     string
	Scores   []float64
	BestSlot string
	Best     float64
	HasBest  bool
	Ignored  []string // stats without a weight, sorted
}

func (sc SlotScores) Weighted() bool { return len(sc.Weights) > 0 }

// scoreSlots weighs the comparisons: weights from the query, then from the file. A corrupt weights file does not
// break the comparison, it only gives a note; bad weights in the query are an error.
func scoreSlots(cmp []SlotComparison, s snapshot.Snapshot, q CompareQuery, sc Scoring) (SlotScores, error) {
	weights, source, note, err := ChooseWeights(q.Weights, sc, s)
	if err != nil {
		return SlotScores{}, err
	}
	if note != "" {
		return SlotScores{Note: note}, nil
	}
	if len(weights) == 0 {
		return SlotScores{}, nil
	}
	r := SlotScores{Weights: weights, Source: source, Best: math.Inf(-1)}
	ignored := map[string]bool{}
	for _, c := range cmp {
		score, unweighted := weights.score(c.Delta)
		r.Scores = append(r.Scores, score)
		for _, stat := range unweighted {
			ignored[stat] = true
		}
		if score > r.Best {
			r.Best, r.BestSlot, r.HasBest = score, c.Slot, true
		}
	}
	for stat := range ignored {
		r.Ignored = append(r.Ignored, stat)
	}
	sort.Strings(r.Ignored)
	return r, nil
}
