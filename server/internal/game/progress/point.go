// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package progress

import "wowmcp/internal/snapshot"

// Point is what the long history keeps of one dump: the character's numbers, without the dump itself.
// A number the dump did not have (including honor without a pvp section) is -1.
type Point struct {
	At                              int64
	Level, XP, XPMax, Money, Rested int64
	Played, Deaths                  int64
	CompletedQuests                 int64
	Honor, HonorableKills           int64
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
		CompletedQuests: int64(p.completed), Zone: zone, Honor: p.honor, HonorableKills: p.kills,
	}, true
}

// progressPoint is the Point as the rate computations see it: no dump behind it.
func (p Point) progressPoint() progressPoint {
	level, hasLevel := unknownAsZero(p.Level)
	xp, hasXP := unknownAsZero(p.XP)
	xpMax, hasXPMax := unknownAsZero(p.XPMax)
	money, hasMoney := unknownAsZero(p.Money)
	return progressPoint{
		at: p.At, played: p.Played, deaths: p.Deaths, honor: p.Honor, kills: p.HonorableKills, completed: int(max(p.CompletedQuests, 0)),
		char: characterState{
			Level: level, XP: xp, XPMax: xpMax, Money: money, Rested: max(p.Rested, 0),
			HasLevel: hasLevel, HasXP: hasXP, HasXPMax: hasXPMax, HasMoney: hasMoney,
		},
	}
}

func unknownAsZero(v int64) (int64, bool) {
	if v < 0 {
		return 0, false
	}
	return v, true
}

// withOlderPoints puts the Series points older than the oldest dump in front of the dump points. A Series error
// leaves the dump points alone: the long history only extends them.
func (t Tracker) withOlderPoints(name string, points []progressPoint) []progressPoint {
	if t.Series == nil || len(points) == 0 {
		return points
	}
	series, err := t.Series.Points(name)
	if err != nil {
		return points
	}
	var older []progressPoint
	for _, p := range series {
		if p.At < points[0].at && p.Level >= 0 && p.Money >= 0 { // without them a point would read as a drop to zero
			older = append(older, p.progressPoint())
		}
	}
	return append(older, points...)
}

func known(v int64, has bool) int64 {
	if !has {
		return -1
	}
	return v
}
