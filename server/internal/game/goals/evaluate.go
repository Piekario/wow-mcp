// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package goals

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"wowmcp/internal/game/consumption"
	"wowmcp/internal/game/items"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/snapshot"
)

// History is what goals need from the dump history (progress.Tracker satisfies it).
type History interface {
	Rates(name string, current snapshot.Snapshot) (progress.Rates, error)
	WholeHistory(name string, current snapshot.Snapshot) (progress.Stretch, error)
}

// Env is the character whose goals are evaluated: its dump and (optionally) history for the ETA.
type Env struct {
	Character string
	Snap      snapshot.Snapshot
	History   History // nil = no ETA
}

// View evaluates the goals of the given character: progress, what is missing and the ETA. Goals of other characters
// are only counted (otherCharacters).
func View(all []Goal, env Env) map[string]any {
	ev := newEvaluator(env)
	list := []any{}
	others := map[string]int{}
	var done int
	for _, g := range all {
		if g.Character != env.Character {
			others[g.Character]++
			continue
		}
		r := ev.evaluate(g)
		if r["done"] == true {
			done++
		}
		list = append(list, r)
	}
	out := map[string]any{"goals": list, "count": len(list), "done": done}
	if len(others) > 0 {
		out["otherCharacters"] = others
	}
	if len(list) == 0 {
		out["note"] = "This character has no saved goals. Add them with the wow_goal_set tool (e.g. type=level, target=25)."
	}
	if ev.ratesErr != nil {
		out["etaNote"] = "ETA unavailable: " + ev.ratesErr.Error()
	} else if len(list) > 0 && !ev.rates.Known {
		out["etaNote"] = "The XP and gold ETA needs history with at least 5 minutes of play (run /mcp reload every so often while playing)."
	}
	return out
}

// evaluator computes the rate from history once per call, because many goals use it.
type evaluator struct {
	env      Env
	rates    progress.Rates
	ratesErr error
}

func newEvaluator(env Env) *evaluator {
	ev := &evaluator{env: env}
	if env.History == nil {
		ev.ratesErr = fmt.Errorf("dump history is disabled")
		return ev
	}
	ev.rates, ev.ratesErr = env.History.Rates(env.Character, env.Snap)
	return ev
}

// outcome is the result of evaluating one goal before it is written into the response.
type outcome struct {
	current, missing any
	percent          float64
	done             bool
	notes            []string
	extra            map[string]any
	eta              *eta
}

// eta is the estimated play time to the goal at the rate from history.
type eta struct {
	hours      float64
	basis      string
	assumption string
}

func (ev *evaluator) evaluate(g Goal) map[string]any {
	var o outcome
	switch g.Type {
	case Level:
		o = ev.level(g)
	case Gold:
		o = ev.gold(g)
	case Reputation:
		o = ev.reputation(g)
	case Profession:
		o = ev.profession(g)
	case Spell:
		o = ev.spell(g)
	case Item:
		o = ev.item(g)
	}
	out := map[string]any{
		"id": g.ID, "type": g.Type, "goal": g.Label(), "done": o.done,
		"percent": math.Round(math.Min(math.Max(o.percent, 0), 100)*10) / 10,
	}
	if g.Note != "" {
		out["note"] = g.Note
	}
	if o.current != nil {
		out["current"] = o.current
	}
	if !o.done && o.missing != nil {
		out["missing"] = o.missing
	}
	if len(o.notes) > 0 {
		out["notes"] = o.notes
	}
	for k, v := range o.extra {
		out[k] = v
	}
	if o.eta != nil && !o.done {
		out["eta"] = ev.describeETA(*o.eta)
	}
	return out
}

func (ev *evaluator) describeETA(e eta) map[string]any {
	out := map[string]any{
		"playHours": snapshot.Round1(e.hours),
		"duration":  snapshot.FormatSeconds(int64(e.hours * 3600)),
		"basis":     e.basis,
	}
	if ev.rates.Known && ev.rates.SessionHours > 0 {
		out["sessions"] = snapshot.Round1(e.hours / ev.rates.SessionHours)
		out["sessionLength"] = fmt.Sprintf("on average %s of play per session from history", snapshot.FormatSeconds(int64(ev.rates.SessionHours*3600)))
	}
	if e.assumption != "" {
		out["assumption"] = e.assumption
	}
	return out
}

