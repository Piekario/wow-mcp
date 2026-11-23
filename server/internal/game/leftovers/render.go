// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package leftovers

import "wowmcp/internal/snapshot"

// View assesses quest items: which are needed by quests in the log (keep) and which
// look like leftovers (leftovers), together with the confidence of the assessment.
func View(s snapshot.Snapshot) map[string]any { return Inspect(s).Render() }

func (r Report) Render() map[string]any {
	if len(r.Verdicts) == 0 {
		return map[string]any{
			"questItems": 0,
			"note":       "No quest items (type Quest) in bags and bank. The bank shows the state from the last visit.",
		}
	}
	keep, leftovers := []any{}, []any{}
	for _, v := range r.Verdicts {
		if v.Keep {
			keep = append(keep, v.render())
		} else {
			leftovers = append(leftovers, v.render())
		}
	}
	out := map[string]any{
		"questItems": len(r.Verdicts),
		"keep":       keep,
		"leftovers":  leftovers,
		"confidence": r.Confidence(),
		"advice":     "These are only candidates. The server does not know whether the item is needed for a quest not yet taken or later in a chain. Before discarding, check the item tooltip in game; nothing is removed and nothing should be removed automatically.",
		"limits":     "Matching is by item name against quest titles and objectives from the log. completedQuests contains only IDs, without names, so an item cannot be tied to a turned-in quest. The bank shows the state from the last visit.",
	}
	if r.CompletedQuests > 0 {
		out["completedQuestsKnown"] = r.CompletedQuests
	}
	return out
}

func (v Verdict) render() map[string]any {
	e := map[string]any{"name": v.Name, "count": v.Count, "where": v.Where}
	if len(v.ForQuests) > 0 {
		e["forQuests"] = v.ForQuests
	}
	e["reason"] = v.Reason
	if !v.Keep {
		e["confidence"] = v.Confidence
	}
	return e
}
