// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package pet

import (
	"fmt"

	"wowmcp/internal/snapshot"
)

// NoPetNote says why there is no pet section: the character has no pet or the section is disabled.
const NoPetNote = "The dump has no pet section: the character has no pet (not a pet class) or the section is disabled in the addon (/mcp off pet; enable it with /mcp on pet)."

// FoodNote explains why the server does not match food to the diet itself.
const FoodNote = "foodInBags is food from bags by item subtype; the server has no table of which food is meat, fish, cheese etc. " +
	"Match it to the pet's foodTypes by name, and when unsure, say so plainly."

// View describes the pet. When there is no section or the pet is not summoned, it returns only an explanation.
func View(s snapshot.Snapshot) map[string]any { return readPet(s).Render() }

func (p Pet) Render() map[string]any {
	switch {
	case !p.Present:
		return map[string]any{"exists": false, "note": NoPetNote}
	case !p.Summoned:
		return map[string]any{"exists": false, "note": "The pet section is in the dump, but the pet was not summoned at dump time (dead, in the stable or dismissed)."}
	}
	out := map[string]any{"exists": true}
	d := p.Shown
	for _, f := range []struct {
		key   string
		value any
	}{
		{"name", d.Name}, {"family", d.Family}, {"level", d.Level}, {"dead", d.Dead}, {"damagePercent", d.DamagePercent},
		{"loyaltyRate", d.LoyaltyRate}, {"loyalty", d.Loyalty}, {"foodTypes", d.FoodTypes},
		{"happinessInfo", d.HappinessInfo}, {"loyaltyInfo", d.LoyaltyInfo}, {"trainingPointsInfo", d.TrainingPointsInfo},
	} {
		if f.value != nil {
			out[f.key] = f.value
		}
	}
	if p.HasHappiness {
		out["happiness"] = map[string]any{"value": p.Happiness, "name": HappinessName(p.Happiness)}
	}
	if pct, ok := p.XPPercent(); ok {
		out["xpPercent"] = pct
	}
	if p.Points.Known() {
		out["trainingPoints"] = p.Points.render()
	}
	if abilities := abilityLabels(p.Learned()); len(abilities) > 0 {
		out["abilities"] = abilities
	}
	if auto := p.autoCast(); len(auto) > 0 {
		out["autoCast"] = auto
	}
	return out
}

func (t TrainingPoints) render() map[string]any {
	out := map[string]any{}
	if t.HasTotal {
		out["total"] = t.Total
	}
	if t.HasSpent {
		out["spent"] = t.Spent
	}
	if t.HasFree {
		out["free"] = t.Free
	}
	return out
}

// abilityLabels names the abilities with their rank: "Growl Rank 3".
func abilityLabels(abilities []Ability) []string {
	var out []string
	for _, a := range abilities {
		name := a.Name
		if a.Rank != "" {
			name += " " + a.Rank
		}
		out = append(out, name)
	}
	return out
}

// autoCast says which abilities on the pet bar run automatically (true) or not (false).
func (p Pet) autoCast() map[string]any {
	out := map[string]any{}
	for _, slot := range p.Bar {
		if slot.HasAutoCast {
			out[slot.Name] = slot.AutoCast
		}
	}
	return out
}

// RenderFood is the answer's list of food stacks; never nil.
func RenderFood(food []FoodStack) []any {
	out := []any{}
	for _, f := range food {
		out = append(out, map[string]any{"name": f.Name, "count": f.Count, "id": f.ID})
	}
	return out
}

// FoodInBags is BagFood for the answer.
func FoodInBags(s snapshot.Snapshot) []any { return RenderFood(readBagFood(s)) }

// Review collects the pet description extras: ability ranks, the training points plan and the stable comparison.
// Sections whose data is not in the dump say what to open in game. For a character without an active pet it returns only the stable.
func Review(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	return Examine(s).Render(fresh)
}

func (c Checkup) Render(fresh snapshot.Freshness) map[string]any {
	out := map[string]any{}
	if c.Pet.Summoned {
		out["rankReview"] = c.Ranks.render()
		out["trainingPlan"] = c.Plan.render()
	}
	if c.HasStable {
		out["stable"] = c.Stable.render(fresh)
	}
	return out
}

// Summarize adds Review to the wow_pet response.
func Summarize(s snapshot.Snapshot, out map[string]any, fresh snapshot.Freshness) {
	snapshot.Merge(out, Review(s, fresh))
}

// trainerWindowNote says where the lesson list came from when the Beast Training window was saved as a trainer window.
const trainerWindowNote = "The Beast Training window was saved as a trainer window without an NPC (this is how Forever 1.60.1 shows it). " +
	"Assumption, not confirmed in game: its cost is the pet's training points, and the list contains what the window showed under the in-game filters."