// paceBasis describes where the XP or gold rate comes from.
func (ev *evaluator) paceBasis(what string, perHour float64) string {
	return fmt.Sprintf("%s: %.0f per hour of play (%s, %s)", what, perHour, ev.rates.Window, ev.rates.Basis)
}

func (ev *evaluator) level(g Goal) outcome {
	c := ev.env.Snap.Character()
	level, _ := snapshot.AsInt(c["level"])
	xp, _ := snapshot.AsInt(c["xp"])
	xpMax, _ := snapshot.AsInt(c["xpMax"])
	o := outcome{current: fmt.Sprintf("level %d", level), done: level >= g.Target}
	if xpMax > 0 {
		frac := float64(xp) / float64(xpMax)
		o.current = fmt.Sprintf("level %d (%.1f%% XP)", level, frac*100)
		if g.Target > 1 {
			o.percent = (float64(level-1) + frac) / float64(g.Target-1) * 100
		}
	}
	o.missing = fmt.Sprintf("%d levels", g.Target-level)
	o.extra = map[string]any{"percentBasis": "computed from level 1 to the target level, with the fraction of XP of the current level"}
	if o.done || xpMax <= 0 {
		return o
	}
	needed := (xpMax - xp) + (g.Target-level-1)*xpMax
	o.extra["xpNeededAtLeast"] = needed
	if ev.rates.Known && ev.rates.XPPerHour > 0 {
		o.eta = &eta{
			hours:      float64(needed) / ev.rates.XPPerHour,
			basis:      ev.paceBasis("XP rate", ev.rates.XPPerHour),
			assumption: "the server does not know the XP table of later levels, so I assume each needs as much XP as the current one; in reality it is often more, so this is a lower bound",
		}
	}
	return o
}

func (ev *evaluator) gold(g Goal) outcome {
	_, money := ev.env.Snap.LevelAndMoney()
	o := outcome{
		current: snapshot.Money(money).String(), done: money >= g.Target,
		missing: snapshot.Money(g.Target - money).String(), percent: float64(money) / float64(g.Target) * 100,
	}
	if !o.done && ev.rates.Known && ev.rates.MoneyPerHour > 0 {
		o.eta = &eta{
			hours:      float64(g.Target-money) / ev.rates.MoneyPerHour,
			basis:      fmt.Sprintf("gold balance: %s per hour of play (%s, %s)", snapshot.Money(int64(ev.rates.MoneyPerHour)), ev.rates.Window, ev.rates.Basis),
			assumption: "this is the change in balance (it includes spending), not income; it will change when you start spending or earning differently",
		}
	}
	return o
}

// Successive reputation standings and their lower thresholds from the classic game (an approximation). When the dump has a faction at a given
// standing, the threshold is taken from its floor field, i.e. from the game.
var standings = []struct {
	name  string
	floor int64
}{
	{"Hated", -42000}, {"Hostile", -6000}, {"Unfriendly", -3000}, {"Neutral", 0},
	{"Friendly", 3000}, {"Honored", 9000}, {"Revered", 21000}, {"Exalted", 42000},
}

func canonicalStanding(name string) (string, bool) {
	for _, s := range standings {
		if strings.EqualFold(s.name, name) {
			return s.name, true
		}
	}
	return "", false
}

func standingNames() []string {
	out := make([]string, len(standings))
	for i, s := range standings {
		out[i] = s.name
	}
	return out
}

// standingFloor returns the reputation threshold for a standing; fromGame = true when it comes from the dump, not the classic table.
func standingFloor(s snapshot.Snapshot, name string) (floor int64, fromGame bool) {
	for _, raw := range s.List("reputation") {
		f := snapshot.AsMap(raw)
		if st, _ := f["standing"].(string); strings.EqualFold(st, name) {
			if v, ok := snapshot.AsInt(f["floor"]); ok {
				return v, true
			}
		}
	}
	for _, st := range standings {
		if st.name == name {
			return st.floor, false
		}
	}
	return 0, false
}

// findByName picks a list entry by name: an exact (case-insensitive) match first, then by fragment.
func findByName(list []any, name string) map[string]any {
	var partial map[string]any
	for _, raw := range list {
		e := snapshot.AsMap(raw)
		n, _ := e["name"].(string)
		switch {
		case strings.EqualFold(n, name):
			return e
		case partial == nil && strings.Contains(strings.ToLower(n), strings.ToLower(name)):
			partial = e
		}
	}
	return partial
}

