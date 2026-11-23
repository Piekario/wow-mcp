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

// Count is a number the dump may not have.
type Count struct {
	N     int64
	Known bool
}

// Capture is when a list was saved. Age is read apart from Captured because the age accepts a number type
// that the capture meta does not.
type Capture struct {
	Captured snapshot.Captured
	Age      Count
}

// The text fields of Friend, Member, Guild and PvP are kept as the client gave them; nil means the dump has no such field.
type Friend struct {
	Name, Level, Class, Area, Notes any
	Online                          bool
}

type FriendList struct {
	Saved         bool
	List          []Friend
	Total, Online Count
	Capture
}

type Member struct {
	Name, Rank, RankIndex, Level, Class, Zone any
	Online                                    bool
}

type GuildState struct {
	Saved                 bool
	In                    bool
	Name, Rank, RankIndex any
	HasRoster, OnlineOnly bool
	Members               []Member
	Total, Online         Count
	Capture
}

// PvP is the pvp section; a classic client has ranks, Forever does not.
type PvPRecord struct {
	Saved                                                                       bool
	HonorableKills, DishonorableKills, HighestRank, Honor, HonorMax, HonorLevel any
	Rank, RankName                                                              any
}

// Roster is the guild members to show: the online ones, or everyone, by rank and then by name.
type Roster struct {
	Shown         []Member
	OfflineHidden int
}

func (g GuildState) Roster(includeOffline bool) Roster {
	var r Roster
	for _, m := range g.Members {
		if !m.Online && !includeOffline {
			r.OfflineHidden++
			continue
		}
		r.Shown = append(r.Shown, m)
	}
	sort.SliceStable(r.Shown, func(i, j int) bool {
		a, _ := snapshot.AsInt(r.Shown[i].RankIndex)
		b, _ := snapshot.AsInt(r.Shown[j].RankIndex)
		if a != b {
			return a < b
		}
		return strings.ToLower(asString(r.Shown[i].Name)) < strings.ToLower(asString(r.Shown[j].Name))
	})
	return r
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
