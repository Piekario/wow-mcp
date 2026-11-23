// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package questrewards

import (
	"sort"

	"wowmcp/internal/snapshot"
)

// questSources are the quests the dump has rewards for. window: the NPC windows, turn-ins first, then offers;
// logged: the quest log, completed ones first. anyRewards: the dump has reward data at all.
type questSources struct {
	window, logged []Quest
	anyRewards     bool
}

func readQuests(s snapshot.Snapshot) questSources {
	var out questSources
	for _, w := range []struct{ section, source string }{{"questTurnIn", fromTurnIn}, {"questOffer", fromOffer}} {
		sec := s.Map(w.section)
		if sec == nil {
			continue
		}
		q := Quest{Source: w.source, Captured: snapshot.CapturedOf(sec), Rewards: readRewards(snapshot.AsMap(sec["rewards"]))}
		q.ID, _ = snapshot.AsInt(sec["id"])
		q.Title, _ = sec["title"].(string)
		q.NPC, _ = sec["npc"].(string)
		out.window = append(out.window, q)
		out.anyRewards = out.anyRewards || sec["rewards"] != nil
	}
	for _, raw := range s.List("quests") {
		m := snapshot.AsMap(raw)
		out.anyRewards = out.anyRewards || m["rewards"] != nil
		rewards := snapshot.AsMap(m["rewards"])
		if len(rewards) == 0 {
			continue
		}
		q := Quest{Source: fromLog, Rewards: readRewards(rewards)}
		q.ID, _ = snapshot.AsInt(m["id"])
		q.Title, _ = m["title"].(string)
		q.Done, _ = m["complete"].(bool)
		out.logged = append(out.logged, q)
	}
	sort.SliceStable(out.logged, func(i, j int) bool { return out.logged[i].Done && !out.logged[j].Done })
	return out
}

func readRewards(m map[string]any) Rewards {
	var r Rewards
	for _, raw := range snapshot.AsList(m["items"]) {
		it := snapshot.AsMap(raw)
		r.Fixed = append(r.Fixed, FixedItem{Name: it["name"], Count: snapshot.ItemCount(it)})
	}
	if money, ok := snapshot.AsInt(m["money"]); ok && money > 0 {
		r.Money = money
	}
	if xp, ok := snapshot.AsInt(m["xp"]); ok && xp > 0 {
		r.XP = xp
	}
	r.Currencies = snapshot.AsList(m["currencies"])
	for _, raw := range snapshot.AsList(m["choices"]) {
		it := snapshot.AsMap(raw)
		c := Choice{Name: it["name"], ID: it["id"], Count: snapshot.ItemCount(it), Raw: it}
		c.ItemID, c.HasItemID = snapshot.AsInt(it["id"])
		r.Choices = append(r.Choices, c)
	}
	return r
}
