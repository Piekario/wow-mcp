// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package rested covers rested XP: how much is in the pool, how many levels and
// "bubbles" that is, whether it covers the rest of the level and when the pool will be full after
// logging out in an inn. The accrual rules come from classic WoW and are
// only an approximation: Forever may have different ones, so the response says so.
package rested

import (
	"time"

	"wowmcp/internal/snapshot"
)

// Source marks in the response where the accrual rules come from.
const Source = "classic rule, approximate"

// Classic WoW rules (values in percent of the XP of a whole level).
const (
	innPercentPer8h = 5   // gain in an inn or city per 8 hours
	capPercent      = 150 // maximum pool
	bubblePercent   = 5   // one "bubble" of the rested bar; 20 per level
	outsideSlower   = 4   // outside an inn it accrues this many times slower
	accrualPeriod   = 8 * time.Hour
)

// Character is what the dump says about the XP bar. HasRested is false when the dump has no restedXp.
type Character struct {
	Level, XP, XPMax, Rested int64
	HasRested                bool
	Resting, HasResting      bool
	BindLocation             string
}

// Fill is the forecast of the pool filling up at one accrual speed.
type Fill struct {
	XPPer8h int64
	After   time.Duration
	At      int64
}

// Pool is the rested pool against its limit. Missing, Inn and Outside are set only when it is not full.
type Pool struct {
	Cap, Missing int64
	Full         bool
	Inn, Outside Fill
}

// Report is the rested XP judged against the clock. Valid is false when the level has no XP bar (max level, old addon).
type Report struct {
	Character
	Valid       bool
	ToNextLevel int64
	CoversRest  bool
	XPLeftAfter int64
	Pool        Pool
}

// Inspect works out what the pool is worth. The bonus is +100% XP from monsters, so a pool P doubles P XP from
// monsters: it covers the remainder R of the level when 2P >= R.
func Inspect(c Character, fresh snapshot.Freshness) Report {
	if c.XPMax <= 0 {
		return Report{Character: c}
	}
	r := Report{Character: c, Valid: true, ToNextLevel: c.XPMax - c.XP}
	if r.ToNextLevel > 0 {
		r.CoversRest = 2*c.Rested >= r.ToNextLevel
		r.XPLeftAfter = max(r.ToNextLevel-2*c.Rested, 0)
	}
	r.Pool = poolOf(c, fresh)
	return r
}

func poolOf(c Character, fresh snapshot.Freshness) Pool {
	limit := c.XPMax * capPercent / 100
	p := Pool{Cap: limit, Full: c.Rested >= limit}
	if p.Full {
		return p
	}
	p.Missing = limit - c.Rested
	perPeriod := float64(c.XPMax) * innPercentPer8h / 100
	p.Inn = fillAt(p.Missing, perPeriod, 1, fresh)
	p.Outside = fillAt(p.Missing, perPeriod, outsideSlower, fresh)
	return p
}

// fillAt computes after what time missing XP will accrue at a gain of perPeriod per 8 h (slower times slower).
func fillAt(missing int64, perPeriod float64, slower int, fresh snapshot.Freshness) Fill {
	d := time.Duration(float64(missing) / perPeriod * float64(accrualPeriod) * float64(slower))
	return Fill{XPPer8h: int64(perPeriod) / int64(slower), After: d, At: fresh.Now().Add(d).Unix()}
}

// Bubbles is the pool in bubbles of the rested bar.
func (c Character) Bubbles() float64 {
	return float64(c.Rested) / (float64(c.XPMax) * bubblePercent / 100)
}
