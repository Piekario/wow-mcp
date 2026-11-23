// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/character"
	"wowmcp/internal/game/itemkind"
	"wowmcp/internal/game/loot"
	"wowmcp/internal/game/periods"
	"wowmcp/internal/game/progress"
	"wowmcp/internal/game/xpgains"
	"wowmcp/internal/snapshot"
)

// notableQuality: items of this quality (Uncommon) and above go into "notableItems".
const notableQuality = 2

// maxListed is how many list entries (quests, items, zones) we show in one session.
const maxListed = 10

// ItemGain is an item that was gained in the bags during the session.
type ItemGain struct {
	Name    string
	Quality int64
	Count   int64
}

// Session is the description of one play session built from the dumps belonging to it.
type Session struct {
	Start, End     int64 // time of the first and the last dump
	Snapshots      int
	ElapsedSeconds int64
	Basis          string

	LevelStart, LevelEnd int64
	XP                   int64
	XPExact              bool
	MoneyDelta           int64

	QuestsCompleted []string // turned in during the session (from the completedQuests list)
	QuestsAccepted  []string // new in the quest log
	NotableItems    []ItemGain
	Loot            []loot.Item   // own loot from the loot section of the session's dumps (excluding the first: its loot predates the session)
	XPGains         xpgains.Gains // XP by source from the xpGains section of the session's dumps (excluding the first, like loot)
	LootPeriods     periods.Merge // the loot periods merged, with those carried from saves the server never read
	XPPeriods       periods.Merge // the same for xpGains
	Deaths          int64
	DeathZones      []string
	Zones           []string // zones seen in the dumps, in order of visit
}

// Describe turns a stretch of history into a session description.
func Describe(st progress.Stretch) Session {
	snaps := st.Snapshots
	s := Session{Snapshots: len(snaps), ElapsedSeconds: st.ElapsedSeconds, Basis: st.Basis}
	if len(snaps) == 0 {
		return s
	}
	first, last := snaps[0], snaps[len(snaps)-1]
	s.Start, _ = first.DumpedAt()
	s.End, _ = last.DumpedAt()
	s.LevelStart, _ = first.LevelAndMoney()
	s.LevelEnd, _ = last.LevelAndMoney()
	_, m0 := first.LevelAndMoney()
	_, m1 := last.LevelAndMoney()
	s.MoneyDelta = m1 - m0
	s.XP, s.XPExact = st.XPGained()
	s.QuestsCompleted = questsCompleted(snaps)
	s.QuestsAccepted = questsAccepted(snaps)
	s.NotableItems = notableItems(snaps)
	s.Loot, s.LootPeriods = loot.Gained(first, snaps[1:])
	s.XPGains, s.XPPeriods = xpgains.Gained(first, snaps[1:])
	s.Deaths, s.DeathZones = deaths(first, last, s.Start, s.End)
	s.Zones = zones(snaps)
	return s
}

// questTitles collects quest titles by ID from all dumps (a turned-in quest disappears from the log, but its title was in an earlier dump).
func questTitles(snaps []snapshot.Snapshot) map[int64]string {
	titles := map[int64]string{}
	for _, s := range snaps {
		for _, raw := range s.List("quests") {
			q := snapshot.AsMap(raw)
			if id, ok := snapshot.AsInt(q["id"]); ok {
				if title, _ := q["title"].(string); title != "" {
					titles[id] = title
				}
			}
		}
	}
	return titles
}

func idSet(list []any) map[int64]bool {
	out := map[int64]bool{}
	for _, raw := range list {
		if id, ok := snapshot.AsInt(raw); ok {
			out[id] = true
		}
	}
	return out
}

// questsCompleted lists the quests added to completedQuests between the first and the last dump.
// The title comes from the quest log in one of the session's dumps; when there is none, only the ID is given.
func questsCompleted(snaps []snapshot.Snapshot) []string {
	first, last := snaps[0], snaps[len(snaps)-1]
	if first.List("completedQuests") == nil || last.List("completedQuests") == nil {
		return nil
	}
	before, after := idSet(first.List("completedQuests")), idSet(last.List("completedQuests"))
	titles := questTitles(snaps)
	var ids []int64
	for id := range after {
		if !before[id] {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]string, len(ids))
	for i, id := range ids {
		if title := titles[id]; title != "" {
			out[i] = title
		} else {
			out[i] = fmt.Sprintf("quest #%d", id)
		}
	}
	return out
}

// questsAccepted lists quests visible in the log in later dumps of the session but absent from the first.
func questsAccepted(snaps []snapshot.Snapshot) []string {
	known := idSet(nil)
	for _, raw := range snaps[0].List("quests") {
		if id, ok := snapshot.AsInt(snapshot.AsMap(raw)["id"]); ok {
			known[id] = true
		}
	}
	var out []string
	for _, s := range snaps[1:] {
		for _, raw := range s.List("quests") {
			q := snapshot.AsMap(raw)
			id, ok := snapshot.AsInt(q["id"])
			if !ok || known[id] {
				continue
			}
			known[id] = true
			out = append(out, fmt.Sprint(q["title"]))
		}
	}
	return out
}

type bagEntry struct {
	count, quality int64
}

