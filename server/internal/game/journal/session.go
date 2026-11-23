// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package journal

import (
	"fmt"
	"sort"

	"wowmcp/internal/game/character"
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
		for _, q := range readQuestLog(s) {
			if title, _ := q.Title.(string); q.HasID && title != "" {
				titles[q.ID] = title
			}
		}
	}
	return titles
}

// questsCompleted lists the quests added to completedQuests between the first and the last dump.
// The title comes from the quest log in one of the session's dumps; when there is none, only the ID is given.
func questsCompleted(snaps []snapshot.Snapshot) []string {
	before, listedBefore := readCompleted(snaps[0])
	after, listedAfter := readCompleted(snaps[len(snaps)-1])
	if !listedBefore || !listedAfter {
		return nil
	}
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
	known := map[int64]bool{}
	for _, q := range readQuestLog(snaps[0]) {
		if q.HasID {
			known[q.ID] = true
		}
	}
	var out []string
	for _, s := range snaps[1:] {
		for _, q := range readQuestLog(s) {
			if !q.HasID || known[q.ID] {
				continue
			}
			known[q.ID] = true
			out = append(out, fmt.Sprint(q.Title))
		}
	}
	return out
}

// notableItems sums gains of items of Uncommon quality and above between consecutive dumps of the session,
// best quality first. An item gained and lost between dumps is invisible.
func notableItems(snaps []snapshot.Snapshot) []ItemGain {
	gains := map[string]*ItemGain{}
	prev := readBagCounts(snaps[0])
	for _, s := range snaps[1:] {
		cur := readBagCounts(s)
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
	for _, d := range readRecentDeaths(last) {
		if d.HasAt && d.At > start && d.At <= end && d.Zone != "" {
			zones = appendUnique(zones, d.Zone)
		}
	}
	return n1 - n0, zones
}

func zones(snaps []snapshot.Snapshot) []string {
	var out []string
	for _, s := range snaps {
		if zone := readZone(s); zone != "" {
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
