// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

// Package social covers the character's friends and guild (the friends and guild sections) and PvP (the pvp section).
// The addon saves these lists only after the social or guild window is opened and reads only client memory:
// it never asks the game server for the friends or guild list itself. So the lists are the state from the last visit,
// and the response says how long ago they were saved.
package social

import (
	"sort"
	"strings"

	"wowmcp/internal/snapshot"
)

// Query is the input of wow_social.
type Query struct {
	Character      string `json:"character,omitempty" jsonschema:"character name; empty = the most recently dumped character"`
	IncludeOffline bool   `json:"includeOffline,omitempty" jsonschema:"true = also show guild members who were offline (default online only; the rest as a count)"`
}

const privacyNote = "The addon reads only what the client has in memory after the social or guild window is opened; it never asks the game server for these lists itself. " +
	"The list is the state at the time of saving (capturedAge), so who is online may have changed."

const friendsHint = "The friends list has not been read yet (this does not mean it is empty): open the social window (friends list) in game and run /mcp reload. The addon never asks the server for this list."
const friendsEmptyNote = "The friends list is empty: the client showed it to the addon and there was nobody on it."

const guildHint = "The dump does not say whether the character is in a guild (an addon older than 0.7.1 saved only the list from the guild window). After /mcp reload, addon 0.7.1 will save it itself."
const notInGuildNote = "The character is not in a guild (the addon checks this on every dump via IsInGuild, without asking the server)."
const rosterHint = "The character is in a guild, but the member list has not been read yet: open the guild window in game and run /mcp reload. The addon never asks the server for this list."

// View describes the character's friends and guild.
func View(s snapshot.Snapshot, q Query, fresh snapshot.Freshness) map[string]any {
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
	return out
}

// InGuild says whether the character is in a guild; known = false when the dump does not know (no guild section).
// A member list without the inGuild field (addon up to 0.7.0) also means a guild.
func InGuild(s snapshot.Snapshot) (in, known bool) {
	sec := s.Map("guild")
	if sec == nil {
		return false, false
	}
	if v, ok := sec["inGuild"].(bool); ok {
		return v, true
	}
	return true, true
}

// Friends describes the friends list; nil when the dump has no friends section.
func Friends(s snapshot.Snapshot, fresh snapshot.Freshness) map[string]any {
	sec := s.Map("friends")
	if sec == nil {
		return nil
	}
	list := []any{}
	for _, raw := range snapshot.AsList(sec["friends"]) {
		m := snapshot.AsMap(raw)
		e := pick(m, "name", "level", "class", "area", "notes")
		online, _ := m["connected"].(bool)
		e["online"] = online
		list = append(list, e)
	}
	out := map[string]any{"friends": list}
	copyInt(out, sec, "total")
	copyInt(out, sec, "online")
	addCapture(out, sec, fresh)
	if len(list) == 0 {
		out["note"] = friendsEmptyNote
	}
	return out
}

// Guild describes the guild: name, the player's rank, counts and online members (everyone when includeOffline); nil without a guild section.
func Guild(s snapshot.Snapshot, includeOffline bool, fresh snapshot.Freshness) map[string]any {
	sec := s.Map("guild")
	if sec == nil {
		return nil
	}
	if in, _ := InGuild(s); !in {
		out := map[string]any{"inGuild": false, "note": notInGuildNote}
		addCapture(out, sec, fresh)
		return out
	}
	out := pick(sec, "name", "rank", "rankIndex")
	out["inGuild"] = true
	if _, ok := sec["members"]; !ok {
		out["rosterHint"] = rosterHint
		addCapture(out, sec, fresh)
		return out
	}
	copyInt(out, sec, "total")
	copyInt(out, sec, "online")
	addCapture(out, sec, fresh)

	var shown []map[string]any
	offline := 0
	for _, raw := range snapshot.AsList(sec["members"]) {
		m := snapshot.AsMap(raw)
		on, _ := m["online"].(bool)
		if !on && !includeOffline {
			offline++
			continue
		}
		e := pick(m, "name", "rank", "rankIndex", "level", "class", "zone")
		e["online"] = on
		shown = append(shown, e)
	}
	sort.SliceStable(shown, func(i, j int) bool {
		a, _ := snapshot.AsInt(shown[i]["rankIndex"])
		b, _ := snapshot.AsInt(shown[j]["rankIndex"])
		if a != b {
			return a < b
		}
		return strings.ToLower(asString(shown[i]["name"])) < strings.ToLower(asString(shown[j]["name"]))
	})
	members := make([]any, len(shown))
	for i, m := range shown {
		members[i] = m
	}
	label := "onlineMembers"
	if includeOffline {
		label = "members"
	}
	out[label] = members
	if !includeOffline && offline > 0 {
		out["offlineHidden"] = offline
	}
	if only, _ := sec["onlineOnly"].(bool); only {
		out["onlineOnlyNote"] = "The guild is large (over 200 people), so the addon saved only online members; the total and online counts come from the client."
	}
	return out
}

// PvP describes the pvp section (honorable kills, honor, rank); nil when the dump has no section.
func PvP(s snapshot.Snapshot) map[string]any {
	sec := s.Map("pvp")
	if sec == nil {
		return nil
	}
	return pick(sec, "honorableKills", "dishonorableKills", "highestRank", "honor", "honorMax", "honorLevel", "rank", "rankName")
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// pick copies the given keys that occur in the map.
func pick(m map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

func copyInt(dst, src map[string]any, key string) {
	if v, ok := snapshot.AsInt(src[key]); ok {
		dst[key] = v
	}
}

// addCapture adds the time and age of the list capture.
func addCapture(out, sec map[string]any, fresh snapshot.Freshness) {
	snapshot.Merge(out, fresh.CapturedMeta(sec))
	if at, ok := snapshot.AsInt(sec["capturedAt"]); ok && fresh.Now != nil {
		out["capturedAge"] = snapshot.FormatAge(fresh.Age(at))
	}
}