func bagCounts(s snapshot.Snapshot) map[string]*bagEntry {
	out := map[string]*bagEntry{}
	for _, bag := range s.Bags() {
		for _, raw := range snapshot.AsList(snapshot.AsMap(bag)["items"]) {
			item := snapshot.AsMap(raw)
			name, _ := item["name"].(string)
			if name == "" {
				continue
			}
			e := out[name]
			if e == nil {
				e = &bagEntry{}
				e.quality, _ = snapshot.AsInt(item["quality"])
				out[name] = e
			}
			e.count += snapshot.ItemCount(item)
		}
	}
	return out
}

// notableItems sums gains of items of Uncommon quality and above between consecutive dumps of the session,
// best quality first. An item gained and lost between dumps is invisible.
func notableItems(snaps []snapshot.Snapshot) []ItemGain {
	gains := map[string]*ItemGain{}
	prev := bagCounts(snaps[0])
	for _, s := range snaps[1:] {
		cur := bagCounts(s)
		for name, e := range cur {
			if e.quality < notableQuality {
				continue
			}
			var before int64
			if p := prev[name]; p != nil {
				before = p.count
			}
			if e.count > before {
				g := gains[name]
				if g == nil {
					g = &ItemGain{Name: name, Quality: e.quality}
					gains[name] = g
				}
				g.Count += e.count - before
			}
		}
		prev = cur
	}
	out := make([]ItemGain, 0, len(gains))
	for _, g := range gains {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Quality != out[j].Quality {
			return out[i].Quality > out[j].Quality
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// deaths gives the new deaths in the session and the zones from the recent deaths list that fall within its time.
func deaths(first, last snapshot.Snapshot, start, end int64) (int64, []string) {
	n0, ok0 := character.DeathCount(first)
	n1, ok1 := character.DeathCount(last)
	if !ok0 || !ok1 || n1 <= n0 {
		return 0, nil
	}
	var zones []string
	for _, raw := range snapshot.AsList(last.Map("deaths")["recent"]) {
		d := snapshot.AsMap(raw)
		if at, ok := snapshot.AsInt(d["at"]); ok && at > start && at <= end {
			if zone, _ := d["zone"].(string); zone != "" {
				zones = appendUnique(zones, zone)
			}
		}
	}
	return n1 - n0, zones
}

func zones(snaps []snapshot.Snapshot) []string {
	var out []string
	for _, s := range snaps {
		if zone, _ := s.Character()["zone"].(string); zone != "" {
			out = appendUnique(out, zone)
		}
	}
	return out
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func qualityLabel(q int64) string {
	if name, ok := itemkind.QualityName(q); ok {
		return name
	}
	return fmt.Sprint(q)
}

func limited[T any](list []T) (shown []T, more int) {
	if len(list) > maxListed {
		return list[:maxListed], len(list) - maxListed
	}
	return list, 0
}

// View is the response about a session; empty categories are omitted.
func (s Session) View() map[string]any {
	out := map[string]any{
		"start":     snapshot.FormatTime(s.Start),
		"end":       snapshot.FormatTime(s.End),
		"snapshots": s.Snapshots,
	}
	if s.Snapshots < 2 {
		out["note"] = "The session has one dump, so there is nothing to compare with; changes will appear after the next /mcp reload."
		out["level"] = s.LevelEnd
		out["zones"] = s.Zones
		return out
	}
	out["duration"] = snapshot.FormatSeconds(s.ElapsedSeconds)
	out["durationBasis"] = s.Basis
	out["level"] = fmt.Sprintf("%d -> %d", s.LevelStart, s.LevelEnd)
	if s.LevelEnd > s.LevelStart {
		out["levelsGained"] = s.LevelEnd - s.LevelStart
	}
	out["xpGained"] = s.XP
	if !s.XPExact {
		out["xpNote"] = "between some dumps there were 2+ level-ups; XP from those stretches is skipped"
	}
	out["goldChange"] = snapshot.Money(s.MoneyDelta).Signed()
	if len(s.QuestsCompleted) > 0 {
		list, more := limited(s.QuestsCompleted)
		out["questsCompleted"] = map[string]any{"count": len(s.QuestsCompleted), "titles": list, "more": more}
	}
	if len(s.QuestsAccepted) > 0 {
		list, more := limited(s.QuestsAccepted)
		out["questsAccepted"] = map[string]any{"count": len(s.QuestsAccepted), "titles": list, "more": more}
	}
	if len(s.NotableItems) > 0 {
		list, more := limited(s.NotableItems)
		items := make([]string, len(list))
		for i, g := range list {
			items[i] = fmt.Sprintf("%s x%d (%s)", g.Name, g.Count, qualityLabel(g.Quality))
		}
		out["notableItems"] = map[string]any{"items": items, "more": more, "note": "gains in bags of Uncommon quality and above, visible between dumps"}
	}
	if l := loot.Describe(s.Loot, maxListed); l != nil {
		l["note"] = "own loot from chat messages (loot section), summed from the session's dumps; unlike notableItems it also shows what was sold or used up"
		s.LootPeriods.Describe(l)
		out["loot"] = l
	}
	if x := xpgains.Describe(s.XPGains); x != nil {
		x["note"] = "own XP from chat messages and turned-in quests (xpGains section), summed from the session's dumps; kills = kills, quests = turned-in quests, other = the rest (e.g. exploration)"
		s.XPPeriods.Describe(x)
		out["xpSources"] = x
	}
	if s.Deaths > 0 {
		d := map[string]any{"count": s.Deaths}
		if len(s.DeathZones) > 0 {
			d["zones"] = s.DeathZones
		}
		out["deaths"] = d
	}
	if len(s.Zones) > 0 {
		out["zones"] = s.Zones
	}
	return out
}
