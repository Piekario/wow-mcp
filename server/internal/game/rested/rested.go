// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package rested covers rested XP: how much is in the pool, how many levels and
// "bubbles" that is, whether it covers the rest of the level and when the pool will be full after
// logging out in an inn. The accrual rules come from classic WoW and are
// only an approximation: Forever may have different ones, so the response says so.
package rested

import (
	"fmt"
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

const ruleText = "The rested pool grows by 5% of the level's XP per 8 h in an inn or city, up to 150% of the level's XP; outside an inn " +
	"it grows four times slower. While the pool lasts, monsters give double XP and the pool shrinks by the bonus XP. This is how it works in classic WoW; " +
	"Forever may count differently."

// View describes the character's rested XP and forecasts the pool filling up.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	c := s.Character()
	xpMax, _ := snapshot.AsInt(c["xpMax"])
	out := map[string]any{"source": Source, "rule": ruleText}
	if xpMax <= 0 {
		out["note"] = "No xpMax in the dump (max level or an old addon), so rested XP makes no sense."
		return out
	}
	pool, hasPool := snapshot.AsInt(c["restedXp"])
	xp, _ := snapshot.AsInt(c["xp"])
	level, _ := snapshot.AsInt(c["level"])
	if !hasPool {
		out["note"] = "The dump has no restedXp field; assuming an empty pool."
	}

	out["level"] = level
	out["restedXp"] = pool
	out["xp"], out["xpMax"] = xp, xpMax
	out["percentOfLevel"] = snapshot.Round1(float64(pool) / float64(xpMax) * 100)
	out["levelsWorth"] = round2(float64(pool) / float64(xpMax))
	out["bubbles"] = snapshot.Round1(float64(pool) / bubbleXP(xpMax))
	out["bubblesPerLevel"] = 100 / bubblePercent
	snapshot.Merge(out, restOfLevel(pool, xpMax-xp))
	if resting, ok := c["resting"].(bool); ok {
		out["resting"] = resting
		out["restingNote"] = restingNote(resting)
	}
	if loc, ok := c["bindLocation"].(string); ok && loc != "" {
		out["bindLocation"] = loc
	}
	out["pool"] = poolForecast(pool, xpMax, fresh)
	return out
}

// restOfLevel computes whether the pool covers the rest of the level. The bonus is +100% XP from monsters, so a pool P doubles P XP from monsters:
// it covers the remainder R when 2P >= R.
func restOfLevel(pool, toNext int64) map[string]any {
	out := map[string]any{"xpToNextLevel": toNext}
	if toNext <= 0 {
		return out
	}
	out["coversRestOfLevel"] = 2*pool >= toNext
	out["xpLeftAfterPool"] = max(toNext-2*pool, 0)
	return out
}

func restingNote(resting bool) string {
	if resting {
		return "According to the dump the character is resting (inn or city), so by the classic rules the pool is growing now."
	}
	return "According to the dump the character is not resting, so by the classic rules the pool grows four times slower."
}

// poolForecast describes the pool limit and the time to fill it in an inn and outside one, counted from now from the pool in the dump.
func poolForecast(pool, xpMax int64, fresh snapshot.Freshness) map[string]any {
	limit := xpMax * capPercent / 100
	out := map[string]any{
		"cap":          limit,
		"percentOfCap": snapshot.Round1(float64(pool) / float64(limit) * 100),
		"full":         pool >= limit,
	}
	if pool >= limit {
		return out
	}
	missing := limit - pool
	out["missing"] = missing
	perPeriod := float64(xpMax) * innPercentPer8h / 100
	out["inn"] = fillEstimate(missing, perPeriod, 1, fresh)
	out["outsideInn"] = fillEstimate(missing, perPeriod, outsideSlower, fresh)
	return out
}

// fillEstimate computes after what time missing XP will accrue at a gain of perPeriod per 8 h (slower times slower).
func fillEstimate(missing int64, perPeriod float64, slower int, fresh snapshot.Freshness) map[string]any {
	d := time.Duration(float64(missing) / perPeriod * float64(accrualPeriod) * float64(slower))
	return map[string]any{
		"xpPer8h":     int64(perPeriod) / int64(slower),
		"fullAfter":   snapshot.FormatAge(d),
		"fullAt":      snapshot.FormatTime(fresh.Now().Add(d).Unix()),
		"assumption":  fmt.Sprintf("from now, with the pool from the dump, without playing (%s)", Source),
		"hoursToFull": snapshot.Round1(d.Hours()),
	}
}

// bubbleXP is the XP of one bubble of the rested bar at a given level.
func bubbleXP(xpMax int64) float64 { return float64(xpMax) * bubblePercent / 100 }

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
