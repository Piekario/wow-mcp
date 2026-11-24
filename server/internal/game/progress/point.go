// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import "wowmcp/internal/snapshot"

// Point is what the long history keeps of one dump: the character's numbers, without the dump itself.
// A number the dump did not have is -1.
type Point struct {
	At                              int64
	Level, XP, XPMax, Money, Rested int64
	Played, Deaths                  int64
	CompletedQuests                 int64
	Zone                            string
}

// Series is the long history port: one Point per dump, oldest first, kept beyond the History limit.
type Series interface {
	Points(character string) ([]Point, error)
}

// PointOf reads the Point of a dump; false for a dump without a time or from an empty client.
func PointOf(s snapshot.Snapshot) (Point, bool) {
	p, ok := readPoint(s)
	if !ok || s.IsUnloadedDump() {
		return Point{}, false
	}
	zone, _ := p.char.Zone.(string)
	return Point{
		At:    p.at,
		Level: known(p.char.Level, p.char.HasLevel), XP: known(p.char.XP, p.char.HasXP),
		XPMax: known(p.char.XPMax, p.char.HasXPMax), Money: known(p.char.Money, p.char.HasMoney),
		Rested: p.char.Rested, Played: p.played, Deaths: p.deaths,
		CompletedQuests: int64(p.completed), Zone: zone,
	}, true
}

func known(v int64, has bool) int64 {
	if !has {
		return -1
	}
	return v
}
