// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package social

import (
	"fmt"
	"sort"
	"strconv"

	"wowmcp/internal/snapshot"
)

const privacyNote = "The addon reads only what the client has in memory after the social or guild window is opened; it never asks the game server for these lists itself. " +
	"The list is the state at the time of saving (capturedAge), so who is online may have changed."

const friendsHint = "The friends list has not been read yet (this does not mean it is empty): open the social window (friends list) in game and run /mcp reload. The addon never asks the server for this list."
const friendsEmptyNote = "The friends list is empty: the client showed it to the addon and there was nobody on it."

const guildHint = "The dump does not say whether the character is in a guild (an addon older than 0.7.1 saved only the list from the guild window). After /mcp reload, addon 0.7.1 will save it itself."
const notInGuildNote = "The character is not in a guild (the addon checks this on every dump via IsInGuild, without asking the server)."
const rosterHint = "The character is in a guild, but the member list has not been read yet: open the guild window in game and run /mcp reload. The addon never asks the server for this list."

// View describes the character's friends and guild, and who of them could group with the character now.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) (map[string]any, error) {
	levelRange := int64(DefaultLevelRange)
	switch {
	case q.LevelRange < 0:
		return nil, fmt.Errorf("levelRange must be positive (default %d), got %d", DefaultLevelRange, q.LevelRange)
	case q.LevelRange > 0:
		levelRange = int64(q.LevelRange)
	}
	out := map[string]any{"privacy": privacyNote}
	if f := Friends(s, fresh); f != nil {
		out["friends"] = f
	} else {
		out["friendsHint"] = friendsHint
	}
	if g := Guild(s, q.IncludeOffline, fresh); g != nil {
		out["guild"] = g
	} else {
		out["guildHint"] = guildHint
	}
	if people, ok := groupablePeople(s); ok {
		out["groupCandidates"] = FindCandidates(people, readSelf(s), levelRange).render()
	}
	return out, nil
}

// groupablePeople are the guild members and friends the dump lists; false when it lists neither.
func groupablePeople(s snapshot.Snapshot) ([]Person, bool) {
	g, f := readGuild(s), readFriends(s)
	if !f.Saved && !(g.In && g.HasRoster) {
		return nil, false
	}
	return append(g.people(), f.people()...), true
}

// Friends describes the friends list; nil when the dump has no friends section.
func Friends(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	f := readFriends(s)
	if !f.Saved {
		return nil
	}
	list := []any{}
	for _, friend := range f.List {
		e := present(map[string]any{"name": friend.Name, "level": friend.Level, "class": friend.Class, "area": friend.Area, "notes": friend.Notes})
		e["online"] = friend.Online
		list = append(list, e)
	}
	out := map[string]any{"friends": list}
	putCount(out, "total", f.Total)
	putCount(out, "online", f.Online)
	f.Capture.render(out, fresh)
	if len(list) == 0 {
		out["note"] = friendsEmptyNote
	}
	return out
}

// Guild describes the guild: name, the player's rank, counts and online members (everyone when includeOffline); nil without a guild section.
func Guild(s snapshot.Snapshot, includeOffline bool, fresh snapshot.Freshness) map[string]any {
	g := readGuild(s)
	if !g.Saved {
		return nil
	}
	if !g.In {
		out := map[string]any{"inGuild": false, "note": notInGuildNote}
		g.Capture.render(out, fresh)
		return out
	}
	out := present(map[string]any{"name": g.Name, "rank": g.Rank, "rankIndex": g.RankIndex})
	out["inGuild"] = true
	if !g.HasRoster {
		out["rosterHint"] = rosterHint
		g.Capture.render(out, fresh)
		return out
	}
	putCount(out, "total", g.Total)
	putCount(out, "online", g.Online)
	g.Capture.render(out, fresh)

	out["summary"] = Summarize(g.people()).render()
	roster := g.Roster(includeOffline)
	members := make([]any, len(roster.Shown))
	for i, m := range roster.Shown {
		e := present(map[string]any{"name": m.Name, "rank": m.Rank, "rankIndex": m.RankIndex, "level": m.Level, "class": m.Class, "zone": m.Zone})
		if m.Note != "" {
			e["note"] = m.Note
		}
		if m.HasOfflineDays {
			e["offlineDays"] = m.OfflineDays
		}
		e["online"] = m.Online
		members[i] = e
	}
	label := "onlineMembers"
	if includeOffline {
		label = "members"
	}
	out[label] = members
	if !includeOffline && roster.OfflineHidden > 0 {
		out["offlineHidden"] = roster.OfflineHidden
	}
	if g.OnlineOnly {
		out["onlineOnlyNote"] = "The guild is large (over 200 people), so the addon saved only online members; the total and online counts come from the client."
	}
	return out
}