func (r RankReview) render() map[string]any {
	out := map[string]any{}
	if r.Lessons.Found {
		if r.Lessons.FromTrainer {
			out["windowNote"] = trainerWindowNote
		}
		learn := []any{}
		for _, t := range r.Learnable {
			e := map[string]any{"ability": t.Label(), "petHas": rankLabel(t.PetRank), "petLevelOk": t.PetLevelOK, "levelRequired": t.LevelRequired}
			if t.HasPoints {
				e["trainingPoints"] = t.Points
			}
			learn = append(learn, e)
		}
		out["learnable"] = learn
		if len(learn) == 0 {
			out["learnableNote"] = "the Beast Training window has no ranks higher than those the pet already has"
		}
	} else {
		out["learnableNote"] = "no data: open the Beast Training window (Hunter) in game and run /mcp reload, then I will see which ranks you know and can teach the pet"
	}
	if len(r.NotLearned) > 0 {
		out["notLearnedByHunter"] = renderTrainerServices(r.NotLearned)
		out["notLearnedNote"] = "ranks from remembered trainer offers that are not in the Beast Training window (the pet does not have them, the Hunter does not know them yet); " +
			"matched only by the pet ability name"
	}
	out["source"] = "only data from the dump (the Beast Training window, pet abilities, trainer offers); the server has no rank table of its own"
	return out
}

func renderTrainerServices(services []TrainerService) []any {
	var out []any
	for _, svc := range services {
		e := map[string]any{"ability": svc.Name + " " + svc.Rank, "trainer": svc.Trainer, "category": svc.Category}
		if svc.HasCost {
			e["cost"] = snapshot.Money(svc.Cost).String()
		}
		if svc.HasLevel {
			e["levelRequired"] = svc.LevelRequired
		}
		out = append(out, e)
	}
	return out
}

func rankLabel(n int) string {
	if n == 0 {
		return "none"
	}
	return fmt.Sprintf("Rank %d", n)
}

func (t TrainingPlan) render() map[string]any {
	out := map[string]any{}
	if t.HasFree {
		out["freePoints"] = t.Free
	} else {
		out["note"] = "the addon did not save the pet's training points (trainingPointsFree); I do not know how many are free"
	}
	if !t.HasLessons {
		out["learnableNote"] = "no data: open the Beast Training window in game and run /mcp reload, then I will compute the lesson costs"
		return out
	}
	if t.HasFree {
		suggested := []any{}
		for _, s := range t.Suggested {
			suggested = append(suggested, map[string]any{"ability": s.Label(), "trainingPoints": s.Points, "upgrade": s.PetRank > 0})
		}
		out["suggested"] = suggested
		out["pointsLeftAfter"] = t.PointsLeft
		out["order"] = "server heuristic: higher ranks of abilities the pet already has first, then new ones, cheapest first; the player decides how valuable an ability is"
	}
	if len(t.DoesNotFit) > 0 {
		out["doesNotFit"] = teachableLabels(t.DoesNotFit, func(x Teachable) string { return fmt.Sprintf("%s (%d pts)", x.Label(), x.Points) })
	}
	if len(t.LevelTooLow) > 0 {
		out["petLevelTooLow"] = teachableLabels(t.LevelTooLow, func(x Teachable) string {
			return fmt.Sprintf("%s (requires pet level %d)", x.Label(), x.LevelRequired)
		})
	}
	if len(t.CostUnknown) > 0 {
		out["costUnknown"] = teachableLabels(t.CostUnknown, func(x Teachable) string { return x.Label() })
	}
	if t.NothingToLearn() {
		out["learnableNote"] = "there are no lessons the Hunter knows that the pet does not have"
	}
	return out
}

func teachableLabels(list []Teachable, label func(Teachable) string) []string {
	out := make([]string, len(list))
	for i, t := range list {
		out[i] = label(t)
	}
	return out
}

func (r StableReview) render(fresh snapshot.Freshness) map[string]any {
	if !r.Present {
		return map[string]any{"note": "no data: open the stable window (Stable Master) in game and run /mcp reload"}
	}
	pets := []any{}
	for _, p := range r.Pets {
		pets = append(pets, p.render(r.HasActive))
	}
	out := map[string]any{"pets": pets, "count": len(pets)}
	if r.HasSlots {
		out["slots"] = r.Slots
	}
	snapshot.Merge(out, fresh.CapturedMetaOf(r.Captured))
	if r.HasActive {
		out["active"] = map[string]any{"name": r.ActiveName, "level": r.ActiveLevel, "family": r.ActiveFamily}
		if len(r.HigherLevel) > 0 {
			out["higherLevelThanActive"] = r.HigherLevel
		}
	} else {
		out["note"] = "there is no pet with the character right now, so there is nothing to compare levels with"
	}
	out["limits"] = "the addon saves for stabled pets only the slot, name, level, family and specialization; there is no data on their abilities, loyalty and diet"
	return out
}

func (c StabledComparison) render(hasActive bool) map[string]any {
	e := map[string]any{}
	d := c.Shown
	for _, f := range []struct {
		key   string
		value any
	}{
		{"slot", d.Slot}, {"name", d.Name}, {"level", d.Level}, {"family", d.Family},
		{"specialization", d.Specialization}, {"loyalty", d.Loyalty}, {"active", d.Active},
	} {
		if f.value != nil {
			e[f.key] = f.value
		}
	}
	if hasActive {
		cmp := map[string]any{}
		if c.VsActive.HasLevelDiff {
			cmp["levelDiff"] = c.VsActive.LevelDiff
		}
		if c.VsActive.HasSameFamily {
			cmp["sameFamily"] = c.VsActive.SameFamily
		}
		e["vsActive"] = cmp
	}
	return e
}
