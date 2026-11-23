// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal

import (
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// minRecordSeconds: sessions shorter than 5 minutes do not give reliable records.
const minRecordSeconds = 5 * 60

// Records are the character's records from the whole history; a nil field is a record not set yet.
type Records struct {
	Longest, BestGold *Session
	FastestLevel      *LevelTime
}

func (r Records) Empty() bool { return r.Longest == nil && r.BestGold == nil && r.FastestLevel == nil }

// LevelTime is the play time spent at Level, which ended with the dump at ReachedAt.
type LevelTime struct {
	Level, Seconds, ReachedAt int64
}

// records picks the longest session, the session with the best gold balance and the fastest level (from /played play time).
func records(all []Session, stretches []progress.Stretch) Records {
	var r Records
	for i := range all {
		s := &all[i]
		if s.ElapsedSeconds < minRecordSeconds {
			continue
		}
		if r.Longest == nil || s.ElapsedSeconds > r.Longest.ElapsedSeconds {
			r.Longest = s
		}
		if s.MoneyDelta > 0 && (r.BestGold == nil || s.MoneyDelta > r.BestGold.MoneyDelta) {
			r.BestGold = s
		}
	}
	var snaps []snapshot.Snapshot
	for _, st := range stretches {
		snaps = append(snaps, st.Snapshots...)
	}
	if level, ok := fastestLevel(snaps); ok {
		r.FastestLevel = &level
	}
	return r
}

// fastestLevel looks for the shortest level: for dumps a, b with a one-level advance, the play time at level a
// is a.levelSeconds + (play time between the dumps) - b.levelSeconds (the part b already spent on the new level).
func fastestLevel(snaps []snapshot.Snapshot) (LevelTime, bool) {
	var best LevelTime
	found := false
	for i := 1; i < len(snaps); i++ {
		a, b := snaps[i-1], snaps[i]
		la, _ := a.LevelAndMoney()
		lb, _ := b.LevelAndMoney()
		ta, lsa, ok1 := readPlayed(a)
		tb, lsb, ok2 := readPlayed(b)
		if lb != la+1 || !ok1 || !ok2 {
			continue
		}
		seconds := lsa + (tb - ta) - lsb
		if seconds <= 0 {
			continue
		}
		if !found || seconds < best.Seconds {
			at, _ := b.DumpedAt()
			best, found = LevelTime{Level: la, Seconds: seconds, ReachedAt: at}, true
		}
	}
	return best, found
}