func (ev *evaluator) reputation(g Goal) outcome {
	f := findByName(ev.env.Snap.List("reputation"), g.Faction)
	if f == nil {
		return outcome{notes: []string{fmt.Sprintf("faction %q is not in the dump (reputation section); the character does not know it yet or the name does not match", g.Faction)}, missing: "the whole goal"}
	}
	value, _ := snapshot.AsInt(f["value"])
	target, o := g.Target, outcome{}
	if g.Standing != "" {
		floor, fromGame := standingFloor(ev.env.Snap, g.Standing)
		target = floor
		src := "classic table (approximation; Forever may have different thresholds)"
		if fromGame {
			src = "threshold from the game (floor field in the dump)"
		}
		o.extra = map[string]any{"standingThreshold": floor, "thresholdSource": src}
	}
	o.current = fmt.Sprintf("%v (%d)", f["standing"], value)
	o.done = value >= target
	o.missing = target - value
	if target > 0 {
		o.percent = float64(value) / float64(target) * 100
	} else if o.done {
		o.percent = 100
	}
	if name, _ := f["name"].(string); !strings.EqualFold(name, g.Faction) {
		o.notes = append(o.notes, fmt.Sprintf("matched faction %q", name))
	}
	if !o.done {
		o.eta = ev.trend("reputation rate", "pts", func(s snapshot.Snapshot) (float64, bool) {
			cur := findByName(s.List("reputation"), fmt.Sprint(f["name"]))
			v, ok := snapshot.AsFloat(cur["value"])
			return v, cur != nil && ok
		}, float64(target-value))
	}
	return o
}

func (ev *evaluator) profession(g Goal) outcome {
	p := findByName(ev.env.Snap.List("professions"), g.Name)
	if p == nil {
		return outcome{missing: "the whole profession", notes: []string{fmt.Sprintf("the character has no profession or skill %q (dump: %s)", g.Name, strings.Join(professionNames(ev.env.Snap), ", "))}}
	}
	rank, _ := snapshot.AsInt(p["rank"])
	maxRank, _ := snapshot.AsInt(p["maxRank"])
	o := outcome{
		current: fmt.Sprintf("%d/%d", rank, maxRank), done: rank >= g.Target,
		missing: g.Target - rank, percent: float64(rank) / float64(g.Target) * 100,
	}
	if maxRank > 0 && maxRank < g.Target && !o.done {
		o.notes = append(o.notes, fmt.Sprintf("the skill cap in the dump (maxRank) is %d, less than the goal %d: training alone is not enough, check in game with a profession trainer what raises the cap", maxRank, g.Target))
	}
	if !o.done {
		o.eta = ev.trend("skill rate", "pts", func(s snapshot.Snapshot) (float64, bool) {
			cur := findByName(s.List("professions"), fmt.Sprint(p["name"]))
			v, ok := snapshot.AsFloat(cur["rank"])
			return v, cur != nil && ok
		}, float64(g.Target-rank))
	}
	return o
}

func professionNames(s snapshot.Snapshot) []string {
	var out []string
	for _, raw := range s.List("professions") {
		out = append(out, fmt.Sprint(snapshot.AsMap(raw)["name"]))
	}
	return out
}

func (ev *evaluator) item(g Goal) outcome {
	h := items.Count(ev.env.Snap, g.Item)
	have := h.Total()
	o := outcome{
		current: fmt.Sprintf("%d (bags %d, bank %d, equipped %d)", have, h.Bags, h.Bank, h.Equipped),
		done:    have >= g.Target, missing: g.Target - have, percent: float64(have) / float64(g.Target) * 100,
	}
	if h.Bank > 0 {
		o.notes = append(o.notes, "the bank shows the state from the last visit")
	}
	if !o.done {
		o.eta = ev.trend("gain rate", "pcs", func(s snapshot.Snapshot) (float64, bool) {
			return float64(items.Count(s, g.Item).Total()), true
		}, float64(g.Target-have))
	}
	return o
}

