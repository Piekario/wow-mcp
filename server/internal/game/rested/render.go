// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package rested

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

const ruleText = "The rested pool grows by 5% of the level's XP per 8 h in an inn or city, up to 150% of the level's XP; outside an inn " +
	"it grows four times slower. While the pool lasts, monsters give double XP and the pool shrinks by the bonus XP. This is how it works in classic WoW; " +
	"Forever may count differently."

// View describes the character's rested XP and forecasts the pool filling up.
func View(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	return Inspect(read(s), fresh).Render()
}

func (r Report) Render() map[string]any {
	out := map[string]any{"source": Source, "rule": ruleText}
	if !r.Valid {
		out["note"] = "No xpMax in the dump (max level or an old addon), so rested XP makes no sense."
		return out
	}
	if !r.HasRested {
		out["note"] = "The dump has no restedXp field; assuming an empty pool."
	}
	out["level"] = r.Level
	out["restedXp"] = r.Rested
	out["xp"], out["xpMax"] = r.XP, r.XPMax
	out["percentOfLevel"] = snapshot.Round1(float64(r.Rested) / float64(r.XPMax) * 100)
	out["levelsWorth"] = round2(float64(r.Rested) / float64(r.XPMax))
	out["bubbles"] = snapshot.Round1(r.Bubbles())
	out["bubblesPerLevel"] = 100 / bubblePercent
	out["xpToNextLevel"] = r.ToNextLevel
	if r.ToNextLevel > 0 {
		out["coversRestOfLevel"] = r.CoversRest
		out["xpLeftAfterPool"] = r.XPLeftAfter
	}
	if r.HasResting {
		out["resting"] = r.Resting
		out["restingNote"] = restingNote(r.Resting)
	}
	if r.BindLocation != "" {
		out["bindLocation"] = r.BindLocation
	}
	out["pool"] = r.Pool.render(r.Rested)
	return out
}

func restingNote(resting bool) string {
	if resting {
		return "According to the dump the character is resting (inn or city), so by the classic rules the pool is growing now."
	}
	return "According to the dump the character is not resting, so by the classic rules the pool grows four times slower."
}

func (p Pool) render(rested int64) map[string]any {
	out := map[string]any{
		"cap":          p.Cap,
		"percentOfCap": snapshot.Round1(float64(rested) / float64(p.Cap) * 100),
		"full":         p.Full,
	}
	if p.Full {
		return out
	}
	out["missing"] = p.Missing
	out["inn"] = p.Inn.render()
	out["outsideInn"] = p.Outside.render()
	return out
}

func (f Fill) render() map[string]any {
	return map[string]any{
		"xpPer8h":     f.XPPer8h,
		"fullAfter":   snapshot.FormatAge(f.After),
		"fullAt":      snapshot.FormatTime(f.At),
		"assumption":  fmt.Sprintf("from now, with the pool from the dump, without playing (%s)", Source),
		"hoursToFull": snapshot.Round1(f.After.Hours()),
	}
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