// PvP describes the pvp section (honorable kills, honor, rank); nil when the dump has no section.
func PvP(s snapshot.Snapshot) map[string]any {
	p := readPvP(s)
	if !p.Saved {
		return nil
	}
	return present(map[string]any{"honorableKills": p.HonorableKills, "dishonorableKills": p.DishonorableKills,
		"highestRank": p.HighestRank, "honor": p.Honor, "honorMax": p.HonorMax, "honorLevel": p.HonorLevel,
		"rank": p.Rank, "rankName": p.RankName})
}

// present drops the fields the dump did not have.
func present(fields map[string]any) map[string]any {
	for k, v := range fields {
		if v == nil {
			delete(fields, k)
		}
	}
	return fields
}

func putCount(out map[string]any, key string, c Count) {
	if c.Known {
		out[key] = c.N
	}
}

func (c Capture) render(out map[string]any, fresh snapshot.Freshness) {
	snapshot.Merge(out, fresh.CapturedMetaOf(c.Captured))
	if c.Age.Known && fresh.Now != nil {
		out["capturedAge"] = snapshot.FormatAge(fresh.Age(c.Age.N))
	}
}

func (s Summary) render() map[string]any {
	out := map[string]any{"listed": s.Listed, "online": s.Online, "classes": s.Classes, "onlineClasses": s.OnlineClasses}
	if len(s.Bands) > 0 {
		out["levelBands"] = renderBands(s.Bands)
	}
	if len(s.OnlineBands) > 0 {
		out["onlineLevelBands"] = renderBands(s.OnlineBands)
		out["onlineAverageLevel"] = s.OnlineAverageLevel
	}
	return out
}

func renderBands(bands map[int64]int) map[string]any {
	out := map[string]any{}
	for from, n := range bands {
		out[strconv.FormatInt(from, 10)+"-"+strconv.FormatInt(from+9, 10)] = n
	}
	return out
}

const (
	maxCandidatesPerRole = 15
	candidatesNote       = "Roles are what the class can do, not what the player will pick: a Paladin or Druid is listed as tank, healer and damage. " +
		"Only people online when the lists were saved (capturedAge) count; ask them, the state may have changed. Sorted by closeness in level."
)

func (c Candidates) render() map[string]any {
	if c.NoLevel {
		return map[string]any{"note": "The dump has no character level, so nobody can be matched by level."}
	}
	out := map[string]any{"yourLevel": c.Self.Level, "levelRange": c.Range, "levels": fmt.Sprintf("%d-%d", c.Low, c.High),
		"count": c.Total, "note": candidatesNote}
	for key, list := range map[string][]Person{"tankCapable": c.Tank, "healerCapable": c.Healer, "damageCapable": c.Damage, "unknownClass": c.Unknown} {
		if len(list) > 0 {
			out[key] = renderPeople(list)
		}
	}
	if c.Total == 0 {
		out["note"] = "Nobody online in the saved lists is within the level range. " + candidatesNote
	}
	if c.OnlineWithoutLevel > 0 {
		out["onlineWithoutLevel"] = c.OnlineWithoutLevel
	}
	return out
}

func renderPeople(people []Person) []any {
	out := []any{}
	for i, p := range people {
		if i == maxCandidatesPerRole {
			out = append(out, map[string]any{"omitted": len(people) - maxCandidatesPerRole})
			break
		}
		e := map[string]any{"name": p.Name, "level": p.Level, "source": renderSources(p.Sources)}
		for k, v := range map[string]string{"class": p.Class, "zone": p.Zone, "rank": p.Rank, "note": p.Note} {
			if v != "" {
				e[k] = v
			}
		}
		out = append(out, e)
	}
	return out
}

func renderSources(sources []Source) []string {
	out := make([]string, len(sources))
	for i, s := range sources {
		out[i] = string(s)
	}
	sort.Strings(out)
	return out
}