// trend computes the rate of increase of a value from the whole history (first to last dump) and the time to cover the shortfall.
// nil when there is no history, the stretch is shorter than 5 minutes of play, or the value is not growing.
func (ev *evaluator) trend(what, unit string, value func(snapshot.Snapshot) (float64, bool), missing float64) *eta {
	if ev.env.History == nil {
		return nil
	}
	st, err := ev.env.History.WholeHistory(ev.env.Character, ev.env.Snap)
	if err != nil || len(st.Snapshots) < 2 || st.ElapsedSeconds < consumption.MinElapsed {
		return nil
	}
	first, ok1 := value(st.Snapshots[0])
	last, ok2 := value(st.Snapshots[len(st.Snapshots)-1])
	if !ok1 || !ok2 || last <= first {
		return nil
	}
	perHour := (last - first) / st.Hours()
	return &eta{
		hours:      missing / perHour,
		basis:      fmt.Sprintf("%s: %.1f %s per hour of play (whole history, %s)", what, perHour, unit, st.Basis),
		assumption: "computed from the whole saved history, so the recent rate may differ",
	}
}

func (ev *evaluator) spell(g Goal) outcome {
	s := ev.env.Snap
	if known := spellKnown(s, g.Name); known {
		return outcome{current: "known", done: true, percent: 100}
	}
	o := outcome{current: "unknown", missing: "learn from a trainer"}
	svc := trainerService(s, g.Name)
	if svc == nil {
		o.notes = append(o.notes, "the most recently opened trainer does not have this ability in the saved offer (or you never opened a trainer window); open the right trainer's window and type /mcp reload")
		return o
	}
	level, money := s.LevelAndMoney()
	cost, _ := snapshot.AsInt(svc["costCopper"])
	req, _ := snapshot.AsInt(svc["levelRequired"])
	var blockers []string
	var hours float64
	var basis []string
	if req > level {
		blockers = append(blockers, fmt.Sprintf("requires level %d (you have %d)", req, level))
		if ev.rates.Known && ev.rates.XPPerHour > 0 {
			if lv := ev.level(Goal{Type: Level, Target: req}); lv.eta != nil {
				hours, basis = math.Max(hours, lv.eta.hours), append(basis, lv.eta.basis)
			}
		}
	}
	if cost > money {
		blockers = append(blockers, fmt.Sprintf("costs %s (you have %s)", snapshot.Money(cost), snapshot.Money(money)))
		if g := ev.gold(Goal{Type: Gold, Target: cost}); g.eta != nil {
			hours, basis = math.Max(hours, g.eta.hours), append(basis, g.eta.basis)
		}
	}
	o.extra = map[string]any{"trainer": map[string]any{
		"npc": s.Map("trainer")["npc"], "cost": snapshot.Money(cost).String(), "levelRequired": req, "category": svc["category"],
	}}
	if len(blockers) == 0 {
		o.notes = append(o.notes, "you can buy it right now (level and gold are enough according to the saved trainer offer)")
		return o
	}
	o.missing = blockers
	if hours > 0 {
		o.eta = &eta{hours: hours, basis: strings.Join(basis, "; "), assumption: "time to meet all conditions (level, gold); I assume the rate from history"}
	}
	return o
}

// spellKnown: the ability is in the spellbook ("Name" or "Name Rank") or the trainer marks it as known.
func spellKnown(s snapshot.Snapshot, name string) bool {
	for _, raw := range s.List("spells") {
		sp := snapshot.AsMap(raw)
		n, _ := sp["name"].(string)
		rank, _ := sp["rank"].(string)
		if strings.EqualFold(n, name) || strings.EqualFold(strings.TrimSpace(n+" "+rank), name) {
			return true
		}
	}
	svc := trainerService(s, name)
	return svc != nil && svc["category"] == "used"
}

// trainerService looks for an ability in the last trainer offer (by name, case-insensitive).
func trainerService(s snapshot.Snapshot, name string) map[string]any {
	var candidates []map[string]any
	for _, raw := range snapshot.AsList(s.Map("trainer")["services"]) {
		svc := snapshot.AsMap(raw)
		n, _ := svc["name"].(string)
		rank, _ := svc["rank"].(string)
		if strings.EqualFold(n, name) || strings.EqualFold(strings.TrimSpace(n+" "+rank), name) {
			candidates = append(candidates, svc)
		}
	}
	// Several ranks with the same name: the first not yet known (available/unavailable) is the next step.
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i]["category"] != "used" && candidates[j]["category"] == "used" })
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

func goldText(copper int64) string {
	return snapshot.Money(copper).String()
}
