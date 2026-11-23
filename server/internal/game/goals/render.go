// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package goals

import (
	"fmt"
	"math"
	"strings"

	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// View evaluates the goals of the given character: progress, what is missing and the ETA.
func View(all []Goal, env Env) map[string]any {
	return Evaluate(all, env).Render()
}

func (e Evaluation) Render() map[string]any {
	list := []any{}
	for _, p := range e.Goals {
		list = append(list, p.Render(e.Rates))
	}
	out := map[string]any{"goals": list, "count": len(list), "done": e.Done}
	if len(e.Others) > 0 {
		out["otherCharacters"] = e.Others
	}
	if len(list) == 0 {
		out["note"] = "This character has no saved goals. Add them with the wow_goal_set tool (e.g. type=level, target=25)."
	}
	if e.RatesErr != nil {
		out["etaNote"] = "ETA unavailable: " + e.RatesErr.Error()
	} else if len(list) > 0 && !e.Rates.Known {
		out["etaNote"] = "The XP and gold ETA needs history with at least 5 minutes of play (run /mcp reload every so often while playing)."
	}
	return out
}

// detail is what a goal type says about itself in the answer.
type detail struct {
	current string
	missing any
	notes   []string
	extra   map[string]any
}

func (p Progress) Render(rates progress.Rates) map[string]any {
	d := p.detail()
	out := map[string]any{
		"id": p.Goal.ID, "type": p.Goal.Type, "goal": p.Goal.Label(), "done": p.Done,
		"percent": math.Round(math.Min(math.Max(p.Percent, 0), 100)*10) / 10,
	}
	if p.Goal.Note != "" {
		out["note"] = p.Goal.Note
	}
	if d.current != "" {
		out["current"] = d.current
	}
	if !p.Done && d.missing != nil {
		out["missing"] = d.missing
	}
	if len(d.notes) > 0 {
		out["notes"] = d.notes
	}
	for k, v := range d.extra {
		out[k] = v
	}
	if p.ETA != nil && !p.Done {
		out["eta"] = p.ETA.render(rates)
	}
	return out
}

func (p Progress) detail() detail {
	switch {
	case p.Level != nil:
		return p.levelDetail()
	case p.Gold != nil:
		g := p.Gold
		return detail{current: snapshot.Money(g.Money).String(), missing: snapshot.Money(p.Goal.Target - g.Money).String()}
	case p.Reputation != nil:
		return p.reputationDetail()
	case p.Skill != nil:
		return p.skillDetail()
	case p.Stock != nil:
		return p.stockDetail()
	case p.Ability != nil:
		return p.abilityDetail()
	}
	return detail{}
}

func (p Progress) levelDetail() detail {
	l := p.Level
	d := detail{
		current: fmt.Sprintf("level %d", l.Level),
		missing: fmt.Sprintf("%d levels", p.Goal.Target-l.Level),
		extra:   map[string]any{"percentBasis": "computed from level 1 to the target level, with the fraction of XP of the current level"},
	}
	if l.XPMax > 0 {
		d.current = fmt.Sprintf("level %d (%.1f%% XP)", l.Level, l.xpFraction()*100)
	}
	if l.HasXPNeeded {
		d.extra["xpNeededAtLeast"] = l.XPNeeded
	}
	return d
}

func (p Progress) reputationDetail() detail {
	r := p.Reputation
	if !r.Found {
		return detail{
			missing: "the whole goal",
			notes:   []string{fmt.Sprintf("faction %q is not in the dump (reputation section); the character does not know it yet or the name does not match", p.Goal.Faction)},
		}
	}
	d := detail{current: fmt.Sprintf("%v (%d)", r.Faction.Standing, r.Value), missing: r.Target - r.Value}
	if r.ByStanding {
		source := "classic table (approximation; Forever may have different thresholds)"
		if r.ThresholdFromGame {
			source = "threshold from the game (floor field in the dump)"
		}
		d.extra = map[string]any{"standingThreshold": r.Threshold, "thresholdSource": source}
	}
	if !strings.EqualFold(r.Faction.Name, p.Goal.Faction) {
		d.notes = append(d.notes, fmt.Sprintf("matched faction %q", r.Faction.Name))
	}
	return d
}

func (p Progress) skillDetail() detail {
	k := p.Skill
	if !k.Found {
		return detail{
			missing: "the whole profession",
			notes:   []string{fmt.Sprintf("the character has no profession or skill %q (dump: %s)", p.Goal.Name, strings.Join(k.Owned, ", "))},
		}
	}
	d := detail{current: fmt.Sprintf("%d/%d", k.Skill.Rank, k.Skill.MaxRank), missing: p.Goal.Target - k.Skill.Rank}
	if k.CapBelowGoal {
		d.notes = append(d.notes, fmt.Sprintf("the skill cap in the dump (maxRank) is %d, less than the goal %d: training alone is not enough, check in game with a profession trainer what raises the cap", k.Skill.MaxRank, p.Goal.Target))
	}
	return d
}

func (p Progress) stockDetail() detail {
	h := p.Stock.Holdings
	d := detail{
		current: fmt.Sprintf("%d (bags %d, bank %d, equipped %d)", h.Total(), h.Bags, h.Bank, h.Equipped),
		missing: p.Goal.Target - h.Total(),
	}
	if h.Bank > 0 {
		d.notes = append(d.notes, "the bank shows the state from the last visit")
	}
	return d
}

func (p Progress) abilityDetail() detail {
	a := p.Ability
	switch {
	case a.Known:
		return detail{current: "known"}
	case !a.Offered:
		return detail{
			current: "unknown", missing: "learn from a trainer",
			notes: []string{"the most recently opened trainer does not have this ability in the saved offer (or you never opened a trainer window); open the right trainer's window and type /mcp reload"},
		}
	}
	d := detail{current: "unknown", missing: "learn from a trainer", extra: map[string]any{"trainer": map[string]any{
		"npc": a.NPC, "cost": snapshot.Money(a.Cost).String(), "levelRequired": a.LevelRequired, "category": a.Category,
	}}}
	if a.canBuyNow() {
		d.notes = []string{"you can buy it right now (level and gold are enough according to the saved trainer offer)"}
		return d
	}
	var blockers []string
	if a.BlockedByLevel {
		blockers = append(blockers, fmt.Sprintf("requires level %d (you have %d)", a.LevelRequired, a.Level))
	}
	if a.BlockedByGold {
		blockers = append(blockers, fmt.Sprintf("costs %s (you have %s)", snapshot.Money(a.Cost), snapshot.Money(a.Money)))
	}
	d.missing = blockers
	return d
}

func (b Basis) render(rates progress.Rates) string {
	switch b.Kind {
	case BasisXPRate:
		return fmt.Sprintf("XP rate: %.0f per hour of play (%s, %s)", b.PerHour, rates.Window, rates.Basis)
	case BasisBalance:
		return fmt.Sprintf("gold balance: %s per hour of play (%s, %s)", snapshot.Money(int64(b.PerHour)), rates.Window, rates.Basis)
	}
	return fmt.Sprintf("%s: %.1f %s per hour of play (whole history, %s)", b.Label, b.PerHour, b.Unit, b.Source)
}

var assumptions = map[Assumption]string{
	AssumeXPTable:    "the server does not know the XP table of later levels, so I assume each needs as much XP as the current one; in reality it is often more, so this is a lower bound",
	AssumeBalance:    "this is the change in balance (it includes spending), not income; it will change when you start spending or earning differently",
	AssumeHistory:    "computed from the whole saved history, so the recent rate may differ",
	AssumeConditions: "time to meet all conditions (level, gold); I assume the rate from history",
}

func (e ETA) render(rates progress.Rates) map[string]any {
	bases := make([]string, len(e.Bases))
	for i, b := range e.Bases {
		bases[i] = b.render(rates)
	}
	out := map[string]any{
		"playHours": snapshot.Round1(e.Hours),
		"duration":  snapshot.FormatSeconds(int64(e.Hours * 3600)),
		"basis":     strings.Join(bases, "; "),
	}
	if rates.Known && rates.SessionHours > 0 {
		out["sessions"] = snapshot.Round1(e.Hours / rates.SessionHours)
		out["sessionLength"] = fmt.Sprintf("on average %s of play per session from history", snapshot.FormatSeconds(int64(rates.SessionHours*3600)))
	}
	out["assumption"] = assumptions[e.Assumption]
	return out
}
